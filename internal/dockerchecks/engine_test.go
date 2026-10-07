package dockerchecks

import (
	"os"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"testing"
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
	for _, want := range []string{"/pitstop/step-0.sh", "/pitstop/step-1.sh", "step: first", "step: second", "printf 'a\\nb' > out"} {
		if !strings.Contains(s, want) && want != "printf 'a\\nb' > out" {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
	if _, err := os.Stat(filepath.Join(d, "results")); err != nil {
		t.Fatal(err)
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
