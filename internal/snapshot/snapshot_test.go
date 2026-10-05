package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCaptureContextHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CaptureContext(ctx, t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("expected cancellation")
	}
}

func TestCaptureUsesWorkingTreeAndExcludesEnv(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("A"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "nested", "deep"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "nested", "deep", "source.txt"), []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "tracked.txt", "nested")
	runGit(t, repo, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("B"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Capture(repo, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(s)
	if got, err := os.ReadFile(filepath.Join(s.Path, "tracked.txt")); err != nil || string(got) != "B" {
		t.Fatalf("tracked=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(s.Path, "nested", "deep", "source.txt")); err != nil || string(got) != "nested" {
		t.Fatalf("nested=%q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(s.Path, "new.txt")); err != nil {
		t.Fatalf("untracked missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Path, ".env")); !os.IsNotExist(err) {
		t.Fatalf("env copied")
	}
	if len(s.ExcludedPaths) != 1 || s.ExcludedPaths[0] != ".env" {
		t.Fatalf("excluded=%v", s.ExcludedPaths)
	}
	if s.SnapshotSHA256 == "" || s.SnapshotSHA256 == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty snapshot digest: %q", s.SnapshotSHA256)
	}
}
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
}
