package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"pitstop/internal/agentassets"
	"pitstop/internal/model"
)

func decodeSkillEnvelope(t *testing.T, data []byte) model.Envelope {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	var envelope model.Envelope
	if err := dec.Decode(&envelope); err != nil {
		t.Fatalf("decode envelope: %v; output=%q", err, data)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("expected one JSON envelope, got %v; output=%q", err, data)
	}
	return envelope
}

func TestSkillInstallProjectDryRunJSON(t *testing.T) {
	repo := cliInitRepo(t)
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	expectedFiles := agentassets.Files()
	if !sort.StringsAreSorted(expectedFiles) {
		t.Fatal("canonical bundle files are not sorted")
	}

	var out, errOut bytes.Buffer
	code := Execute([]string{
		"skill", "install", "--agent", "codex", "--scope", "project",
		"--repo", repo, "--dry-run", "--json",
	}, &out, &errOut)
	envelope := decodeSkillEnvelope(t, out.Bytes())
	if code != 0 {
		t.Errorf("expected success, code=%d stderr=%q envelope=%#v", code, errOut.String(), envelope)
	}
	if envelope.Command != "skill install" {
		t.Errorf("command=%q, want %q", envelope.Command, "skill install")
	}
	if envelope.ExitCode != 0 {
		t.Errorf("exitCode=%d, want 0", envelope.ExitCode)
	}

	result, ok := envelope.Result.(map[string]any)
	if !ok {
		t.Fatalf("result=%#v, want JSON object", envelope.Result)
	}
	if got := result["agent"]; got != "codex" {
		t.Errorf("agent=%v, want codex", got)
	}
	if got := result["scope"]; got != "project" {
		t.Errorf("scope=%v, want project", got)
	}
	if got := result["targetPath"]; got != target {
		t.Errorf("targetPath=%v, want %q", got, target)
	}
	if got := result["bundleVersion"]; got != agentassets.BundleVersion {
		t.Errorf("bundleVersion=%v, want %q", got, agentassets.BundleVersion)
	}
	if got := result["action"]; got != "install" {
		t.Errorf("action=%v, want install", got)
	}
	if got := result["installed"]; got != false {
		t.Errorf("installed=%v, want false", got)
	}
	var gotFiles []string
	for _, item := range result["files"].([]any) {
		gotFiles = append(gotFiles, item.(string))
	}
	if !reflect.DeepEqual(gotFiles, expectedFiles) {
		t.Errorf("files=%v, want %v", gotFiles, expectedFiles)
	}
	if !sort.StringsAreSorted(gotFiles) {
		t.Errorf("files are not sorted: %v", gotFiles)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dry-run created .agents: %v", err)
	}
}

func TestSkillInstallUserRejectsExplicitRepo(t *testing.T) {
	repo := cliInitRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	var out, errOut bytes.Buffer
	code := Execute([]string{
		"skill", "install", "--agent", "codex", "--scope", "user", "--repo", repo, "--json",
	}, &out, &errOut)
	if code != 2 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	e := decodeSkillEnvelope(t, out.Bytes())
	if e.Command != "skill install" || e.ExitCode != 2 || e.Error == nil || e.Error.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("envelope=%#v", e)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("user destination changed: %v", err)
	}
}

func TestSkillInstallBuiltCLIWorksOutsideSourceCheckout(t *testing.T) {
	repo := cliInitRepo(t)
	bin := filepath.Join(t.TempDir(), "pitstop")
	build := exec.Command("go", "build", "-o", bin, "./cmd/pitstop")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	cmd := exec.Command(bin, "skill", "install", "--agent", "codex", "--scope", "project", "--repo", repo, "--json")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("built CLI: %v: %s", err, out)
	}
	e := decodeSkillEnvelope(t, out)
	if e.Command != "skill install" || e.ExitCode != 0 {
		t.Fatalf("envelope=%#v", e)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "pitstop-config", "SKILL.md")); err != nil {
		t.Fatalf("installed bundle missing outside checkout: %v", err)
	}
}
