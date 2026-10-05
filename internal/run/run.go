package run

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"pitstop/internal/actengine"
	"pitstop/internal/config"
	"pitstop/internal/model"
	"pitstop/internal/report"
	"pitstop/internal/snapshot"
	"pitstop/internal/workflow"
	"strings"
	"syscall"
	"time"
)

type Options struct {
	Repo             string
	Jobs             []string
	Timeout          time.Duration
	Architecture     string
	ForceIntegration bool
	Progress         io.Writer
}
type lock struct{ f *os.File }

func (l *lock) Release() {
	if l != nil && l.f != nil {
		l.f.Close()
	}
}
func acquireLock(repo string) (*lock, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(repo))
	dir := filepath.Join(cache, "pitstop", "locks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%x.lock", sum[:])), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, model.Err("RUN_ALREADY_ACTIVE", "another Pitstop run is active for this repository", err)
	}
	return &lock{f: f}, nil
}
func Execute(ctx context.Context, o Options) (model.Envelope, error) {
	fail := func(err error) (model.Envelope, error) {
		if ctx.Err() != nil {
			err = model.Err("CANCELLED", "run cancelled", ctx.Err())
		}
		return errorEnvelope("run", err), err
	}
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	started := time.Now()
	repo, err := config.ResolveRepo(o.Repo)
	if err != nil {
		return fail(err)
	}
	l, err := acquireLock(repo)
	if err != nil {
		return fail(err)
	}
	defer l.Release()
	cfg, err := config.Load(repo)
	if err != nil {
		return fail(err)
	}
	initialConfigBytes, _ := json.Marshal(cfg)
	if o.Architecture != "" {
		cfg.Runner.Architecture = o.Architecture
	}
	for _, j := range o.Jobs {
		found := false
		for _, x := range cfg.Jobs {
			if x == j {
				found = true
			}
		}
		if !found {
			e := model.Err("CONFIG_INVALID", "job is not in configured selection: "+j, nil)
			return fail(e)
		}
	}
	workflowPath, err := config.ResolveWorkflow(repo, cfg.Workflow)
	if err != nil {
		return fail(err)
	}
	inv, err := workflow.Inspect(repo, workflowPath)
	if err != nil {
		return fail(err)
	}
	if len(o.Jobs) > 0 {
		cfg.Jobs = o.Jobs
	}
	if err := cfg.Validate(); err != nil {
		return fail(err)
	}
	if o.ForceIntegration && (cfg.Adapter != "recruit100-service-integration-v1" || !contains(cfg.Jobs, "integration")) {
		e := model.Err("CONFIG_INVALID", "--force-integration requires the service adapter and selected integration job", nil)
		return fail(e)
	}
	plan, err := workflow.Plan(repo, cfg, inv, o.ForceIntegration)
	if err != nil {
		return fail(err)
	}
	env, err := actengine.Check(ctx)
	if err != nil {
		return fail(err)
	}
	env.Info.Image = cfg.Runner.Image
	env.Info.Architecture = cfg.Runner.Architecture
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	cache, err := os.UserCacheDir()
	if err != nil {
		return fail(err)
	}
	runDir := filepath.Join(cache, "pitstop", "runs", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return fail(err)
	}
	env.Home = filepath.Join(runDir, "home")
	if err := os.MkdirAll(filepath.Join(env.Home, ".config"), 0700); err != nil {
		return fail(err)
	}
	defer os.RemoveAll(env.Home)
	// Every exit after runner creation gets a fresh cleanup context.
	cleanupDone := false
	defer func() {
		if !cleanupDone {
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if e := actengine.Cleanup(c, runDir); e != nil && o.Progress != nil {
				fmt.Fprintln(o.Progress, "CLEANUP_INCOMPLETE:", e)
			}
		}
	}()
	if err := os.WriteFile(filepath.Join(runDir, "event.json"), []byte(`{}`+"\n"), 0600); err != nil {
		return fail(err)
	}
	snap, err := snapshot.CaptureContext(ctx, repo, runDir)
	if err != nil {
		return fail(err)
	}
	defer snapshot.Remove(snap)
	snap.CheckoutMode = "local-source-copy"
	snap.ExcludedPaths = nonNil(snap.ExcludedPaths)
	capturedConfig, err := config.Load(snap.Path)
	if err != nil {
		return fail(err)
	}
	capturedBytes, _ := json.Marshal(capturedConfig)
	if string(capturedBytes) != string(initialConfigBytes) {
		err := model.Err("SNAPSHOT_CHANGED", "config changed before capture", nil)
		return fail(err)
	}
	sourceWorkflow := filepath.Join(snap.Path, cfg.Workflow)
	capturedInv, capturedErr := workflow.Inspect(snap.Path, sourceWorkflow)
	if capturedErr != nil {
		return fail(capturedErr)
	}
	capturedPlan, capturedErr := workflow.Plan(snap.Path, cfg, capturedInv, o.ForceIntegration)
	if capturedErr != nil {
		return fail(capturedErr)
	}
	inv = capturedInv
	plan = capturedPlan
	if o.ForceIntegration && !strings.HasPrefix(env.DockerHost, "unix://") {
		for i := range plan.Jobs {
			if plan.Jobs[i].ID == "integration" {
				plan.Jobs[i].Eligible = false
				plan.Jobs[i].Reasons = append(plan.Jobs[i].Reasons, "integration requires a local Unix Docker endpoint")
			}
		}
	}
	originalCopy := filepath.Join(runDir, "original-workflow.yml")
	originalBytes, err := os.ReadFile(sourceWorkflow)
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(originalCopy, originalBytes, 0600); err != nil {
		return fail(err)
	}
	preparedCopy := filepath.Join(runDir, "prepared-workflow.yml")
	adjustments := []model.Adjustment{}
	if o.ForceIntegration && cfg.Adapter == "recruit100-service-integration-v1" {
		prepared := preparedCopy
		a, e := workflow.PrepareIntegration(sourceWorkflow, prepared)
		if e != nil {
			return fail(e)
		}
		adjustments = a
		plan.ExecutionWorkflowPath = prepared
	} else {
		if err := os.WriteFile(preparedCopy, originalBytes, 0600); err != nil {
			return fail(err)
		}
		plan.ExecutionWorkflowPath = preparedCopy
	}
	var results []model.JobResult
	for idx, j := range plan.Jobs {
		if !j.Eligible {
			code := "WORKFLOW_UNSUPPORTED"
			if len(j.Reasons) > 0 && strings.Contains(join(j.Reasons), "integration requires") {
				code = "INTEGRATION_RUNTIME_UNSUPPORTED"
			}
			msg := join(j.Reasons)
			results = append(results, model.JobResult{ID: j.ID, Status: "blocked", Provenance: "preflight", ReasonCode: &code, Reason: &msg, Steps: []model.StepResult{}})
			continue
		}
		r, e := actengine.Run(ctx, env, snap, plan, j, runDir, o.Timeout, o.Progress)
		if e != nil {
			reason := e.Error()
			r = model.JobResult{ID: j.ID, Status: "skipped", Provenance: "unknown", ReasonCode: model.Reason("ENGINE_OUTCOME_UNKNOWN"), Reason: &reason, Steps: []model.StepResult{}}
			if ctx.Err() != nil {
				r.ReasonCode = model.Reason("CANCELLED")
			}
		}
		results = append(results, r)
		if ctx.Err() != nil {
			for _, remaining := range plan.Jobs[idx+1:] {
				code := "CANCELLED"
				msg := "run cancelled before scheduling job"
				results = append(results, model.JobResult{ID: remaining.ID, Status: "skipped", Provenance: "unknown", ReasonCode: &code, Reason: &msg, Steps: []model.StepResult{}})
			}
			break
		}
	}
	code := report.ExitCode(results, ctx.Err() != nil)
	warnings := []string{}
	if e := snapshot.Remove(snap); e != nil {
		warnings = append(warnings, "CLEANUP_INCOMPLETE: source snapshot: "+e.Error())
		if code == 0 {
			code = 2
		}
	}
	if env.Home != "" {
		if e := os.RemoveAll(env.Home); e != nil {
			warnings = append(warnings, "CLEANUP_INCOMPLETE: ephemeral home: "+e.Error())
			if code == 0 {
				code = 2
			}
		}
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 5*time.Second)
	if cleanupErr := actengine.Cleanup(cleanupCtx, runDir); cleanupErr != nil {
		warnings = append(warnings, "CLEANUP_INCOMPLETE: "+cleanupErr.Error())
		if code == 0 {
			code = 2
		}
	}
	cancelCleanup()
	cleanupDone = true
	digestCtx, cancelDigest := context.WithTimeout(context.Background(), 5*time.Second)
	env.Info.ImageDigest = actengine.ImageDigest(digestCtx, env, cfg.Runner.Image)
	cancelDigest()
	coverage := "complete"
	for _, r := range results {
		if r.Status != "passed" && !(r.Status == "failed" && r.Provenance == "engine_exit" && r.ReasonCode == nil) {
			coverage = "partial"
		}
	}
	workflowCoverage := "complete"
	if coverage == "partial" || len(results) != len(inv.Jobs) || len(adjustments) > 0 {
		workflowCoverage = "partial"
	}
	rr := model.RunReport{RunID: runID, Repo: repo, Source: snap, Engine: env.Info, Jobs: results, Coverage: model.Coverage{Selected: coverage, Workflow: workflowCoverage}, Adjustments: adjustments, Warnings: warnings, DurationMs: time.Since(started).Milliseconds(), ReportPath: filepath.Join(runDir, "report.json")}
	envl := model.Envelope{SchemaVersion: 1, Command: "run", ExitCode: code, Result: rr}
	b, marshalErr := json.MarshalIndent(envl, "", "  ")
	if marshalErr != nil {
		return fail(marshalErr)
	}
	persistErr := writeMetadata(runDir, runID, started, originalCopy, preparedCopy)
	if persistErr == nil {
		persistErr = writeAtomic(rr.ReportPath, append(b, '\n'))
	}
	if persistErr != nil {
		if envl.ExitCode == 0 {
			envl.ExitCode = 2
		}
		envl.Error = &model.ErrorInfo{Code: "REPORT_WRITE_FAILED", Message: persistErr.Error()}
		return envl, persistErr
	}
	return envl, nil
}
func writeMetadata(dir, runID string, started time.Time, original, prepared string) error {
	hash := func(p string) (string, error) {
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
	originalHash, err := hash(original)
	if err != nil {
		return err
	}
	preparedHash, err := hash(prepared)
	if err != nil {
		return err
	}
	argv := map[string][]string{}
	paths, err := filepath.Glob(filepath.Join(dir, "argv-*.json"))
	if err != nil {
		return err
	}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var a []string
		if e := json.Unmarshal(b, &a); e != nil {
			return e
		}
		argv[filepath.Base(p)] = a
	}
	m := map[string]any{"runId": runID, "startedAt": started.UTC().Format(time.RFC3339Nano), "finishedAt": time.Now().UTC().Format(time.RFC3339Nano), "originalWorkflowSha256": originalHash, "preparedWorkflowSha256": preparedHash, "argv": argv}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "metadata.json"), append(b, '\n'))
}
func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func errorEnvelope(command string, err error) model.Envelope {
	code := 2
	var me *model.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (asModelError(err, &me) && me.Code == "CANCELLED") {
		code = 130
	}
	return model.Envelope{SchemaVersion: 1, Command: command, ExitCode: code, Error: &model.ErrorInfo{Code: errorCode(err), Message: err.Error()}}
}
func errorCode(err error) string {
	var me *model.Error
	if asModelError(err, &me) {
		return me.Code
	}
	return "CONFIG_INVALID"
}
func asModelError(err error, out **model.Error) bool { return errors.As(err, out) }
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func join(xs []string) string {
	if len(xs) == 0 {
		return "job is unsupported"
	}
	return strings.Join(xs, "; ")
}
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
