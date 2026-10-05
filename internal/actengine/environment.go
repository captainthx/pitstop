package actengine

import (
	"context"
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
	_ = docker
	contextName := os.Getenv("DOCKER_CONTEXT")
	host := ""
	if contextName != "" {
		out, _ := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}", contextName).Output()
		host = strings.TrimSpace(string(out))
	} else {
		host = os.Getenv("DOCKER_HOST")
		if host == "" {
			contextOut, _ := exec.CommandContext(ctx, "docker", "context", "show").Output()
			contextName = strings.TrimSpace(string(contextOut))
			if contextName == "" {
				contextName = "default"
			}
			out, _ := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}", contextName).Output()
			host = strings.TrimSpace(string(out))
		}
	}
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	if out, e := exec.CommandContext(ctx, "docker", "--host", host, "version", "--format", "{{.Server.Version}}").Output(); e != nil || strings.TrimSpace(string(out)) == "" {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "Docker daemon is unavailable at resolved endpoint", e)
	}
	return Environment{ActPath: act, DockerHost: host, Info: model.EngineInfo{Name: "act", Version: v, Architecture: runtime.GOOS + "/" + runtime.GOARCH}}, nil
}
