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
	"pitstop/internal/config"
	"pitstop/internal/dockerchecks"
	"pitstop/internal/model"
	"pitstop/internal/report"
	"pitstop/internal/snapshot"
	"syscall"
	"time"
)

type Options struct {
	Repo                string
	Jobs                []string
	Timeout             time.Duration
	Architecture        string
	AllowTestcontainers bool
	Progress            io.Writer
}
type lock struct{ f *os.File }

// Keep captured-config validation tests deterministic without a racing writer.
var captureSnapshot = snapshot.CaptureContext

func (l *lock) Release() {
	if l != nil && l.f != nil {
		_ = l.f.Close()
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
		_ = f.Close()
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
	cfg, err := config.Load(repo)
	if err != nil {
		return fail(err)
	}
	originalConfig, _ := json.Marshal(cfg)
	cfg, err = config.ApplyOverrides(cfg, o.Jobs, o.Architecture)
	if err != nil {
		return fail(err)
	}
	for _, id := range cfg.Jobs {
		for _, check := range cfg.Checks {
			if check.ID == id && check.Testcontainers && !o.AllowTestcontainers {
				return fail(model.Err("TESTCONTAINERS_AUTH_REQUIRED", "selected check "+id+" requires --allow-testcontainers", nil))
			}
		}
	}
	l, err := acquireLock(repo)
	if err != nil {
		return fail(err)
	}
	defer l.Release()
	env, err := dockerchecks.Check(ctx)
	if err != nil {
		return fail(err)
	}
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	cache, err := os.UserCacheDir()
	if err != nil {
		return fail(err)
	}
	runDir := filepath.Join(cache, "pitstop", "runs", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return fail(err)
	}
	cleanupDone := false
	defer func() {
		if !cleanupDone {
			c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = dockerchecks.Cleanup(c, runDir)
		}
	}()
	snap, err := captureSnapshot(ctx, repo, runDir)
	if err != nil {
		return fail(err)
	}
	snap.CheckoutMode = "local-source-copy"
	defer snapshot.Remove(snap)
	capturedCfg, err := config.Load(snap.Path)
	if err != nil {
		return fail(err)
	}
	capturedBytes, _ := json.Marshal(capturedCfg)
	if string(capturedBytes) != string(originalConfig) {
		return fail(model.Err("SNAPSHOT_CHANGED", "config changed before capture", nil))
	}
	if err := os.WriteFile(filepath.Join(runDir, "config.json"), append(originalConfig, '\n'), 0600); err != nil {
		return fail(err)
	}
	effective, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(runDir, "effective-config.json"), append(effective, '\n'), 0600); err != nil {
		return fail(err)
	}
	env.Info.Architecture = cfg.Runner.Architecture
	env, err = dockerchecks.Prepare(ctx, env, cfg.Runner, o.Progress)
	if err != nil {
		return fail(err)
	}
	checks := map[string]model.Check{}
	for _, c := range capturedCfg.Checks {
		checks[c.ID] = c
	}
	results := []model.JobResult{}
	runWarnings := []string{}
	for i, id := range cfg.Jobs {
		if ctx.Err() != nil {
			for _, rest := range cfg.Jobs[i:] {
				results = append(results, skipped(rest, "CANCELLED", "run cancelled before scheduling job"))
			}
			break
		}
		check, ok := checks[id]
		if !ok {
			results = append(results, skipped(id, "ENGINE_OUTCOME_UNKNOWN", "selected check missing from captured config"))
			continue
		}
		r, e := dockerchecks.Run(ctx, env, snap, check, runDir, o.Timeout, o.Progress)
		if e != nil {
			runWarnings = append(runWarnings, e.Error())
			cancelledResult := r.Status == "skipped" && r.ReasonCode != nil && *r.ReasonCode == "CANCELLED"
			blockedTestcontainers := r.Status == "skipped" && r.ReasonCode != nil && (*r.ReasonCode == "TESTCONTAINERS_PROVISION_FAILED" || *r.ReasonCode == "TESTCONTAINERS_DAEMON_NOT_READY")
			if r.Status == "failed" || r.Status == "passed" || cancelledResult || blockedTestcontainers {
				results = append(results, r)
				continue
			}
			code := "ENGINE_OUTCOME_UNKNOWN"
			msg := e.Error()
			if ctx.Err() != nil {
				code = "CANCELLED"
			}
			if len(r.Steps) == 0 {
				r.Steps = []model.StepResult{}
			}
			r = model.JobResult{ID: id, Status: "skipped", Provenance: "unknown", ReasonCode: &code, Reason: &msg, Steps: r.Steps, LogPath: filepath.Join(runDir, id+".stdout"), Testcontainers: r.Testcontainers}
			if ctx.Err() != nil {
				results = append(results, r)
				for _, rest := range cfg.Jobs[i+1:] {
					results = append(results, skipped(rest, "CANCELLED", "run cancelled before scheduling job"))
				}
				break
			}
			results = append(results, r)
			continue
		}
		results = append(results, r)
	}
	code := report.ExitCode(results, ctx.Err() != nil)
	if code == 0 && len(runWarnings) > 0 {
		code = 2
	}
	warnings := append([]string{}, runWarnings...)
	if e := snapshot.Remove(snap); e != nil {
		warnings = append(warnings, "CLEANUP_INCOMPLETE: source snapshot: "+e.Error())
		if code == 0 {
			code = 2
		}
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cleanupErr := dockerchecks.Cleanup(cleanupCtx, runDir)
	cancel()
	cleanupDone = true
	if cleanupErr != nil {
		warnings = append(warnings, "CLEANUP_INCOMPLETE: "+cleanupErr.Error())
		if code == 0 {
			code = 2
		}
	}
	if ctx.Err() != nil {
		code = 130
	}
	coverage := "complete"
	for _, r := range results {
		if r.Status != "passed" && !(r.Status == "failed" && r.Provenance == "engine_exit" && (r.FailureCategory == nil || *r.FailureCategory != "engine")) {
			coverage = "partial"
			break
		}
	}
	rr := model.RunReport{RunID: runID, Repo: repo, Source: snap, Engine: env.Info, Jobs: results, Coverage: model.Coverage{Selected: coverage, Workflow: "not_applicable"}, Adjustments: []model.Adjustment{}, Warnings: warnings, DurationMs: time.Since(started).Milliseconds(), ReportPath: filepath.Join(runDir, "report.json")}
	envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "run", ExitCode: code, Result: rr}
	if ctx.Err() != nil {
		envelope.ExitCode = 130
	}
	b, marshalErr := json.MarshalIndent(envelope, "", "  ")
	if marshalErr != nil {
		return fail(marshalErr)
	}
	if err := writeDockerMetadata(runDir, runID, started, cfg, snap); err != nil {
		if ctx.Err() != nil {
			envelope.ExitCode = 130
		} else if envelope.ExitCode == 0 {
			envelope.ExitCode = 2
		}
		envelope.Error = &model.ErrorInfo{Code: "REPORT_WRITE_FAILED", Message: err.Error()}
		return envelope, err
	}
	if err := writeAtomic(rr.ReportPath, append(b, '\n')); err != nil {
		if ctx.Err() != nil {
			envelope.ExitCode = 130
		} else if envelope.ExitCode == 0 {
			envelope.ExitCode = 2
		}
		envelope.Error = &model.ErrorInfo{Code: "REPORT_WRITE_FAILED", Message: err.Error()}
		return envelope, err
	}
	// A signal can arrive after the first serialization but before the report
	// write completes. Re-serialize once so the persisted envelope and the
	// process result retain cancellation precedence.
	if ctx.Err() != nil {
		envelope.ExitCode = 130
		b, marshalErr = json.MarshalIndent(envelope, "", "  ")
		if marshalErr != nil {
			return fail(marshalErr)
		}
		if err := writeAtomic(rr.ReportPath, append(b, '\n')); err != nil {
			envelope.Error = &model.ErrorInfo{Code: "REPORT_WRITE_FAILED", Message: err.Error()}
			return envelope, err
		}
	}
	return envelope, nil
}

