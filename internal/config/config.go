package config

import (
	"os/exec"
	"path/filepath"
	"strings"

	"pitstop/internal/model"
)

func ResolveRepo(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", model.Err("CONFIG_INVALID", "invalid repo path", err)
	}
	cmd := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", model.Err("CONFIG_INVALID", "path is not inside a Git repository", err)
	}
	return filepath.EvalSymlinks(strings.TrimSpace(string(out)))
}
func Load(repo string) (model.Config, error) {
	b, err := readConfig(repo)
	if err != nil {
		return model.Config{}, model.Err("CONFIG_INVALID", ".pitstop.json is required for run", err)
	}
	return Parse(b)
}
