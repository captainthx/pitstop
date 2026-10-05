package config

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
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
	b, err := os.ReadFile(filepath.Join(repo, ".pitstop.json"))
	if err != nil {
		return model.Config{}, model.Err("CONFIG_INVALID", ".pitstop.json is required for run", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c model.Config
	if err := dec.Decode(&c); err != nil {
		return c, model.Err("CONFIG_INVALID", "invalid .pitstop.json", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return c, model.Err("CONFIG_INVALID", "config must contain one JSON document", nil)
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}
func ResolveWorkflow(repo, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return "", model.Err("CONFIG_INVALID", "workflow must be a repo-relative path", nil)
	}
	clean := filepath.Clean(relative)
	if filepath.Ext(clean) != ".yml" && filepath.Ext(clean) != ".yaml" {
		return "", model.Err("CONFIG_INVALID", "workflow must end in .yml or .yaml", nil)
	}
	path := filepath.Join(repo, clean)
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", model.Err("CONFIG_INVALID", "workflow does not exist", err)
	}
	root, _ := filepath.EvalSymlinks(repo)
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", model.Err("CONFIG_INVALID", "workflow escapes repository", nil)
	}
	return real, nil
}
func Context(ctx context.Context) context.Context { return ctx }
