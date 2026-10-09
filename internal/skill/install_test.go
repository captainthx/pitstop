package skill

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

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
	original, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
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
	got, readErr = os.ReadFile(filepath.Join(target, "SKILL.md"))
	if readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("owned file changed: readErr=%v", readErr)
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

func TestInstallCancellationWinsGitResolutionSuccessAndFailure(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "failure"}[success], func(t *testing.T) {
			repo := skillTestRepo(t)
			fakeBin := t.TempDir()
			started := filepath.Join(t.TempDir(), "started")
			release := filepath.Join(t.TempDir(), "release")
			defer func() { _ = os.WriteFile(release, nil, 0600) }()
			git := filepath.Join(fakeBin, "git")
			script := "#!/bin/sh\nset -eu\n: > \"$PITSTOP_FAKE_GIT_STARTED\"\nwhile [ ! -e \"$PITSTOP_FAKE_GIT_RELEASE\" ]; do sleep 0.01; done\n"
			if success {
				script += "printf '%s\\n' \"$PITSTOP_FAKE_GIT_REPO\"\n"
			} else {
				script += "exit 1\n"
			}
			if err := os.WriteFile(git, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("PITSTOP_FAKE_GIT_STARTED", started)
			t.Setenv("PITSTOP_FAKE_GIT_RELEASE", release)
			t.Setenv("PITSTOP_FAKE_GIT_REPO", repo)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type result struct {
				value model.SkillInstallResult
				err   error
			}
			results := make(chan result, 1)
			go func() {
				value, err := Install(ctx, Options{Agent: "codex", Scope: "project", Repo: repo})
				results <- result{value: value, err: err}
			}()
			deadline := time.After(5 * time.Second)
			for {
				if _, err := os.Stat(started); err == nil {
					break
				}
				select {
				case <-deadline:
					t.Fatal("fake Git did not start")
				default:
					time.Sleep(5 * time.Millisecond)
				}
			}
			cancel()
			if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-results:
				var installErr *model.Error
				if got.err == nil || got.value.Installed || !errors.As(got.err, &installErr) || installErr.Code != "CANCELLED" {
					t.Fatalf("result=%#v err=%v", got.value, got.err)
				}
				if _, err := os.Stat(filepath.Join(repo, ".agents")); err == nil {
					t.Fatal("cancellation created skill directories before destination resolution completed")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("installation did not return after releasing fake Git")
			}
		})
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

func TestInstallForceUpgradesKnownHistoricalBundle(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeHistoricalDraft(t, target)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || !result.Installed {
		t.Fatalf("historical upgrade result=%#v err=%v", result, err)
	}
}

func TestInstallForceUpgradesTask6Bundle(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeTask6Fixture(t, target)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || !result.Installed {
		t.Fatalf("Task 6 upgrade result=%#v err=%v", result, err)
	}
}

func TestInstallForceUpgradesTask7Bundle(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeTask7Fixture(t, target)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || !result.Installed {
		t.Fatalf("Task 7 upgrade result=%#v err=%v", result, err)
	}
}

func TestInstallForceUpgradesBDEB767Bundle(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeTrustedFixture(t, target, "bdeb767")
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || !result.Installed {
		t.Fatalf("bdeb767 upgrade result=%#v err=%v", result, err)
	}
}

func TestTask6ManifestMatchesPredecessorFixture(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "agentassets", "testdata", "21cd1c2", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !trustedManifestContains(want) {
		t.Fatalf("trusted Task 6 manifest does not match predecessor fixture")
	}
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	writeTask6Fixture(t, target)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err != nil || !result.Installed {
		t.Fatalf("predecessor fixture upgrade result=%#v err=%v", result, err)
	}
}

func TestTask7ManifestMatchesPredecessorFixture(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "agentassets", "testdata", "d21a0af", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !trustedManifestContains(want) {
		t.Fatalf("trusted Task 7 manifest does not match predecessor fixture")
	}
}

func trustedManifestContains(want []byte) bool {
	for _, manifest := range agentassets.TrustedBundles() {
		if bytes.Equal(manifest, want) {
			return true
		}
	}
	return false
}

