package dockerchecks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"runtime"
	"strings"
)

type Environment struct {
	DockerPath string
	DockerHost string
	Info       model.EngineInfo
	// ImageReference is the immutable reference accepted by docker create.
	// A multi-platform inspect can return a platform config ID that is not a
	// create reference, so this may be the target repository digest while
	// Info.ImageID remains the resolved platform config ID.
	ImageReference string
}

func Check(ctx context.Context) (Environment, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return Environment{}, model.Err("TOOL_MISSING", "git is required", err)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return Environment{}, model.Err("TOOL_MISSING", "docker is required", err)
	}
	host, err := resolveHost(ctx, docker)
	if err != nil {
		return Environment{}, err
	}
	out, err := command(ctx, Environment{DockerPath: docker, DockerHost: host}, "version", "--format", "{{.Server.Version}}").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "Docker daemon is unavailable at resolved endpoint", err)
	}
	return Environment{DockerPath: docker, DockerHost: host, Info: model.EngineInfo{Name: "docker", Version: strings.TrimSpace(string(out)), Architecture: runtime.GOOS + "/" + runtime.GOARCH}}, nil
}

func resolveHost(ctx context.Context, docker string) (string, error) {
	contextName := os.Getenv("DOCKER_CONTEXT")
	if contextName == "" {
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			return host, nil
		}
		out, err := exec.CommandContext(ctx, docker, "context", "show").Output()
		if err != nil {
			return "", model.Err("DOCKER_UNAVAILABLE", "cannot read active Docker context", err)
		}
		contextName = strings.TrimSpace(string(out))
		if contextName == "" {
			contextName = "default"
		}
	}
	out, err := exec.CommandContext(ctx, docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}", contextName).Output()
	if err != nil {
		return "", model.Err("DOCKER_UNAVAILABLE", "cannot resolve Docker context endpoint", err)
	}
	host := strings.TrimSpace(string(out))
	if host == "" {
		return "", model.Err("DOCKER_UNAVAILABLE", "configured Docker context has no endpoint", nil)
	}
	return host, nil
}

