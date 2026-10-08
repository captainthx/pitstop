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

// ApplyOverrides returns a validated effective config without mutating c.
func ApplyOverrides(c model.Config, jobs []string, architecture string) (model.Config, error) {
	effective := c
	if len(jobs) > 0 {
		configured := make(map[string]struct{}, len(c.Jobs))
		for _, id := range c.Jobs {
			configured[id] = struct{}{}
		}
		seen := make(map[string]struct{}, len(jobs))
		for _, id := range jobs {
			if _, ok := configured[id]; !ok {
				return model.Config{}, overrideIssue("SELECTION_INVALID", "job is not in configured selection")
			}
			if _, ok := seen[id]; ok {
				return model.Config{}, overrideIssue("SELECTION_INVALID", "job selection contains duplicates")
			}
			seen[id] = struct{}{}
		}
		effective.Jobs = append([]string(nil), jobs...)
	}
	if architecture != "" {
		effective.Runner.Architecture = architecture
	}
	if err := effective.Validate(); err != nil {
		return model.Config{}, err
	}
	return effective, nil
}

func overrideIssue(code, message string) error {
	return model.Err("CONFIG_INVALID", message, &model.ValidationError{Issue: model.ConfigIssue{Code: code, Message: message}})
}
