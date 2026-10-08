package cli

import (
	"errors"
	"github.com/spf13/cobra"
	"io"
	"path/filepath"
	"pitstop/internal/config"
	"pitstop/internal/model"
)

func validateCmd(out io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	var jobs []string
	var arch string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate the repository configuration without running checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Context().Err(); err != nil {
				return writeValidateError(out, jsonOut, err)
			}
			resolved, err := config.ResolveRepo(*repo)
			if err != nil {
				return err
			}
			if err := cmd.Context().Err(); err != nil {
				return writeValidateError(out, jsonOut, err)
			}
			configPath := filepath.Join(resolved, ".pitstop.json")
			cfg, err := config.Load(resolved)
			if err != nil {
				return writeValidateConfigError(out, jsonOut, resolved, configPath, err)
			}
			if err := cmd.Context().Err(); err != nil {
				return writeValidateError(out, jsonOut, err)
			}
			effective, err := config.ApplyOverrides(cfg, jobs, arch)
			if err != nil {
				return writeValidateConfigError(out, jsonOut, resolved, configPath, err)
			}
			result := validateResult(resolved, configPath, effective)
			envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "validate", ExitCode: 0, Result: result}
			if err := writeCommandResult(out, envelope, jsonOut); err != nil {
				return err
			}
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	c.Flags().StringSliceVar(&jobs, "job", nil, "configured check ID (repeatable or comma-separated)")
	c.Flags().StringVar(&arch, "arch", "", "override configured container architecture (linux/arm64 or linux/amd64)")
	return c
}

func validateResult(repo, configPath string, cfg model.Config) model.ValidateResult {
	r := model.ValidateResult{
		Repo:         repo,
		ConfigPath:   configPath,
		Valid:        true,
		SelectedJobs: append([]string(nil), cfg.Jobs...),
		Architecture: cfg.Runner.Architecture,
		CheckCount:   len(cfg.Checks),
		Issues:       []model.ConfigIssue{},
	}
	for _, check := range cfg.Checks {
		r.StepCount += len(check.Steps)
	}
	return r
}

func writeValidateConfigError(out io.Writer, jsonOut bool, repo, configPath string, err error) error {
	issue := model.ConfigIssue{Code: "FILE_INVALID", Path: "", Message: "configuration is invalid"}
	var ve *model.ValidationError
	if errors.As(err, &ve) {
		issue = ve.Issue
	}
	result := model.ValidateResult{Repo: repo, ConfigPath: configPath, Valid: false, SelectedJobs: []string{}, Issues: []model.ConfigIssue{issue}}
	e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "validate", ExitCode: 2, Result: result, Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: issue.Message}}
	if werr := writeCommandResult(out, e, jsonOut); werr != nil {
		return werr
	}
	return exitError{code: 2}
}

func writeValidateError(out io.Writer, jsonOut bool, err error) error {
	code, message, exit := "CANCELLED", "validation cancelled", 130
	if err != nil && err.Error() != "context canceled" {
		message = err.Error()
	}
	e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "validate", ExitCode: exit, Error: &model.ErrorInfo{Code: code, Message: message}}
	if werr := writeCommandResult(out, e, jsonOut); werr != nil {
		return werr
	}
	return exitError{code: exit}
}
