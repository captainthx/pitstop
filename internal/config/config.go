package config

import (
	"bytes"
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
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	probeDec := json.NewDecoder(bytes.NewReader(b))
	if err := probeDec.Decode(&probe); err != nil {
		return model.Config{}, model.Err("CONFIG_INVALID", "invalid .pitstop.json", err)
	}
	if probe.SchemaVersion == 1 {
		return model.Config{}, model.Err("CONFIG_INVALID", "schemaVersion 1 act workflows are no longer supported; migrate to schemaVersion 2 Docker checks", nil)
	}
	if probe.SchemaVersion != model.ConfigSchemaVersion {
		return model.Config{}, model.Err("CONFIG_INVALID", "unsupported config schemaVersion", nil)
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
