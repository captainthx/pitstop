package run

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
	cache, _ := os.UserCacheDir()
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
	started := time.Now()
	repo, err := config.ResolveRepo(o.Repo)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	l, err := acquireLock(repo)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	defer l.Release()
	cfg, err := config.Load(repo)
	if err != nil {
		return errorEnvelope("run", err), err
	}
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
			return errorEnvelope("run", e), e
		}
	}
	workflowPath, err := config.ResolveWorkflow(repo, cfg.Workflow)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	inv, err := workflow.Inspect(repo, workflowPath)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	if len(o.Jobs) > 0 {
		cfg.Jobs = o.Jobs
	}
	if o.ForceIntegration && (cfg.Adapter != "recruit100-service-integration-v1" || !contains(cfg.Jobs, "integration")) {
		e := model.Err("CONFIG_INVALID", "--force-integration requires the service adapter and selected integration job", nil)
		return errorEnvelope("run", e), e
	}
	plan, err := workflow.Plan(repo, cfg, inv, o.ForceIntegration)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	env, err := actengine.Check(ctx)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	env.Info.Image = cfg.Runner.Image
	env.Info.Architecture = cfg.Runner.Architecture
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	cache, _ := os.UserCacheDir()
	runDir := filepath.Join(cache, "pitstop", "runs", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return errorEnvelope("run", err), err
	}
	env.Home = filepath.Join(runDir, "home")
	if err := os.MkdirAll(filepath.Join(env.Home, ".config"), 0700); err != nil {
		return errorEnvelope("run", err), err
	}
	defer os.RemoveAll(env.Home)
	_ = os.WriteFile(filepath.Join(runDir, "event.json"), []byte(`{"event":"pull_request"}`+"\n"), 0600)
	snap, err := snapshot.Capture(repo, runDir)
	if err != nil {
		return errorEnvelope("run", err), err
	}
	defer snapshot.Remove(snap)
	snap.CheckoutMode = "local-source-copy"
	snap.ExcludedPaths = nonNil(snap.ExcludedPaths)
	sourceWorkflow := filepath.Join(snap.Path, cfg.Workflow)
	capturedInv, capturedErr := workflow.Inspect(snap.Path, sourceWorkflow)
	if capturedErr != nil {
		return errorEnvelope("run", capturedErr), capturedErr
	}
	capturedPlan, capturedErr := workflow.Plan(snap.Path, cfg, capturedInv, o.ForceIntegration)
	if capturedErr != nil {
		return errorEnvelope("run", capturedErr), capturedErr
	}
	inv = capturedInv
	plan = capturedPlan
	originalCopy := filepath.Join(runDir, "original-workflow.yml")
	if b, e := os.ReadFile(sourceWorkflow); e == nil {
		_ = os.WriteFile(originalCopy, b, 0600)
	}
	adjustments := []model.Adjustment{}
	if o.ForceIntegration && cfg.Adapter == "recruit100-service-integration-v1" {
		prepared := filepath.Join(runDir, "prepared-workflow.yml")
		a, e := workflow.PrepareIntegration(sourceWorkflow, prepared)
		if e != nil {
			return errorEnvelope("run", e), e
		}
		adjustments = a
		plan.ExecutionWorkflowPath = prepared
	} else {
		plan.ExecutionWorkflowPath = sourceWorkflow
	}
	_ = writeMetadata(runDir, runID, started, workflowPath, plan.ExecutionWorkflowPath)
	var results []model.JobResult
	for idx, j := range plan.Jobs {
		if !j.Eligible {
			code := "WORKFLOW_UNSUPPORTED"
			if len(j.Reasons) > 0 && contains(j.Reasons, "integration requires") {
				code = "INTEGRATION_RUNTIME_UNSUPPORTED"
			}
			msg := join(j.Reasons)
			results = append(results, model.JobResult{ID: j.ID, Status: "blocked", Provenance: "preflight", ReasonCode: &code, Reason: &msg, Steps: []model.StepResult{}})
			continue
		}
		r, e := actengine.Run(ctx, env, snap, plan, j, runDir, o.Timeout, o.Progress)
		if e != nil {
			return errorEnvelope("run", e), e
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
	coverage := "complete"
	for _, r := range results {
		if r.Status != "passed" {
			coverage = "partial"
		}
	}
	workflowCoverage := "complete"
	if len(results) != len(inv.Jobs) || len(adjustments) > 0 {
		workflowCoverage = "partial"
	}
	rr := model.RunReport{RunID: runID, Repo: repo, Source: snap, Engine: env.Info, Jobs: results, Coverage: model.Coverage{Selected: coverage, Workflow: workflowCoverage}, Adjustments: adjustments, Warnings: warnings, DurationMs: time.Since(started).Milliseconds(), ReportPath: filepath.Join(runDir, "report.json")}
	envl := model.Envelope{SchemaVersion: 1, Command: "run", ExitCode: code, Result: rr}
	b, marshalErr := json.MarshalIndent(envl, "", "  ")
	if marshalErr != nil {
		return errorEnvelope("run", marshalErr), marshalErr
	}
	if writeErr := os.WriteFile(rr.ReportPath, append(b, '\n'), 0600); writeErr != nil {
		return errorEnvelope("run", writeErr), writeErr
	}
	_ = writeMetadata(runDir, runID, started, workflowPath, plan.ExecutionWorkflowPath)
	return envl, nil
}
func writeMetadata(dir, runID string, started time.Time, original, prepared string) error {
	hash := func(p string) string {
		f, e := os.Open(p)
		if e != nil {
			return ""
		}
		defer f.Close()
		h := sha256.New()
		_, _ = io.Copy(h, f)
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	argv := map[string][]string{}
	for _, p := range func() []string { ps, _ := filepath.Glob(filepath.Join(dir, "argv-*.json")); return ps }() {
		b, e := os.ReadFile(p)
		if e == nil {
			var a []string
			if json.Unmarshal(b, &a) == nil {
				argv[filepath.Base(p)] = a
			}
		}
	}
	m := map[string]any{"runId": runID, "startedAt": started.UTC().Format(time.RFC3339Nano), "finishedAt": time.Now().UTC().Format(time.RFC3339Nano), "originalWorkflowSha256": hash(original), "preparedWorkflowSha256": hash(prepared), "argv": argv}
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "metadata.json"), append(b, '\n'), 0600)
}
func errorEnvelope(command string, err error) model.Envelope {
	code := 2
	var me *model.Error
	if ok := asModelError(err, &me); ok && me.Code == "CANCELLED" {
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
func asModelError(err error, out **model.Error) bool {
	if e, ok := err.(*model.Error); ok {
		*out = e
		return true
	}
	return false
}
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
	return xs[0]
}
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