func skipped(id, code, msg string) model.JobResult {
	return model.JobResult{ID: id, Status: "skipped", Provenance: "unknown", ReasonCode: &code, Reason: &msg, Steps: []model.StepResult{}}
}
func writeDockerMetadata(dir, runID string, started time.Time, cfg model.Config, snap model.Snapshot) error {
	hash := func(path string) (string, error) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		h := sha256.Sum256(b)
		return fmt.Sprintf("%x", h[:]), nil
	}
	cfgHash, err := hash(filepath.Join(dir, "config.json"))
	if err != nil {
		return err
	}
	effectiveHash, err := hash(filepath.Join(dir, "effective-config.json"))
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
		if e = json.Unmarshal(b, &a); e != nil {
			return e
		}
		argv[filepath.Base(p)] = a
	}
	m := map[string]any{"runId": runID, "startedAt": started.UTC().Format(time.RFC3339Nano), "finishedAt": time.Now().UTC().Format(time.RFC3339Nano), "configSha256": cfgHash, "effectiveConfigSha256": effectiveHash, "snapshotSha256": snap.SnapshotSHA256, "argv": argv}
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
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func errorEnvelope(command string, err error) model.Envelope {
	code := 2
	var me *model.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &me) && me.Code == "CANCELLED") {
		code = 130
	}
	return model.Envelope{SchemaVersion: model.SchemaVersion, Command: command, ExitCode: code, Error: &model.ErrorInfo{Code: errorCode(err), Message: err.Error()}}
}
func errorCode(err error) string {
	var me *model.Error
	if errors.As(err, &me) {
		return me.Code
	}
	return "CONFIG_INVALID"
}
