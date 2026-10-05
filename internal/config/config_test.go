package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLoadStrictAndResolveWorkflow(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d, ".pitstop.json")
	valid := `{"schemaVersion":1,"workflow":".github/workflows/ci.yml","event":"pull_request","jobs":["verify"],"runner":{"label":"ubuntu-latest","image":"x","architecture":"linux/arm64"}}`
	if err := os.WriteFile(path, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(valid+"\n{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected trailing document rejection")
	}
	if err := os.MkdirAll(filepath.Join(d, ".github/workflows"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, ".github/workflows/ci.yml"), []byte("jobs: {}"), 0600); err != nil {
		t.Fatal(err)
	}
	wantWorkflow, _ := filepath.EvalSymlinks(filepath.Join(d, ".github/workflows/ci.yml"))
	if got, err := ResolveWorkflow(d, ".github/workflows/ci.yml"); err != nil || got != wantWorkflow {
		t.Fatalf("resolve=%q err=%v", got, err)
	}
}

func TestResolveRepoUsesGitTopLevel(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	n := filepath.Join(d, "nested", "space dir")
	if err := os.MkdirAll(n, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRepo(n)
	want, _ := filepath.EvalSymlinks(d)
	if err != nil || got != want {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
