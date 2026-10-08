package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"pitstop/internal/agentassets"
	"pitstop/internal/model"
)

type InitOptions struct {
	Preset       string
	Image        string
	Architecture string
	Commands     []string
	DryRun       bool
}

const configName = ".pitstop.json"

func Init(ctx context.Context, repo string, opts InitOptions) (model.InitResult, error) {
	if err := ctx.Err(); err != nil {
		return model.InitResult{}, err
	}
	preset := opts.Preset
	if preset == "" {
		preset = "base"
	}
	result := model.InitResult{
		Repo:       repo,
		ConfigPath: filepath.Join(repo, configName),
		Preset:     preset,
		DryRun:     opts.DryRun,
		Warnings:   []string{},
	}

	var cfg model.Config
	if preset == "custom" {
		if opts.Image == "" || len(opts.Commands) == 0 {
			return result, model.Err("CONFIG_INVALID", "custom preset requires --image and at least one --command", nil)
		}
		cfg = model.Config{
			SchemaVersion: model.ConfigSchemaVersion,
			Jobs:          []string{"verify"},
			Runner:        model.RunnerConfig{Image: opts.Image},
			Checks:        []model.Check{{ID: "verify", Steps: commandSteps(opts.Commands)}},
		}
	} else {
		data, err := agentassets.Preset(preset)
		if err != nil {
			return result, model.Err("CONFIG_INVALID", "unsupported init preset", nil)
		}
		cfg, err = Parse(data)
		if err != nil {
			return result, err
		}
		if opts.Image != "" {
			cfg.Runner.Image = opts.Image
		}
		if len(opts.Commands) > 0 {
			cfg.Checks[0].Steps = commandSteps(opts.Commands)
		}
	}

	if opts.Architecture != "" {
		cfg.Runner.Architecture = opts.Architecture
	} else {
		arch, ok := hostArchitecture()
		if !ok {
			return result, model.Err("CONFIG_INVALID", "unsupported host architecture; specify --arch linux/arm64 or linux/amd64", nil)
		}
		cfg.Runner.Architecture = arch
		result.Warnings = append(result.Warnings, "architecture defaults to the host runtime; remote Docker architecture is not detected")
	}
	if preset == "base" && len(opts.Commands) == 0 {
		result.Warnings = append(result.Warnings, "base is a starter with an explicit failure placeholder; edit .pitstop.json before running")
	} else if preset != "custom" {
		result.Warnings = append(result.Warnings, "starter prerequisites are not checked; review the generated commands before running")
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		return result, model.Err("CONFIG_INVALID", "could not generate configuration", err)
	}
	// Validate the final bytes, including every override, before touching the destination.
	cfg, err = Parse(encoded)
	if err != nil {
		return result, err
	}
	result.Config = cfg
	result.Architecture = cfg.Runner.Architecture

	exists, err := destinationExists(repo)
	if err != nil {
		return result, model.Err("WRITE_FAILED", "could not inspect configuration destination", err)
	}
	result.DestinationExists = exists
	if opts.DryRun {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	root, err := os.OpenRoot(repo)
	if err != nil {
		return result, model.Err("WRITE_FAILED", "could not open repository", err)
	}
	defer root.Close()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	f, err := root.OpenFile(configName, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return result, model.Err("CONFIG_EXISTS", "configuration file already exists", nil)
		}
		return result, model.Err("WRITE_FAILED", "could not create configuration file", err)
	}
	claimed := true
	cleanup := func() {
		if claimed {
			_ = root.Remove(configName)
		}
	}
	if err := ctx.Err(); err != nil {
		_ = f.Close()
		cleanup()
		return result, err
	}
	if err := writeAll(f, encoded); err != nil {
		_ = f.Close()
		cleanup()
		return result, model.Err("WRITE_FAILED", "could not write configuration file", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return result, model.Err("WRITE_FAILED", "could not sync configuration file", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return result, model.Err("WRITE_FAILED", "could not close configuration file", err)
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return result, err
	}
	claimed = false
	result.Written = true
	return result, nil
}

func commandSteps(commands []string) []model.CheckStep {
	steps := make([]model.CheckStep, len(commands))
	for i, command := range commands {
		steps[i] = model.CheckStep{ID: "step-" + strconv.Itoa(i+1), Run: command}
	}
	return steps
}

func hostArchitecture() (string, bool) {
	switch runtime.GOARCH {
	case "arm64":
		return "linux/arm64", true
	case "amd64":
		return "linux/amd64", true
	default:
		return "", false
	}
}

func destinationExists(repo string) (bool, error) {
	_, err := os.Lstat(filepath.Join(repo, configName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func writeAll(f *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := f.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return errors.New("short write")
		}
		data = data[n:]
	}
	return nil
}
