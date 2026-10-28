package dockerchecks

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"testing"
	"time"
)

func TestWriteScriptsPreservesOrderAndCommands(t *testing.T) {
	d := t.TempDir()
	c := model.Check{ID: "verify", Steps: []model.CheckStep{{ID: "first", Name: "Name with spaces", Run: "printf 'a\\nb' > out"}, {ID: "second", Run: "exit 7"}}}
	if err := writeScripts(d, c); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"/pitstop/step-0.sh", "/pitstop/step-1.sh", "step: first Name with spaces", "step: second"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	step, err := os.ReadFile(filepath.Join(d, "step-0.sh"))
	if err != nil || !strings.Contains(string(step), "printf 'a\\nb' > out") {
		t.Fatalf("step script does not preserve command: %q %v", step, err)
	}
	if _, err := os.Stat(filepath.Join(d, "results")); err != nil {
		t.Fatal(err)
	}
}

func TestCancelResultSkipsEveryStep(t *testing.T) {
	c := model.Check{ID: "verify", Steps: []model.CheckStep{{ID: "one", Run: "true"}, {ID: "two", Run: "true"}}}
	r := cancelResult(model.JobResult{ID: c.ID}, c, context.Canceled, time.Now())
	if r.Status != "skipped" || r.ReasonCode == nil || *r.ReasonCode != "CANCELLED" || len(r.Steps) != 2 {
		t.Fatalf("unexpected cancellation result: %#v", r)
	}
	for _, step := range r.Steps {
		if step.Status == nil || *step.Status != "skipped" {
			t.Fatalf("step was not skipped: %#v", step)
		}
	}
}

func TestCheckDeadlineStartsAfterImagePreparation(t *testing.T) {
	started := time.Now()
	jobCtx, cancel, err := startCheckDeadline(context.Background(), 10*time.Millisecond, func() error {
		time.Sleep(40 * time.Millisecond)
		return nil
	})
	defer cancel()
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) < 40*time.Millisecond {
		t.Fatal("preparation did not run before deadline creation")
	}
	if jobCtx.Err() != nil {
		t.Fatalf("check deadline was consumed during preparation: %v", jobCtx.Err())
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func TestCopyLogReportsShortWritesAndKeepsPrimaryLog(t *testing.T) {
	var log bytes.Buffer
	err := copyLog(strings.NewReader("output"), &log, shortWriter{})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected progress short write, got %v", err)
	}
	if log.String() != "output" {
		t.Fatalf("primary log was discarded: %q", log.String())
	}
}

func TestWaitForStreamsDrainsHighVolumeBeforeWait(t *testing.T) {
	cmd := exec.Command("sh", "-c", "yes x | head -c 4000000")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	streams := make(chan error, 2)
	go func() { streams <- copyLog(stdout, &output, nil) }()
	go func() { streams <- copyLog(stderr, io.Discard, nil) }()
	streamErr, drained := waitForStreams(context.Background(), streams, stdout, stderr)
	if !drained || streamErr != nil {
		t.Fatalf("streams did not drain: drained=%v err=%v", drained, streamErr)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 4000000 {
		t.Fatalf("captured %d bytes, want 4000000", output.Len())
	}
}
func TestReadResultRequiresEvidence(t *testing.T) {
	d := t.TempDir()
	c := model.Check{ID: "verify", Steps: []model.CheckStep{{ID: "check", Run: "true"}}}
	r, err := readResult(c, containerState{ExitCode: 0}, d, model.JobResult{ID: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "skipped" || r.ReasonCode == nil || *r.ReasonCode != "ENGINE_OUTCOME_UNKNOWN" {
		t.Fatalf("%#v", r)
	}
}
func TestReadResultMatchesFailureEvidence(t *testing.T) {
	d := t.TempDir()
	c := model.Check{ID: "verify", Steps: []model.CheckStep{{ID: "one", Run: "true"}, {ID: "two", Run: "exit 7"}, {ID: "three", Run: "true"}}}
	if err := os.WriteFile(filepath.Join(d, "step-0.exit"), []byte("0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "step-1.exit"), []byte("7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := readResult(c, containerState{ExitCode: 7}, d, model.JobResult{ID: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "failed" || r.EngineExitCode == nil || *r.EngineExitCode != 7 || r.Steps[2].Status == nil || *r.Steps[2].Status != "skipped" {
		t.Fatalf("%#v", r)
	}
	r, err = readResult(c, containerState{ExitCode: 8}, d, model.JobResult{ID: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "skipped" || r.ReasonCode == nil || *r.ReasonCode != "ENGINE_OUTCOME_UNKNOWN" {
		t.Fatalf("%#v", r)
	}
}
func TestReadResultRuntimeFailureDoesNotPassSteps(t *testing.T) {
	d := t.TempDir()
	c := model.Check{ID: "verify", Steps: []model.CheckStep{{ID: "one", Run: "true"}}}
	r, err := readResult(c, containerState{ExitCode: 137, OOMKilled: true}, d, model.JobResult{ID: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "failed" || r.ReasonCode == nil || *r.ReasonCode != "CONTAINER_RUNTIME_FAILED" || r.Steps[0].Status == nil || *r.Steps[0].Status != "skipped" {
		t.Fatalf("%#v", r)
	}
}
