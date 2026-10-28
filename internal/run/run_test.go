package run

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"pitstop/internal/snapshot"
	"strings"
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

func TestExecuteBlocksTestcontainersBeforeDocker(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	marker := filepath.Join(d, "docker-called")
	fakeBin := t.TempDir()
	docker := filepath.Join(fakeBin, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nprintf called > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	config := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["unit","integration"],"checks":[{"id":"unit","steps":[{"id":"check","run":"true"}]},{"id":"integration","testcontainers":true,"steps":[{"id":"check","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := Execute(context.Background(), Options{Repo: d, Jobs: []string{"unit", "integration"}})
	if err == nil || e.ExitCode != 2 || e.Error == nil || e.Error.Code != "TESTCONTAINERS_AUTH_REQUIRED" {
		t.Fatalf("envelope=%#v err=%v", e, err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("Docker was invoked before Testcontainers authorization")
	}
}

func TestExecuteRejectsInvalidOriginalBeforeDocker(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	marker := filepath.Join(d, "docker-called")
	fakeBin := t.TempDir()
	docker := filepath.Join(fakeBin, "docker")
	if err := os.WriteFile(docker, []byte("#!/bin/sh\nprintf called > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	bad := strings.Replace(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`, `"run":"true"`, `"run":"echo\u0000secret"`, 1)
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(bad), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := Execute(context.Background(), Options{Repo: d})
	if err == nil || e.Error == nil || e.Error.Code != "CONFIG_INVALID" {
		t.Fatalf("envelope=%#v err=%v", e, err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("Docker was invoked for invalid original config")
	}
}

func TestExecuteRejectsInvalidCapturedConfigBeforePrepare(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	valid := `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", d, "add", ".pitstop.json").CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	if out, err := exec.Command("git", "-C", d, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-qm", "config").CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	fakeBin := t.TempDir()
	dockerLog := filepath.Join(d, "docker.log")
	docker := filepath.Join(fakeBin, "docker")
	dockerScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$PITSTOP_DOCKER_LOG\"\ncase \"$*\" in *'version --format'*) printf '25.0\\n';; esac\n"
	if err := os.WriteFile(docker, []byte(dockerScript), 0700); err != nil {
		t.Fatal(err)
	}
	invalidCaptured := strings.Replace(valid, `"run":"true"`, `"run":"echo\u0000captured"`, 1)
	previousCapture := captureSnapshot
	t.Cleanup(func() { captureSnapshot = previousCapture })
	captureSnapshot = func(ctx context.Context, repo, runDir string) (model.Snapshot, error) {
		snap, captureErr := snapshot.CaptureContext(ctx, repo, runDir)
		if captureErr == nil {
			if writeErr := os.WriteFile(filepath.Join(snap.Path, ".pitstop.json"), []byte(invalidCaptured), 0600); writeErr != nil {
				_ = snapshot.Remove(snap)
				return model.Snapshot{}, writeErr
			}
		}
		return snap, captureErr
	}
	t.Setenv("PITSTOP_DOCKER_LOG", dockerLog)
	t.Setenv("DOCKER_HOST", "unix:///chosen.sock")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	e, err := Execute(context.Background(), Options{Repo: d})
	if err == nil || e.Error == nil || e.Error.Code != "CONFIG_INVALID" {
		t.Fatalf("envelope=%#v err=%v", e, err)
	}
	log, readErr := os.ReadFile(dockerLog)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(log), "image inspect") {
		t.Fatalf("Docker image preparation ran after invalid captured config: %s", log)
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
