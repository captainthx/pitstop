package dockerchecks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pitstop/internal/model"
)

func TestProvisionTestcontainersOwnsPrivateResourcesAndCleansThem(t *testing.T) {
	d := t.TempDir()
	log := filepath.Join(d, "docker.log")
	docker := filepath.Join(d, "docker")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$PITSTOP_DOCKER_LOG"
case "$*" in
  *"image inspect"*) printf '{"Id":"sha256:dind","Os":"linux","Architecture":"amd64","RepoDigests":["docker@sha256:manifest"]}\n' ;;
  *"network create"*) printf 'network-id\n' ;;
  *"volume create"*) printf 'pitstop-run-check-dind-data\n' ;;
  *"create --privileged"*) printf 'daemon-id\n' ;;
  *"exec daemon-id docker -H tcp://127.0.0.1:2375 info"*) printf '29.8.2\n' ;;
  *"inspect --format"*) printf 'run\n' ;;
esac
`
	if err := os.WriteFile(docker, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PITSTOP_DOCKER_LOG", log)
	env := Environment{DockerPath: docker, DockerHost: "unix:///chosen.sock", Info: model.EngineInfo{Architecture: "linux/amd64"}}
	runDir := filepath.Join(d, "run")
	if err := os.MkdirAll(runDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "engine-meta.json"), []byte(`{"dockerHost":"unix:///chosen.sock","label":"com.pitstop.run=run"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := provisionTestcontainers(context.Background(), env, runDir, "check", "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}
	if s.result.Readiness != "ready" || s.result.DaemonImageID != "sha256:dind" || s.result.DaemonImageDigest == nil || s.result.NetworkID != "network-id" {
		t.Fatalf("unexpected daemon result: %#v", s.result)
	}
	if err := s.cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(b)
	if !strings.Contains(logText, "create --privileged") || !strings.Contains(logText, "--mount type=volume,source=pitstop-run-check-dind-data") {
		t.Fatalf("daemon was not isolated as expected: %s", logText)
	}
	if strings.Contains(logText, " -p ") || strings.Contains(logText, "--publish") {
		t.Fatalf("daemon published an outer port: %s", logText)
	}
	for _, want := range []string{"volume rm --force pitstop-run-check-dind-data", "network rm network-id"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("missing cleanup operation %q in %s", want, logText)
		}
	}
}

func TestRunnerForTestcontainersUsesOnlyPrivateDockerOverrides(t *testing.T) {
	s := &testcontainersService{network: "private-net"}
	args := runnerCreateArgs("runner", "com.pitstop.run=run", "linux/arm64", "sha256:runner", s)
	joined := strings.Join(args, " ")
	for _, forbidden := range []string{"--privileged", "--mount", "--publish", " -v ", "DOCKER_CONTEXT=", "DOCKER_AUTH_CONFIG="} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("runner args contain forbidden %q: %s", forbidden, joined)
		}
	}
	for _, required := range []string{"--network private-net", "DOCKER_HOST=tcp://pitstop-dind:2375", "TESTCONTAINERS_HOST_OVERRIDE=pitstop-dind", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("runner args missing %q: %s", required, joined)
		}
	}
}

func TestEngineMetadataPreservesResourcesAcrossChecks(t *testing.T) {
	d := t.TempDir()
	if err := writeEngineMeta(d, "unix:///chosen.sock", "com.pitstop.run=run"); err != nil {
		t.Fatal(err)
	}
	if err := recordEngineResource(d, "networkId", "network-one"); err != nil {
		t.Fatal(err)
	}
	if err := writeEngineMeta(d, "unix:///chosen.sock", "com.pitstop.run=run"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d, "engine-meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if values, ok := meta["networkIds"].([]any); !ok || len(values) != 1 || values[0] != "network-one" {
		t.Fatalf("owned resource history was lost: %#v", meta)
	}
}

func TestProvisionRejectsForeignExistingVolumeBeforeDaemonCreate(t *testing.T) {
	d := t.TempDir()
	log := filepath.Join(d, "docker.log")
	docker := filepath.Join(d, "docker")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$PITSTOP_DOCKER_LOG"
case "$*" in
  *"image inspect"*) printf '{"Id":"sha256:dind","Os":"linux","Architecture":"amd64","RepoDigests":["docker@sha256:manifest"]}\n' ;;
  *"network create"*) printf 'network-id\n' ;;
  *"network inspect"*) printf 'run\n' ;;
  *"volume create"*) printf 'pitstop-run-check-dind-data\n' ;;
  *"volume inspect"*) printf 'other-run\n' ;;
esac
`
	if err := os.WriteFile(docker, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PITSTOP_DOCKER_LOG", log)
	env := Environment{DockerPath: docker, DockerHost: "unix:///chosen.sock", Info: model.EngineInfo{Architecture: "linux/amd64"}}
	runDir := filepath.Join(d, "run")
	if err := os.MkdirAll(runDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "engine-meta.json"), []byte(`{"dockerHost":"unix:///chosen.sock","label":"com.pitstop.run=run"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := provisionTestcontainers(context.Background(), env, runDir, "check", "linux/amd64"); err == nil {
		t.Fatal("foreign existing volume was accepted")
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "create --privileged") {
		t.Fatalf("daemon was created despite foreign volume ownership: %s", b)
	}
}
