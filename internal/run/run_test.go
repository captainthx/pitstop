package run

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"reflect"
	"testing"
	"time"
)

func TestExecuteMissingConfigIsActionable(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	e, err := Execute(context.Background(), Options{Repo: d})
	if err == nil || e.ExitCode != 2 || e.Error == nil || e.Error.Code != "CONFIG_INVALID" {
		t.Fatalf("envelope=%#v err=%v", e, err)
	}
}
func TestAcquireRepoLock(t *testing.T) {
	d := t.TempDir()
	lock, err := acquireLock(d)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := os.Stat(filepath.Join(d, ".pitstop-lock")); !os.IsNotExist(err) {
		t.Fatal("lock must not mutate checkout")
	}
	if _, err := acquireLock(d); err == nil {
		t.Fatal("expected lock conflict")
	}
	_ = os.Remove(filepath.Join(d, ".pitstop-lock"))
}

func TestMetadataRejectsMissingWorkflowEvidence(t *testing.T) {
	if err := writeMetadata(t.TempDir(), "id", time.Now(), "/missing-original", "/missing-prepared"); err == nil {
		t.Fatal("missing workflow evidence silently hashed as empty")
	}
}
func TestCoverageFailureStillExecutedAndBlockedIsPartial(t *testing.T) {
	// JSON coverage is a public result field, not a nested implementation detail.
	b, err := json.Marshal(model.RunReport{Coverage: model.Coverage{Selected: "complete", Workflow: "partial"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(b, &doc)
	if doc["selectedCoverage"] != "complete" || doc["workflowCoverage"] != "partial" {
		t.Fatal(string(b))
	}
}

func TestExecuteRetainsHashesAndCoverageAfterSnapshotCleanup(t *testing.T) {
	d := t.TempDir()
	for _, args := range [][]string{{"init", "-q", d}, {"-C", d, "config", "user.name", "Test"}, {"-C", d, "config", "user.email", "test@localhost"}} {
		if out, e := exec.Command("git", args...).CombinedOutput(); e != nil {
			t.Fatal(string(out), e)
		}
	}
	os.MkdirAll(filepath.Join(d, ".github/workflows"), 0700)
	os.WriteFile(filepath.Join(d, ".github/workflows/ci.yml"), []byte("on: pull_request\njobs:\n  verify:\n    runs-on: ubuntu-latest\n    steps:\n      - run: exit 7\n"), 0600)
	os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(`{"schemaVersion":1,"workflow":".github/workflows/ci.yml","event":"pull_request","jobs":["verify"],"runner":{"label":"ubuntu-latest","image":"test-image","architecture":"linux/arm64"}}`), 0600)
	for _, args := range [][]string{{"-C", d, "add", "."}, {"-C", d, "commit", "-qm", "fixture"}} {
		if out, e := exec.Command("git", args...).CombinedOutput(); e != nil {
			t.Fatal(string(out), e)
		}
	}
	tools := t.TempDir()
	os.WriteFile(filepath.Join(tools, "act"), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'act version 0.2.89'; exit 0; fi\nprintf '{\"jobID\":\"verify\",\"jobResult\":\"failure\"}\\n'\nexit 7\n"), 0755)
	os.WriteFile(filepath.Join(tools, "docker"), []byte("#!/bin/sh\ncase \"$*\" in *version*) echo 29.4.0;; *inspect*) echo '[]';; esac\n"), 0755)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_HOST", "unix:///test/socket")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	for _, opts := range []Options{{Repo: d, Architecture: "nonsense"}, {Repo: d, Jobs: []string{"verify", "verify"}}} {
		e, err := Execute(context.Background(), opts)
		if err == nil || e.ExitCode != 2 || e.Error == nil || e.Error.Code != "CONFIG_INVALID" {
			t.Fatalf("override accepted: %#v %v", e, err)
		}
	}
	e, err := Execute(context.Background(), Options{Repo: d, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r := e.Result.(model.RunReport)
	defer os.RemoveAll(filepath.Dir(r.ReportPath))
	if e.ExitCode != 1 || r.Coverage.Selected != "complete" || r.Coverage.Workflow != "complete" {
		t.Fatalf("executed failure coverage: %#v", e)
	}
	if r.Jobs[0].FailureCategory == nil || *r.Jobs[0].FailureCategory != "unknown" {
		t.Fatal(r.Jobs)
	}
	dir := filepath.Dir(r.ReportPath)
	if _, err := os.Stat(filepath.Join(dir, "snapshot")); !os.IsNotExist(err) {
		t.Fatal("snapshot was retained", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	for _, key := range []string{"originalWorkflowSha256", "preparedWorkflowSha256"} {
		if len(m[key].(string)) != 64 {
			t.Fatalf("missing retained hash %s: %s", key, b)
		}
	}
	b, err = os.ReadFile(r.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	var persisted, emitted any
	json.Unmarshal(b, &persisted)
	eb, _ := json.Marshal(e)
	json.Unmarshal(eb, &emitted)
	if !reflect.DeepEqual(persisted, emitted) {
		t.Fatal("stdout report differs from retained report")
	}

	os.WriteFile(filepath.Join(d, ".github/workflows/ci.yml"), []byte("on: pull_request\njobs:\n  changes:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo changes\n  integration:\n    runs-on: ubuntu-latest\n    needs: changes\n    if: needs.changes.outputs.persistence == 'true'\n    steps:\n      - run: go test -race -tags integration ./...\n"), 0600)
	os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(`{"schemaVersion":1,"workflow":".github/workflows/ci.yml","event":"pull_request","jobs":["integration"],"adapter":"recruit100-service-integration-v1","runner":{"label":"ubuntu-latest","image":"test-image","architecture":"linux/arm64"}}`), 0600)
	t.Setenv("DOCKER_HOST", "tcp://test:2375")
	blocked, err := Execute(context.Background(), Options{Repo: d, ForceIntegration: true})
	if err != nil {
		t.Fatal(err)
	}
	br := blocked.Result.(model.RunReport)
	defer os.RemoveAll(filepath.Dir(br.ReportPath))
	if blocked.ExitCode != 2 || br.Jobs[0].ReasonCode == nil || *br.Jobs[0].ReasonCode != "INTEGRATION_RUNTIME_UNSUPPORTED" {
		t.Fatalf("unsafe integration endpoint: %#v", blocked)
	}
}
