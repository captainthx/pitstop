package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecuteMissingConfigIsActionable(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	e, err := Execute(context.Background(), Options{Repo: d, JSON: true})
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
