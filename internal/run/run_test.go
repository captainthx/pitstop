package run

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"testing"
	"time"
)

func TestExecuteMissingConfigIsActionable(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	e, err := Execute(context.Background(), Options{Repo: d})
	if err == nil || e.ExitCode != 2 || e.Error == nil || e.Error.Code != "CONFIG_INVALID" {
		t.Fatalf("envelope=%#v err=%v", e, err)
	}
}
func TestAcquireRepoLock(t *testing.T) {
	d := t.TempDir()
	first, err := acquireLock(d)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := acquireLock(d); err == nil {
		t.Fatal("expected lock conflict")
	}
}
func TestCancelledPreflightReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e, err := Execute(ctx, Options{Repo: t.TempDir()})
	if err == nil || e.ExitCode != 130 || e.Error == nil || e.Error.Code != "CANCELLED" {
		t.Fatalf("cancelled preflight: %#v %v", e, err)
	}
}
func TestMetadataRetainsConfigAndSnapshotHashes(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "config.json"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "effective-config.json"), []byte("effective"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeDockerMetadata(d, "run", time.Now(), model.Config{}, model.Snapshot{SnapshotSHA256: "abc"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["configSha256"] == nil || doc["effectiveConfigSha256"] == nil || doc["snapshotSha256"] != "abc" {
		t.Fatal(doc)
	}
}
