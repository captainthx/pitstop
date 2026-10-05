package actengine

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"testing"
	"time"
)

func TestRunDrainsBothStreamsAndMapsExit(t *testing.T) {
	d := t.TempDir()
	fake := filepath.Join(d, "act")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '{\"jobID\":\"verify\",\"jobResult\":\"success\"}\\n'\nprintf 'stderr\\n' >&2\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	e := Environment{ActPath: fake, Home: d, Architecture: "linux/arm64"}
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

func TestTerminalEvidenceRejectsRawFailureAndOversizedRecords(t *testing.T) {
	for _, line := range []string{`{"jobID":"verify","jobResult":"failure"}`, `{"jobID":"verify","jobResult":"success","raw_output":true}`, `{"jobID":"verify","jobResult":"success"}` + strings.Repeat(" ", 2*1024*1024)} {
		var log bytes.Buffer
		if parseStream(strings.NewReader(line+"\n"), &log, nil, "verify") {
			t.Fatal("untrusted record became success evidence")
		}
		if log.String() != line+"\n" {
			t.Fatal("raw log lost bytes")
		}
	}
}

type failingLog struct{}

func (failingLog) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestLogWriteFailureIsNotSilentlyDiscarded(t *testing.T) {
	if _, err := readLineBounded(bufio.NewReader(strings.NewReader("raw\n")), failingLog{}); err == nil {
		t.Fatal("log failure was hidden")
	}
}
func TestCleanupRejectsInvalidOwnershipMetadata(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "engine-meta.json"), []byte(`{}`), 0600)
	if err := Cleanup(context.Background(), d); err == nil {
		t.Fatal("invalid ownership metadata accepted")
	}
}