func command(ctx context.Context, env Environment, args ...string) *exec.Cmd {
	argv := append([]string{"--host", env.DockerHost}, args...)
	cmd := exec.CommandContext(ctx, env.DockerPath, argv...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "DOCKER_CONTEXT=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	return cmd
}

type imageInspect struct {
	ID           string   `json:"Id"`
	Os           string   `json:"Os"`
	Architecture string   `json:"Architecture"`
	RepoDigests  []string `json:"RepoDigests"`
}
type imageNotFoundError struct{}

func (imageNotFoundError) Error() string { return "configured image is not present locally" }

func ImageReadiness(ctx context.Context, env Environment, runner model.RunnerConfig) (string, string) {
	info, err := inspectImage(ctx, env, runner.Image, runner.Architecture)
	if err != nil && imageMissing(err) {
		return "ready", "image will be pulled during run"
	}
	if err != nil {
		return "blocked", "cannot inspect configured image: " + err.Error()
	}
	want := strings.TrimPrefix(runner.Architecture, "linux/")
	if info.Os != "linux" || info.Architecture != want {
		return "blocked", "local image platform does not match " + runner.Architecture
	}
	return "ready", "image available"
}
func imageMissing(err error) bool {
	if _, ok := err.(imageNotFoundError); ok {
		return true
	}
	if x, ok := err.(*exec.ExitError); ok {
		return strings.Contains(strings.ToLower(string(x.Stderr)), "no such image") || strings.Contains(strings.ToLower(string(x.Stderr)), "not found")
	}
	return false
}

func inspectImage(ctx context.Context, env Environment, image string, platform ...string) (imageInspect, error) {
	args := []string{"image", "inspect"}
	if len(platform) > 0 && platform[0] != "" {
		args = append(args, "--platform", platform[0])
	}
	args = append(args, image, "--format", "{{json .}}")
	out, err := command(ctx, env, args...).Output()
	if err != nil {
		return imageInspect{}, err
	}
	if t := strings.TrimSpace(string(out)); t == "" || t == "[]" {
		return imageInspect{}, imageNotFoundError{}
	}
	var info imageInspect
	if err := json.Unmarshal(out, &info); err != nil {
		return imageInspect{}, err
	}
	if info.ID == "" {
		return imageInspect{}, fmt.Errorf("image inspect returned no ID")
	}
	return info, nil
}

func imageDigest(info imageInspect) *string {
	for _, d := range info.RepoDigests {
		if i := strings.Index(d, "@sha256:"); i >= 0 {
			x := d[i+1:]
			return &x
		}
	}
	return nil
}

func Cleanup(ctx context.Context, runDir string) error {
	b, err := os.ReadFile(filepath.Join(runDir, "engine-meta.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var m struct {
		DockerHost, Label string
		Testcontainers    bool     `json:"testcontainers"`
		NetworkID         string   `json:"networkId"`
		VolumeName        string   `json:"volumeName"`
		NetworkIDs        []string `json:"networkIds"`
		VolumeNames       []string `json:"volumeNames"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.DockerHost == "" || m.Label != "com.pitstop.run="+filepath.Base(runDir) {
		return model.Err("CLEANUP_INCOMPLETE", "invalid runner ownership metadata", nil)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	env := Environment{DockerPath: docker, DockerHost: m.DockerHost}
	out, err := command(ctx, env, "ps", "-aq", "--filter", "label="+m.Label).Output()
	if err != nil {
		return err
	}
	currentIDs := strings.Fields(string(out))
	if currentIDs == nil {
		currentIDs = []string{}
	}
	retainedBytes, _ := os.ReadFile(filepath.Join(runDir, "owned-resources.json"))
	var retained []string
	_ = json.Unmarshal(retainedBytes, &retained)
	seen := map[string]bool{}
	for _, id := range retained {
		seen[id] = true
	}
	for _, id := range currentIDs {
		if !seen[id] {
			retained = append(retained, id)
			seen[id] = true
		}
	}
	if b, err := json.Marshal(retained); err != nil {
		return err
	} else if err := os.WriteFile(filepath.Join(runDir, "owned-resources.json"), append(b, '\n'), 0600); err != nil {
		return err
	}
	for _, id := range currentIDs {
		labelOut, e := command(ctx, env, "inspect", "--format", "{{index .Config.Labels \"com.pitstop.run\"}}", id).Output()
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(labelOut)) != strings.TrimPrefix(m.Label, "com.pitstop.run=") {
			return model.Err("CLEANUP_INCOMPLETE", "container ownership changed during cleanup", nil)
		}
		if e := command(ctx, env, "rm", "--force", "--volumes", id).Run(); e != nil {
			return model.Err("CLEANUP_INCOMPLETE", "owned runner cleanup failed", e)
		}
	}
	if !m.Testcontainers && len(m.VolumeNames) == 0 && len(m.NetworkIDs) == 0 && m.VolumeName == "" && m.NetworkID == "" {
		return nil
	}
	m.VolumeNames = appendUnique(m.VolumeNames, m.VolumeName)
	volumeIDs, err := command(ctx, env, "volume", "ls", "-q", "--filter", "label="+m.Label).Output()
	if err != nil {
		return err
	}
	for _, name := range strings.Fields(string(volumeIDs)) {
		m.VolumeNames = appendUnique(m.VolumeNames, name)
	}
	for _, name := range m.VolumeNames {
		if name != "" {
			if err := removeOwnedVolume(ctx, env, name, m.Label); err != nil && !resourceMissing(err) {
				return err
			}
		}
	}
	m.NetworkIDs = appendUnique(m.NetworkIDs, m.NetworkID)
	networkIDs, err := command(ctx, env, "network", "ls", "-q", "--filter", "label="+m.Label).Output()
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(string(networkIDs)) {
		m.NetworkIDs = appendUnique(m.NetworkIDs, id)
	}
	for _, id := range m.NetworkIDs {
		if id != "" {
			if err := removeOwnedNetwork(ctx, env, id, m.Label); err != nil && !resourceMissing(err) {
				return err
			}
		}
	}
	return nil
}

func appendUnique(values []string, extra string) []string {
	if extra == "" {
		return values
	}
	for _, value := range values {
		if value == extra {
			return values
		}
	}
	return append(values, extra)
}

func removeOwnedVolume(ctx context.Context, env Environment, name, label string) error {
	out, err := command(ctx, env, "volume", "inspect", "--format", "{{index .Labels \"com.pitstop.run\"}}", name).Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != strings.TrimPrefix(label, "com.pitstop.run=") {
		return model.Err("CLEANUP_INCOMPLETE", "volume ownership changed during cleanup", nil)
	}
	if err := command(ctx, env, "volume", "rm", "--force", name).Run(); err != nil {
		return model.Err("CLEANUP_INCOMPLETE", "owned Testcontainers volume cleanup failed", err)
	}
	return nil
}

func removeOwnedNetwork(ctx context.Context, env Environment, id, label string) error {
	out, err := command(ctx, env, "network", "inspect", "--format", "{{index .Labels \"com.pitstop.run\"}}", id).Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != strings.TrimPrefix(label, "com.pitstop.run=") {
		return model.Err("CLEANUP_INCOMPLETE", "network ownership changed during cleanup", nil)
	}
	if err := command(ctx, env, "network", "rm", id).Run(); err != nil {
		return model.Err("CLEANUP_INCOMPLETE", "owned Testcontainers network cleanup failed", err)
	}
	return nil
}

func resourceMissing(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	text := strings.ToLower(string(exitErr.Stderr))
	return strings.Contains(text, "no such") || strings.Contains(text, "not found")
}
