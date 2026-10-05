package actengine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"pitstop/internal/model"
	"runtime"
	"strings"
)

type Environment struct {
	ActPath, DockerHost, Home, Architecture string
	Info                                    model.EngineInfo
}

func Check(ctx context.Context) (Environment, error) {
	act, err := exec.LookPath("act")
	if err != nil {
		return Environment{}, model.Err("TOOL_MISSING", "act is required", err)
	}
	out, err := exec.CommandContext(ctx, act, "--version").Output()
	if err != nil {
		return Environment{}, model.Err("ACT_VERSION_UNSUPPORTED", "cannot read act version", err)
	}
	v := strings.TrimSpace(string(out))
	if v != "act version 0.2.89" && v != "0.2.89" {
		return Environment{}, model.Err("ACT_VERSION_UNSUPPORTED", "Pitstop requires act 0.2.89; got "+v, nil)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Environment{}, model.Err("TOOL_MISSING", "git is required", err)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return Environment{}, model.Err("TOOL_MISSING", "docker is required", err)
	}
	contextName := os.Getenv("DOCKER_CONTEXT")
	host := ""
	if contextName != "" {
		out, err := exec.CommandContext(ctx, docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}", contextName).Output()
		if err != nil {
			return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot resolve Docker context endpoint", err)
		}
		host = strings.TrimSpace(string(out))
		if host == "" {
			return Environment{}, model.Err("DOCKER_UNAVAILABLE", "configured Docker context has no endpoint", nil)
		}
	} else {
		host = os.Getenv("DOCKER_HOST")
		if host == "" {
			contextOut, err := exec.CommandContext(ctx, docker, "context", "show").Output()
			if err != nil {
				return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot read active Docker context", err)
			}
			contextName = strings.TrimSpace(string(contextOut))
			if contextName == "" {
				contextName = "default"
			}
			out, err := exec.CommandContext(ctx, docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}", contextName).Output()
			if err != nil {
				return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot resolve Docker context endpoint", err)
			}
			host = strings.TrimSpace(string(out))
		}
	}
	if host == "" {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "active Docker context has no endpoint", nil)
	}
	if out, e := dockerCommand(ctx, docker, "--host", host, "version", "--format", "{{.Server.Version}}").Output(); e != nil || strings.TrimSpace(string(out)) == "" {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "Docker daemon is unavailable at resolved endpoint", e)
	}
	return Environment{ActPath: act, DockerHost: host, Info: model.EngineInfo{Name: "act", Version: v, Architecture: runtime.GOOS + "/" + runtime.GOARCH}}, nil
}

// An explicit endpoint must not be overridden by the user's active context.
func dockerCommand(ctx context.Context, docker string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, docker, args...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "DOCKER_CONTEXT=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	return cmd
}
func ImageDigest(ctx context.Context, env Environment, image string) *string {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil
	}
	out, err := dockerCommand(ctx, docker, "--host", env.DockerHost, "image", "inspect", image, "--format", "{{json .RepoDigests}}").Output()
	if err != nil {
		return nil
	}
	var digests []string
	if json.Unmarshal(out, &digests) != nil || len(digests) == 0 {
		return nil
	}
	for _, d := range digests {
		if strings.Contains(d, "@sha256:") {
			digest := strings.SplitN(d, "@", 2)[1]
			return &digest
		}
	}
	return nil
}
