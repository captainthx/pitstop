package dockerchecks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pitstop/internal/model"
)

const (
	testcontainersDaemonImage  = "docker:29.8.2-dind"
	testcontainersDaemonAlias  = "pitstop-dind"
	testcontainersReadyTimeout = 45 * time.Second
)

type testcontainersService struct {
	env       Environment
	runDir    string
	checkID   string
	label     string
	networkID string
	network   string
	volume    string
	daemonID  string
	result    model.TestcontainersResult
	cleaned   bool
}

func provisionTestcontainers(ctx context.Context, env Environment, runDir, checkID, architecture string) (*testcontainersService, error) {
	return provisionTestcontainersWithImage(ctx, env, runDir, checkID, architecture)
}

func provisionTestcontainersWithImage(ctx context.Context, env Environment, runDir, checkID, architecture string, prepared ...imageInspect) (*testcontainersService, error) {
	label := "com.pitstop.run=" + filepath.Base(runDir)
	if err := markEngineTestcontainers(runDir); err != nil {
		return nil, err
	}
	s := &testcontainersService{
		env: env, runDir: runDir, checkID: checkID, label: label,
		network: "pitstop-" + filepath.Base(runDir) + "-" + checkID + "-net",
		// shortcut: a unique labelled volume is recoverable after partial container removal; upgrade when anonymous-volume ownership can be recovered safely.
		volume: "pitstop-" + filepath.Base(runDir) + "-" + checkID + "-dind-data",
		result: model.TestcontainersResult{DaemonImage: testcontainersDaemonImage, NetworkAlias: testcontainersDaemonAlias, Readiness: "provisioning"},
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			if s.daemonID != "" && s.result.Diagnostics == "" {
				s.result.Diagnostics = s.captureDiagnostics()
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.cleanup(cleanupCtx)
			cancel()
		}
	}()
	arch := architecture
	if arch == "" {
		arch = env.Info.Architecture
	}
	var info imageInspect
	var err error
	if len(prepared) > 0 {
		info = prepared[0]
	} else {
		info, err = prepareTestcontainersImage(ctx, env, arch)
		if err != nil {
			return s, err
		}
	}
	wantArch := strings.TrimPrefix(arch, "linux/")
	if info.Os != "linux" || info.Architecture != wantArch {
		return s, model.Err("CONFIG_INVALID", "Testcontainers daemon image does not match runner architecture", nil)
	}
	s.result.DaemonImageID, s.result.DaemonImageDigest = info.ID, imageDigest(info)
	imageRef := imageReference(info, testcontainersDaemonImage)

	networkOut, err := command(ctx, env, "network", "create", "--driver", "bridge", "--attachable", "--label", label, s.network).Output()
	if err != nil {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot create private Testcontainers network", err)
	}
	s.networkID = strings.TrimSpace(string(networkOut))
	if s.networkID == "" {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "Docker returned no Testcontainers network ID", nil)
	}
	owner, inspectErr := command(ctx, env, "network", "inspect", "--format", "{{index .Labels \"com.pitstop.run\"}}", s.networkID).Output()
	if inspectErr != nil || strings.TrimSpace(string(owner)) != strings.TrimPrefix(label, "com.pitstop.run=") {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "created network ownership could not be verified", inspectErr)
	}
	if err := recordEngineResource(runDir, "networkId", s.networkID); err != nil {
		return s, err
	}
	s.result.NetworkID = s.networkID

	volumeOut, err := command(ctx, env, "volume", "create", "--label", label, s.volume).Output()
	if err != nil {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot create Testcontainers daemon data volume", err)
	}
	if strings.TrimSpace(string(volumeOut)) != s.volume {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "Docker returned an unexpected Testcontainers volume", nil)
	}
	owner, inspectErr = command(ctx, env, "volume", "inspect", "--format", "{{index .Labels \"com.pitstop.run\"}}", s.volume).Output()
	if inspectErr != nil || strings.TrimSpace(string(owner)) != strings.TrimPrefix(label, "com.pitstop.run=") {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "created volume ownership could not be verified", inspectErr)
	}
	if err := recordEngineResource(runDir, "volumeName", s.volume); err != nil {
		return s, err
	}

	name := "pitstop-" + filepath.Base(runDir) + "-" + checkID + "-dind"
	createArgs := []string{"create", "--privileged", "--name", name, "--label", label, "--platform", arch, "--network", s.network, "--network-alias", testcontainersDaemonAlias, "--pull", "never", "--mount", "type=volume,source=" + s.volume + ",destination=/var/lib/docker", "--env", "DOCKER_TLS_CERTDIR=", "--entrypoint", "dockerd", imageRef, "--host=tcp://0.0.0.0:2375", "--host=unix:///var/run/docker.sock"}
	if err := saveArgv(runDir, checkID+"-testcontainers-daemon-create", createArgs); err != nil {
		return s, err
	}
	daemonOut, err := command(ctx, env, createArgs...).CombinedOutput()
	if err != nil {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot create Testcontainers daemon", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(daemonOut))))
	}
	s.daemonID = strings.TrimSpace(string(daemonOut))
	if s.daemonID == "" {
		return s, model.Err("TESTCONTAINERS_PROVISION_FAILED", "Docker returned no Testcontainers daemon ID", nil)
	}
	if err := recordEngineResource(runDir, "daemonId", s.daemonID); err != nil {
		return s, err
	}
	s.result.DaemonContainerID = s.daemonID
	if err := command(ctx, env, "start", s.daemonID).Run(); err != nil {
		s.result.Readiness = "failed"
		s.result.Diagnostics = s.captureDiagnostics()
		return s, model.Err("TESTCONTAINERS_DAEMON_NOT_READY", "Testcontainers daemon failed to start", err)
	}
	readyCtx, readyCancel := context.WithTimeout(ctx, testcontainersReadyTimeout)
	defer readyCancel()
	for {
		if out, probeErr := command(readyCtx, env, "exec", s.daemonID, "docker", "-H", "tcp://127.0.0.1:2375", "info", "--format", "{{.ServerVersion}}").Output(); probeErr == nil && strings.TrimSpace(string(out)) != "" {
			s.result.Readiness = "ready"
			cleanupOnError = false
			return s, nil
		}
		select {
		case <-readyCtx.Done():
			s.result.Readiness = "failed"
			s.result.Diagnostics = s.captureDiagnostics()
			return s, model.Err("TESTCONTAINERS_DAEMON_NOT_READY", "Testcontainers daemon did not become ready", readyCtx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func prepareTestcontainersImage(ctx context.Context, env Environment, architecture string) (imageInspect, error) {
	imageCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()
	info, err := inspectImage(imageCtx, env, testcontainersDaemonImage, architecture)
	if err != nil && !imageMissing(err) {
		return imageInspect{}, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot inspect Testcontainers daemon image", err)
	}
	if err != nil {
		if err := command(imageCtx, env, "pull", "--platform", architecture, testcontainersDaemonImage).Run(); err != nil {
			return imageInspect{}, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot prepare Testcontainers daemon image", err)
		}
		info, err = inspectImage(imageCtx, env, testcontainersDaemonImage, architecture)
	}
	if err != nil {
		return imageInspect{}, model.Err("TESTCONTAINERS_PROVISION_FAILED", "cannot inspect prepared Testcontainers daemon image", err)
	}
	return info, nil
}

func (s *testcontainersService) captureDiagnostics() string {
	if s == nil || s.daemonID == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := command(ctx, s.env, "logs", s.daemonID).CombinedOutput()
	if err != nil {
		out = append(out, []byte("\nlogs command failed: "+err.Error())...)
	}
	if len(out) > 16384 {
		out = out[len(out)-16384:]
	}
	path := filepath.Join(s.runDir, "testcontainers-daemon-"+s.checkID+".log")
	if err := os.WriteFile(path, out, 0600); err != nil {
		return ""
	}
	return path
}

func (s *testcontainersService) cleanup(ctx context.Context) error {
	if s == nil || s.cleaned {
		return nil
	}
	s.cleaned = true
	var first error
	if s.daemonID != "" {
		if err := removeOwnedContainer(ctx, s.env, s.daemonID, s.label); err != nil && !resourceMissing(err) {
			first = err
		}
	}
	if s.volume != "" {
		if err := removeOwnedVolume(ctx, s.env, s.volume, s.label); err != nil && !resourceMissing(err) && first == nil {
			first = err
		}
	}
	if s.networkID != "" {
		if err := removeOwnedNetwork(ctx, s.env, s.networkID, s.label); err != nil && !resourceMissing(err) && first == nil {
			first = err
		}
	}
	return first
}

func recordEngineResource(dir, key, value string) error {
	path := filepath.Join(dir, "engine-meta.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	m[key] = value
	plural := key + "s"
	var values []string
	if existing, ok := m[plural].([]any); ok {
		for _, item := range existing {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
	}
	seen := false
	for _, item := range values {
		if item == value {
			seen = true
			break
		}
	}
	if !seen {
		values = append(values, value)
	}
	m[plural] = values
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}

func writeEngineMeta(dir, dockerHost, label string) error {
	path := filepath.Join(dir, "engine-meta.json")
	m := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	m["dockerHost"], m["label"] = dockerHost, label
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}

func markEngineTestcontainers(dir string) error {
	path := filepath.Join(dir, "engine-meta.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	m["testcontainers"] = true
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0600)
}
