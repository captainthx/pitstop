package skill

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"pitstop/internal/agentassets"
	"pitstop/internal/model"
)

func skillTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return repo
}

func TestInstallProjectCopiesCompleteBundleAndDryRunIsReadOnly(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if result.TargetPath != target || result.Action != "install" || result.Installed || result.BundleVersion != agentassets.BundleVersion {
		t.Fatalf("dry-run result=%#v", result)
	}
	if !sort.StringsAreSorted(result.Files) || len(result.Files) != len(agentassets.Files()) {
		t.Fatalf("dry-run files=%v", result.Files)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created directories: %v", err)
	}

	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	if err != nil || !result.Installed {
		t.Fatalf("install result=%#v err=%v", result, err)
	}
	for _, path := range agentassets.Files() {
		info, err := os.Stat(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("file %s mode=%o", path, info.Mode().Perm())
		}
	}
	for _, dir := range []string{filepath.Join(repo, ".agents"), filepath.Join(repo, ".agents", "skills"), target} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 {
			t.Fatalf("directory %s mode=%o", dir, info.Mode().Perm())
		}
	}
	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, DryRun: true})
	if err != nil || result.Action != "blocked" || result.Installed {
		t.Fatalf("existing dry-run result=%#v err=%v", result, err)
	}
	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, DryRun: true, Force: true})
	if err != nil || result.Action != "replace" || result.Installed {
		t.Fatalf("force dry-run result=%#v err=%v", result, err)
	}
	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || result.Action != "replace" || !result.Installed {
		t.Fatalf("force replacement result=%#v err=%v", result, err)
	}
}

func TestInstallForcePreservesUnknownDestination(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if _, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo}); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("user-authored")
	if err := os.WriteFile(filepath.Join(target, "extra.txt"), sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("force unexpectedly succeeded or wrong error: result=%#v err=%v", result, err)
	}
	got, readErr := os.ReadFile(filepath.Join(target, "extra.txt"))
	if readErr != nil || string(got) != string(sentinel) {
		t.Fatalf("unknown file changed: %q err=%v", got, readErr)
	}
}

func TestInstallRejectsContentionAndCancellation(t *testing.T) {
	repo := skillTestRepo(t)
	parent := filepath.Join(repo, ".agents", "skills")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(parent, lockName)
	if err := os.WriteFile(lock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	if err == nil || result.Installed {
		t.Fatalf("contention result=%#v err=%v", result, err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = Install(ctx, Options{Agent: "codex", Scope: "project", Repo: repo})
	if err == nil || result.Installed {
		t.Fatalf("cancel result=%#v err=%v", result, err)
	}
}

func TestInstallPublishFailureRollsBackOriginal(t *testing.T) {
	repo := skillTestRepo(t)
	if _, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo}); err != nil {
		t.Fatal(err)
	}
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	original, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	previous := installRename
	installRename = func(root *os.Root, oldPath, newPath string) error {
		if strings.Contains(filepath.ToSlash(oldPath), stagePrefix) {
			return errors.New("publish fault")
		}
		return previous(root, oldPath, newPath)
	}
	defer func() { installRename = previous }()
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	got, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(got) != string(original) {
		t.Fatalf("original not restored: err=%v", err)
	}
}

func TestInstallFailedRollbackRetainsBackupAndBlocksReentry(t *testing.T) {
	repo := skillTestRepo(t)
	if _, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo}); err != nil {
		t.Fatal(err)
	}
	previous := installRename
	installRename = func(root *os.Root, oldPath, newPath string) error {
		path := filepath.ToSlash(oldPath)
		if strings.Contains(path, stagePrefix) || strings.Contains(path, backupPrefix) {
			return errors.New("rename fault")
		}
		return previous(root, oldPath, newPath)
	}
	defer func() { installRename = previous }()
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" || len(result.Warnings) == 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	parent := filepath.Join(resolvedRepo, ".agents", "skills")
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	var backup string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), backupPrefix) {
			backup = filepath.Join(parent, entry.Name())
		}
	}
	if backup == "" {
		t.Fatal("rollback backup was not retained")
	}
	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err == nil || result.Installed {
		t.Fatalf("retained backup was silently discarded: result=%#v err=%v", result, err)
	}
}

func TestInstallCancellationAfterPublishReportsCompleted(t *testing.T) {
	repo := skillTestRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	previous := installAfterPublish
	installAfterPublish = cancel
	defer func() { installAfterPublish = previous }()
	result, err := Install(ctx, Options{Agent: "codex", Scope: "project", Repo: repo})
	if err != nil || !result.Installed || len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "after publish") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestInstallRejectsAncestorAndFinalSymlinks(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(resolvedRepo, ".agents")); err != nil {
		t.Fatal(err)
	}
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	if err == nil || result.Installed {
		t.Fatalf("ancestor symlink accepted: result=%#v err=%v", result, err)
	}
	if err := os.Remove(filepath.Join(resolvedRepo, ".agents")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(resolvedRepo, ".agents", "skills"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")); err != nil {
		t.Fatal(err)
	}
	result, err = Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	if err == nil || result.Installed {
		t.Fatalf("final symlink accepted: result=%#v err=%v", result, err)
	}
}

func TestInstallFixedTargetMappings(t *testing.T) {
	home := t.TempDir()
	previousHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", previousHome)
	for _, tc := range []struct {
		agent string
		want  string
	}{
		{"codex", filepath.Join(home, ".agents", "skills", "pitstop-config")},
		{"claude", filepath.Join(home, ".claude", "skills", "pitstop-config")},
	} {
		result, err := Install(context.Background(), Options{Agent: tc.agent, Scope: "user", DryRun: true})
		if err != nil || result.TargetPath != tc.want {
			t.Fatalf("user %s result=%#v err=%v", tc.agent, result, err)
		}
	}
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	for _, tc := range []struct {
		agent string
		want  string
	}{
		{"codex", filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")},
		{"claude", filepath.Join(resolvedRepo, ".claude", "skills", "pitstop-config")},
	} {
		result, err := Install(context.Background(), Options{Agent: tc.agent, Scope: "project", Repo: repo, DryRun: true})
		if err != nil || result.TargetPath != tc.want {
			t.Fatalf("project %s result=%#v err=%v", tc.agent, result, err)
		}
	}
}
