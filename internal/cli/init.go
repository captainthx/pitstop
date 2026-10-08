package cli

import (
	"context"
	"errors"
	"io"

	"github.com/spf13/cobra"
	"pitstop/internal/config"
	"pitstop/internal/model"
)

func initCmd(out io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	var opts config.InitOptions
	c := &cobra.Command{
		Use:   "init",
		Short: "Create a starter .pitstop.json configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Context().Err(); err != nil {
				return writeInitError(out, jsonOut, err)
			}
			resolved, err := config.ResolveRepo(*repo)
			if err != nil {
				return err
			}
			result, err := config.Init(cmd.Context(), resolved, opts)
			if err != nil {
				return writeInitError(out, jsonOut, err)
			}
			envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "init", ExitCode: 0, Result: result}
			return writeCommandResult(out, envelope, jsonOut)
		},
	}
	c.Flags().StringVar(&opts.Preset, "preset", "base", "starter preset (base, node-npm, node-pnpm, node-yarn-classic, node-yarn-berry, go, python-pip, python-uv, java-maven, java-gradle, dotnet, rust, php-composer, ruby-bundler, custom)")
	c.Flags().StringVar(&opts.Image, "image", "", "replace the runner image")
	c.Flags().StringArrayVar(&opts.Commands, "command", nil, "replace verify steps with a shell command (repeatable)")
	c.Flags().StringVar(&opts.Architecture, "arch", "", "container architecture (linux/arm64 or linux/amd64)")
	c.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview the generated configuration without writing it")
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}

func writeInitError(out io.Writer, jsonOut bool, err error) error {
	code, exitCode, message := "CONFIG_INVALID", 2, err.Error()
	if errors.Is(err, context.Canceled) {
		code, exitCode, message = "CANCELLED", 130, "initialization cancelled"
	}
	var me *model.Error
	if errors.As(err, &me) {
		code, message = me.Code, me.Message
	}
	e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "init", ExitCode: exitCode, Error: &model.ErrorInfo{Code: code, Message: message}}
	if werr := writeCommandResult(out, e, jsonOut); werr != nil {
		return werr
	}
	return exitError{code: exitCode}
}
