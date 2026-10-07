package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"pitstop/internal/config"
	"pitstop/internal/dockerchecks"
	"pitstop/internal/model"
	"pitstop/internal/report"
	"pitstop/internal/run"
	"time"
)

var Version = "0.1.0"
var Commit string
var BuildTime string

type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("pitstop exited %d", e.code) }
func Execute(args []string, stdout, stderr io.Writer) int {
	return ExecuteContext(context.Background(), args, stdout, stderr)
}
func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	jsonMode := false
	for _, a := range args {
		if a == "--json" || a == "--json=true" {
			jsonMode = true
		}
		if a == "--json=false" {
			jsonMode = false
		}
	}
	root := newRoot(ctx, stdout, stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if x, ok := err.(exitError); ok {
			return x.code
		}
		if jsonMode {
			_ = report.WriteEnvelope(stdout, model.Envelope{SchemaVersion: model.SchemaVersion, Command: command(args), ExitCode: 2, Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: err.Error()}})
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 2
	}
	return 0
}
func command(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		if a == "--repo" {
			_ = i
			continue
		}
		switch a {
		case "run", "doctor", "list", "version":
			return a
		}
	}
	return "pitstop"
}
func newRoot(ctx context.Context, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{Use: "pitstop", Short: "Run repository checks in isolated Docker containers.", Long: "Run configured checks in fresh Docker containers using the current working-tree snapshot.\n\nFeatures:\n  - Run selected checks without installing their toolchain on the host.\n  - Keep steps in one job filesystem while isolating jobs from one another.\n  - Capture step evidence, logs, reports, timeout, cancellation, and cleanup.\n\nConfigure .pitstop.json with schemaVersion 2, a Docker image, and checks.", Example: "  pitstop doctor --repo /path/to/repo\n  pitstop list --repo /path/to/repo\n  pitstop run --repo /path/to/repo --json", SilenceUsage: true, SilenceErrors: true}
	root.SetContext(ctx)
	root.SetOut(stdout)
	root.SetErr(stderr)
	var repo string
	root.PersistentFlags().StringVar(&repo, "repo", ".", "target repository path")
	root.AddCommand(versionCmd(stdout), doctorCmd(stdout, &repo), listCmd(stdout, &repo), runCmd(ctx, stdout, stderr, &repo))
	return root
}
func versionCmd(out io.Writer) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "version", Short: "Show version and build information", RunE: func(cmd *cobra.Command, _ []string) error {
		r := model.VersionResult{Version: Version}
		if Commit != "" {
			r.Commit = &Commit
		}
		if BuildTime != "" {
			r.BuildTime = &BuildTime
		}
		e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "version", ExitCode: 0, Result: r}
		if jsonOut {
			return report.WriteEnvelope(out, e)
		}
		_, err := fmt.Fprintf(out, "pitstop %s\n", Version)
		return err
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}
func doctorCmd(out io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "doctor", Short: "Check Git, config, Docker, and image readiness", RunE: func(cmd *cobra.Command, _ []string) error {
		r := model.DoctorResult{Checks: []model.DoctorCheck{}, Ready: true}
		check := func(name string, err error) {
			x := model.DoctorCheck{Name: name, Status: "ready", Message: "ready"}
			if err != nil {
				x.Status, x.Message, x.ReasonCode, r.Ready = "blocked", err.Error(), "CONFIG_INVALID", false
				var me *model.Error
				if errors.As(err, &me) {
					x.ReasonCode = me.Code
				}
			}
			r.Checks = append(r.Checks, x)
		}
		resolved, e := config.ResolveRepo(*repo)
		r.Repo = resolved
		check("repository", e)
		if e == nil {
			cfg, configErr := config.Load(resolved)
			e = configErr
			check("config", e)
			if e == nil {
				env, dockerErr := dockerchecks.Check(cmd.Context())
				if dockerErr == nil {
					status, message := dockerchecks.ImageReadiness(cmd.Context(), env, cfg.Runner)
					x := model.DoctorCheck{Name: "image", Status: status, Message: message}
					if status != "ready" {
						x.ReasonCode, r.Ready = "CONFIG_INVALID", false
					}
					r.Checks = append(r.Checks, x)
				}
			}
		}
		_, e = dockerchecks.Check(cmd.Context())
		check("docker", e)
		code := 0
		if !r.Ready {
			code = 2
		}
		if cmd.Context().Err() != nil {
			code = 130
		}
		env := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "doctor", ExitCode: code, Result: r}
		if jsonOut {
			e = report.WriteEnvelope(out, env)
		} else {
			e = report.Human(out, env)
		}
		if e != nil {
			return e
		}
		if code != 0 {
			return exitError{code}
		}
		return nil
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}
func listCmd(out io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "list", Short: "List configured checks and selection", RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := list(*repo)
		if err != nil {
			return err
		}
		e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: "list", ExitCode: 0, Result: r}
		if jsonOut {
			return report.WriteEnvelope(out, e)
		}
		return report.Human(out, e)
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}
func list(repo string) (model.ListResult, error) {
	r, err := config.ResolveRepo(repo)
	if err != nil {
		return model.ListResult{}, err
	}
	cfg, err := config.Load(r)
	if err != nil {
		return model.ListResult{}, err
	}
	selected := map[string]bool{}
	for _, j := range cfg.Jobs {
		selected[j] = true
	}
	out := model.ListResult{Repo: r, ConfigPath: ".pitstop.json", Jobs: []model.ListJob{}}
	for _, c := range cfg.Checks {
		out.Jobs = append(out.Jobs, model.ListJob{ID: c.ID, Name: c.ID, Selected: selected[c.ID], Eligible: true, Reasons: []string{}})
	}
	return out, nil
}
func runCmd(ctx context.Context, out, errOut io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	var jobs []string
	var timeout time.Duration
	var arch string
	c := &cobra.Command{Use: "run", Short: "Run selected checks in isolated Docker containers", Long: "Run configured checks from the current working-tree snapshot. Each selected check gets a fresh container; steps within a check share /workspace. The timeout applies per check and Ctrl+C cancels the run.", Example: "  pitstop run --repo /path/to/repo\n  pitstop run --repo /path/to/repo --job verify\n  pitstop run --repo /path/to/repo --timeout 10m\n  pitstop run --repo /path/to/repo --json > result.json 2> progress.log", RunE: func(cmd *cobra.Command, _ []string) error {
		if timeout <= 0 {
			return model.Err("CONFIG_INVALID", "timeout must be positive", nil)
		}
		progress := out
		if jsonOut {
			progress = errOut
		}
		e, err := run.Execute(ctx, run.Options{Repo: *repo, Jobs: jobs, Timeout: timeout, Architecture: arch, Progress: progress})
		if jsonOut {
			if werr := report.WriteEnvelope(out, e); werr != nil {
				return werr
			}
		} else {
			_ = report.Human(out, e)
		}
		if e.ExitCode != 0 {
			return exitError{e.ExitCode}
		}
		return err
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	c.Flags().StringSliceVar(&jobs, "job", nil, "configured check ID (repeatable or comma-separated)")
	c.Flags().DurationVar(&timeout, "timeout", 20*time.Minute, "per-check timeout")
	c.Flags().StringVar(&arch, "arch", "", "override configured container architecture (linux/arm64 or linux/amd64)")
	return c
}
