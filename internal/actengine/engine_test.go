package actengine

import (
	"context"
	"os"
	"path/filepath"
	"pitstop/internal/model"
	"testing"
	"time"
)

func TestRunDrainsBothStreamsAndMapsExit(t *testing.T) {
	d := t.TempDir()
	fake := filepath.Join(d, "act")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '{\"jobID\":\"verify\",\"jobResult\":\"success\"}\\n'\nprintf 'stderr\\n' >&2\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	e := Environment{ActPath: fake, Home: d, Image: "image", Architecture: "linux/arm64"}
	s := model.Snapshot{Path: d}
	p := model.Plan{ExecutionWorkflowPath: filepath.Join(d, "ci.yml")}
	j := model.PlannedJob{ID: "verify", Eligible: true}
	r, err := Run(context.Background(), e, s, p, j, d, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "passed" || r.EngineExitCode == nil || *r.EngineExitCode != 0 {
		t.Fatalf("result=%#v", r)
	}
}

func TestRunZeroWithoutSelectionEvidenceIsUnknown(t *testing.T) {
	d := t.TempDir()
	fake := filepath.Join(d, "act")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), Environment{ActPath: fake, Home: d}, model.Snapshot{Path: d}, model.Plan{ExecutionWorkflowPath: filepath.Join(d, "ci.yml")}, model.PlannedJob{ID: "verify", Eligible: true}, d, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "skipped" || r.ReasonCode == nil || *r.ReasonCode != "ENGINE_OUTCOME_UNKNOWN" {
		t.Fatalf("%#v", r)
	}
}
func TestRunTimeoutIsEngineFailure(t *testing.T) {
	d := t.TempDir()
	fake := filepath.Join(d, "act")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nsleep 2\n"), 0755); err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), Environment{ActPath: fake, Home: d}, model.Snapshot{Path: d}, model.Plan{ExecutionWorkflowPath: filepath.Join(d, "ci.yml")}, model.PlannedJob{ID: "verify", Eligible: true}, d, 20*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "failed" || r.ReasonCode == nil || *r.ReasonCode != "TIMEOUT" {
		t.Fatalf("%#v", r)
	}
}