func writeHistoricalDraft(t *testing.T, target string) {
	t.Helper()
	manifests := agentassets.TrustedBundles()
	if len(manifests) < 5 {
		t.Fatal("historical manifest missing")
	}
	m, err := parseManifest(manifests[4])
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range m.Files {
		fixture := filepath.Join("..", "agentassets", "testdata", "ff87a16", filepath.FromSlash(entry.Path))
		data, readErr := os.ReadFile(fixture)
		if errors.Is(readErr, os.ErrNotExist) {
			fixture = filepath.Join("..", "agentassets", "testdata", "21cd1c2", filepath.FromSlash(entry.Path))
			data, readErr = os.ReadFile(fixture)
		}
		if errors.Is(readErr, os.ErrNotExist) {
			data, readErr = agentassets.ReadFile(entry.Path)
		}
		if readErr != nil {
			t.Fatalf("historical fixture %s: %v", entry.Path, readErr)
		}
		path := filepath.Join(target, filepath.FromSlash(entry.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.json"), manifests[4], 0600); err != nil {
		t.Fatal(err)
	}
}

func writeTask6Fixture(t *testing.T, target string) {
	t.Helper()
	writeTrustedFixture(t, target, "21cd1c2")
}

func writeTask7Fixture(t *testing.T, target string) {
	t.Helper()
	writeTrustedFixture(t, target, "d21a0af")
}

func writeTrustedFixture(t *testing.T, target, fixtureName string) {
	t.Helper()
	manifestPath := filepath.Join("..", "agentassets", "testdata", fixtureName, "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range m.Files {
		fixture := filepath.Join("..", "agentassets", "testdata", fixtureName, filepath.FromSlash(entry.Path))
		data, readErr := os.ReadFile(fixture)
		if readErr != nil {
			t.Fatalf("trusted fixture %s: %v", entry.Path, readErr)
		}
		path := filepath.Join(target, filepath.FromSlash(entry.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestInstallDryRunBlocksRetainedRecovery(t *testing.T) {
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	parent := filepath.Join(resolvedRepo, ".agents", "skills")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(parent, backupPrefix+"stale")
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, DryRun: true})
	var installErr *model.Error
	if err == nil || result.Action != "blocked" || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("retained recovery result=%#v err=%v", result, err)
	}
}

func TestInstallStagingFailureLeavesNoDestination(t *testing.T) {
	repo := skillTestRepo(t)
	previous := installWrite
	installWrite = func(*os.File, []byte) error { return errors.New("staging write fault") }
	defer func() { installWrite = previous }()
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("staging fault result=%#v err=%v", result, err)
	}
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	if _, statErr := os.Stat(filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination survived staging failure: %v", statErr)
	}
}

func TestInstallForceStagingFailurePreservesOwnedBundle(t *testing.T) {
	repo := skillTestRepo(t)
	if _, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo}); err != nil {
		t.Fatal(err)
	}
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	target := filepath.Join(resolvedRepo, ".agents", "skills", "pitstop-config")
	original := make(map[string][]byte)
	for _, path := range agentassets.Files() {
		data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		original[path] = data
	}
	previous := installWrite
	installWrite = func(*os.File, []byte) error { return errors.New("replacement staging write fault") }
	defer func() { installWrite = previous }()
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "SKILL_INSTALL_FAILED" {
		t.Fatalf("replacement staging fault result=%#v err=%v", result, err)
	}
	for path, want := range original {
		got, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("original %s changed: readErr=%v", path, readErr)
		}
	}
}

func TestInstallCancellationAfterBackupMoveRestoresOriginal(t *testing.T) {
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
	ctx, cancel := context.WithCancel(context.Background())
	previous := installAfterBackupMove
	installAfterBackupMove = cancel
	defer func() { installAfterBackupMove = previous }()
	result, err := Install(ctx, Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	var installErr *model.Error
	if err == nil || result.Installed || !errors.As(err, &installErr) || installErr.Code != "CANCELLED" {
		t.Fatalf("cancellation result=%#v err=%v", result, err)
	}
	got, readErr := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if readErr != nil || string(got) != string(original) {
		t.Fatalf("original not restored after cancellation: err=%v", readErr)
	}
}

func TestInstallForceRejectsMissingManifestAndInsideSymlink(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(string) error
	}{
		{"missing manifest", func(target string) error { return os.Remove(filepath.Join(target, "manifest.json")) }},
		{"inside symlink", func(target string) error {
			path := filepath.Join(target, "assets", "examples", "base.json")
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Symlink(filepath.Join(os.TempDir(), "outside-bundle"), path)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			if err := tc.edit(target); err != nil {
				t.Fatal(err)
			}
			result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
			if err == nil || result.Installed {
				t.Fatalf("unsafe bundle accepted: result=%#v err=%v", result, err)
			}
			if got, readErr := os.ReadFile(filepath.Join(target, "SKILL.md")); readErr != nil || !bytes.Equal(got, original) {
				t.Fatalf("owned file changed: readErr=%v", readErr)
			}
		})
	}
}

func TestInstallForceRejectsForgedManifest(t *testing.T) {
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
	manifestPath := filepath.Join(target, "manifest.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest = bytes.Replace(manifest, []byte(`"owner": "pitstop"`), []byte(`"owner": "forged"`), 1)
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, Force: true})
	if err == nil || result.Installed {
		t.Fatalf("forged manifest accepted: result=%#v err=%v", result, err)
	}
	if got, readErr := os.ReadFile(filepath.Join(target, "SKILL.md")); readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("owned file changed or disappeared: readErr=%v", readErr)
	}
}

func TestInstallPermissionFailureDoesNotPublish(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass permission checks")
	}
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	parent := filepath.Join(resolvedRepo, ".agents", "skills")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo})
	if err == nil || result.Installed {
		t.Fatalf("permission failure result=%#v err=%v", result, err)
	}
}

func TestInstallDryRunReportsRecoveryScanFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass permission checks")
	}
	repo := skillTestRepo(t)
	resolvedRepo, _ := filepath.EvalSymlinks(repo)
	parent := filepath.Join(resolvedRepo, ".agents", "skills")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	result, err := Install(context.Background(), Options{Agent: "codex", Scope: "project", Repo: repo, DryRun: true})
	if err == nil || result.Action != "blocked" {
		t.Fatalf("recovery scan failure result=%#v err=%v", result, err)
	}
}
