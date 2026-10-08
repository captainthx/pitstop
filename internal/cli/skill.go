package cli

import (
	"context"
	"errors"
	"io"

	"github.com/spf13/cobra"
	"pitstop/internal/model"
	"pitstop/internal/skill"
)

func skillCmd(out io.Writer, repo *string) *cobra.Command {
	c := &cobra.Command{Use: "skill", Short: "Manage installed agent skills"}
	c.AddCommand(skillInstallCmd(out, repo))
	return c
}

func skillInstallCmd(out io.Writer, repo *string) *cobra.Command {
	var opts skill.Options
	var jsonOut bool
	c := &cobra.Command{
		Use:   "install",
		Short: "Install the Pitstop config skill for an agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.Scope == "" {
				return writeSkillError(out, jsonOut, model.Err("SKILL_INSTALL_FAILED", "--scope is required", nil))
			}
			if opts.Agent == "" {
				return writeSkillError(out, jsonOut, model.Err("SKILL_INSTALL_FAILED", "--agent is required", nil))
			}
			if opts.Scope == "user" && (cmd.Flags().Changed("repo") || cmd.InheritedFlags().Changed("repo")) {
				return writeSkillError(out, jsonOut, model.Err("SKILL_INSTALL_FAILED", "--repo is not valid for user scope", nil))
			}
			if opts.Scope == "project" {
				opts.Repo = *repo
			}
			result, err := skill.Install(cmd.Context(), opts)
			if err != nil {
				return writeSkillResultError(out, jsonOut, result, err)
			}
			envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "skill install", ExitCode: 0, Result: result}
			return writeCommandResult(out, envelope, jsonOut)
		},
	}
	c.Flags().StringVar(&opts.Agent, "agent", "", "agent skill target (codex or claude)")
	c.Flags().StringVar(&opts.Scope, "scope", "", "installation scope (user or project)")
	c.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview without writing")
	c.Flags().BoolVar(&opts.Force, "force", false, "replace an owned Pitstop bundle")
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}

func writeSkillError(out io.Writer, jsonOut bool, err error) error {
	return writeSkillResultError(out, jsonOut, model.SkillInstallResult{}, err)
}

func writeSkillResultError(out io.Writer, jsonOut bool, result model.SkillInstallResult, err error) error {
	code, exitCode, message := "CONFIG_INVALID", 2, err.Error()
	if errors.Is(err, context.Canceled) {
		code, exitCode, message = "CANCELLED", 130, "skill installation cancelled"
	}
	var me *model.Error
	if errors.As(err, &me) {
		code, message = me.Code, me.Message
	}
	envelope := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "skill install", ExitCode: exitCode, Result: result, Error: &model.ErrorInfo{Code: code, Message: message}}
	if werr := writeCommandResult(out, envelope, jsonOut); werr != nil {
		return werr
	}
	return exitError{code: exitCode}
}
