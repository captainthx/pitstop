package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"pitstop/internal/config"
	"pitstop/internal/model"
	"pitstop/internal/report"
	"pitstop/internal/run"
	"pitstop/internal/workflow"
	"strings"
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
		if a == "--json" {
			jsonMode = true
		}
	}
	root := newRoot(ctx, stdout, stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if x, ok := err.(exitError); ok {
			return x.code
		}
		if jsonMode {
			e := model.Envelope{SchemaVersion: 1, Command: command(args), ExitCode: 2, Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: err.Error()}}
			_ = report.WriteEnvelope(stdout, e)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 2
	}
	return 0
}
func command(args []string) string {
	if len(args) > 0 && args[0] != "--json" {
		return args[0]
	}
	return "pitstop"
}
func newRoot(ctx context.Context, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{Use: "pitstop", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(stdout)
	root.SetErr(stderr)
	var repo string
	root.PersistentFlags().StringVar(&repo, "repo", ".", "repository path")
	root.AddCommand(versionCmd(stdout), doctorCmd(stdout, stderr, &repo), listCmd(stdout, stderr, &repo), runCmd(ctx, stdout, stderr, &repo))
	return root
}
func versionCmd(out io.Writer) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, _ []string) error {
		r := model.VersionResult{Version: Version}
		if Commit != "" {
			r.Commit = &Commit
		}
		if BuildTime != "" {
			r.BuildTime = &BuildTime
		}
		e := model.Envelope{SchemaVersion: 1, Command: "version", ExitCode: 0, Result: r}
		if jsonOut {
			return report.WriteEnvelope(out, e)
		}
		_, err := fmt.Fprintf(out, "pitstop %s\n", Version)
		return err
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}
func doctorCmd(out, errOut io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "doctor", RunE: func(cmd *cobra.Command, _ []string) error {
		r := model.DoctorResult{Checks: []model.DoctorCheck{}}
		if x, e := config.ResolveRepo(*repo); e == nil {
			r.Repo = x
		}
		for _, tool := range []string{"git", "act", "docker"} {
			if _, e := exec.LookPath(tool); e != nil {
				r.Checks = append(r.Checks, model.DoctorCheck{Name: tool, Status: "blocked", ReasonCode: "TOOL_MISSING", Message: "not found"})
			} else {
				r.Checks = append(r.Checks, model.DoctorCheck{Name: tool, Status: "ready", Message: "found"})
			}
		}
		if _, e := config.Load(r.Repo); e != nil {
			r.Checks = append(r.Checks, model.DoctorCheck{Name: "config", Status: "blocked", ReasonCode: "CONFIG_INVALID", Message: e.Error()})
		} else {
			r.Checks = append(r.Checks, model.DoctorCheck{Name: "config", Status: "ready", Message: "valid"})
		}
		if out, e := exec.CommandContext(cmd.Context(), "docker", "version", "--format", "{{.Server.Version}}").Output(); e != nil || len(strings.TrimSpace(string(out))) == 0 {
			r.Checks = append(r.Checks, model.DoctorCheck{Name: "docker-daemon", Status: "blocked", ReasonCode: "DOCKER_UNAVAILABLE", Message: "Docker daemon unavailable"})
		} else {
			r.Checks = append(r.Checks, model.DoctorCheck{Name: "docker-daemon", Status: "ready", Message: strings.TrimSpace(string(out))})
		}
		r.Ready = true
		for _, check := range r.Checks {
			if check.Status != "ready" {
				r.Ready = false
			}
		}
		e := model.Envelope{SchemaVersion: 1, Command: "doctor", ExitCode: 0, Result: r}
		if jsonOut {
			return report.WriteEnvelope(out, e)
		}
		return report.Human(out, e)
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return c
}
func listCmd(out, errOut io.Writer, repo *string) *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := list(*repo)
		if err != nil {
			return err
		}
		e := model.Envelope{SchemaVersion: 1, Command: "list", ExitCode: 0, Result: r}
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
		if !errors.Is(err, os.ErrNotExist) {
			return model.ListResult{}, err
		}
		cfg = model.Config{Workflow: ".github/workflows/ci.yml"}
	}
	p, err := config.ResolveWorkflow(r, cfg.Workflow)
	if err != nil {
		return model.ListResult{}, err
	}
	inv, err := workflow.Inspect(r, p)
	if err != nil {
		return model.ListResult{}, err
	}
	selected := map[string]bool{}
	for _, j := range cfg.Jobs {
		selected[j] = true
	}
	out := model.ListResult{Repo: r, WorkflowPath: cfg.Workflow, Jobs: []model.ListJob{}}
	for _, j := range inv.Jobs {
		reasons := j.CapabilityReasons
		if reasons == nil {
			reasons = []string{}
		}
		out.Jobs = append(out.Jobs, model.ListJob{ID: j.ID, Name: j.Name, Selected: selected[j.ID], Eligible: len(reasons) == 0, Reasons: reasons})
	}
	return out, nil
}
func runCmd(ctx context.Context, out, errOut io.Writer, repo *string) *cobra.Command {
	var jsonOut, force bool
	var jobs []string
	var timeout time.Duration
	var arch string
	c := &cobra.Command{Use: "run", RunE: func(cmd *cobra.Command, _ []string) error {
		if timeout <= 0 {
			return model.Err("CONFIG_INVALID", "timeout must be positive", nil)
		}
		progress := out
		if jsonOut {
			progress = errOut
		}
		e, err := run.Execute(ctx, run.Options{Repo: *repo, Jobs: jobs, Timeout: timeout, Architecture: arch, ForceIntegration: force, JSON: jsonOut, Progress: progress})
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
	c.Flags().StringSliceVar(&jobs, "job", nil, "selected job (repeatable)")
	c.Flags().DurationVar(&timeout, "timeout", 20*time.Minute, "per-job timeout")
	c.Flags().StringVar(&arch, "arch", "", "container architecture")
	c.Flags().BoolVar(&force, "force-integration", false, "enable guarded service integration adapter")
	return c
}

var _ = strings.TrimSpace
