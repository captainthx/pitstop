package cli

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
	"pitstop/internal/model"
	"pitstop/internal/update"
)

func updateCmd(out, progress io.Writer, repo *string) *cobra.Command {
	var version string
	var jsonOut bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Update Pitstop and existing agent skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err != nil {
				return updateError(out, jsonOut, model.UpdateResult{}, model.Err("UPDATE_UNSAFE_DESTINATION", "could not resolve running executable", err))
			}
			result, runErr := update.Execute(cmd.Context(), update.Options{
				CurrentVersion:  Version,
				ExecutablePath:  exe,
				Repo:            *repo,
				RepoExplicit:    cmd.InheritedFlags().Changed("repo"),
				Version:         version,
				ExplicitVersion: cmd.Flags().Changed("version"),
				Progress:        progress,
			})
			if runErr != nil {
				return updateError(out, jsonOut, result, runErr)
			}
			e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "update", ExitCode: 0, Result: result}
			return writeCommandResult(out, e, jsonOut)
		},
	}
	c.Flags().StringVar(&version, "version", "", "target stable release (vMAJOR.MINOR.PATCH)")
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}

func updateError(out io.Writer, jsonOut bool, result model.UpdateResult, err error) error {
	code, exitCode, message := "UPDATE_FAILED", 2, err.Error()
	if errors.Is(err, context.Canceled) {
		code, exitCode, message = "CANCELLED", 130, "update cancelled"
	}
	var me *model.Error
	if errors.As(err, &me) {
		code, message = me.Code, me.Message
	}
	envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "update", ExitCode: exitCode, Result: result, Error: &model.ErrorInfo{Code: code, Message: message}}
	if werr := writeCommandResult(out, envelope, jsonOut); werr != nil {
		return werr
	}
	return exitError{code: exitCode}
}
