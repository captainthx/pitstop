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
	"strconv"
	"strings"
	"time"
)

var Version = "0.1.0"
var Commit string
var BuildTime string

type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("pitstop exited %d", e.code) }

type outputError struct{ err error }

func (e outputError) Error() string { return e.err.Error() }
func (e outputError) Unwrap() error { return e.err }

func Execute(args []string, stdout, stderr io.Writer) int {
	return ExecuteContext(context.Background(), args, stdout, stderr)
}
func ExecuteContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	jsonOut := jsonMode(args)
	root := newRoot(ctx, stdout, stderr)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if cmd != nil {
		if flag := cmd.Flags().Lookup("json"); flag != nil && flag.Changed {
			if parsed, parseErr := strconv.ParseBool(flag.Value.String()); parseErr == nil {
				jsonOut = parsed
			}
		}
	}
	if err != nil {
		if x, ok := err.(exitError); ok {
			return x.code
		}
		var oe outputError
		if errors.As(err, &oe) {
			_, _ = fmt.Fprintln(stderr, report.DisplayMessage(oe.err.Error()))
			return 2
		}
		if jsonOut {
			name := command(args)
			if cmd != nil {
				switch cmd.Name() {
				case "run", "doctor", "list", "version", "validate", "init", "update":
					name = cmd.Name()
				case "install":
					if cmd.Parent() != nil && cmd.Parent().Name() == "skill" {
						name = "skill install"
					}
				}
			}
			e := model.Envelope{SchemaVersion: model.SchemaVersion, Command: name, ExitCode: 2, Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: err.Error()}}
			if werr := report.WriteEnvelope(stdout, e); werr != nil {
				_, _ = fmt.Fprintln(stderr, report.DisplayMessage(werr.Error()))
			}
		} else {
			_, _ = fmt.Fprintln(stderr, report.DisplayMessage(err.Error()))
		}
		return 2
	}
	return 0
}
func command(args []string) string {
	valueFlags := map[string]bool{"--repo": true, "--job": true, "--arch": true, "--timeout": true, "--command": true, "--preset": true, "--image": true, "--agent": true, "--scope": true, "--version": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		name, hasValue := splitFlag(a)
		if valueFlags[name] {
			if !hasValue && i+1 < len(args) && args[i+1] != "--" {
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch a {
		case "run", "doctor", "list", "version", "validate", "init", "update":
			return a
		case "skill":
			if i+1 < len(args) && args[i+1] == "install" {
				return "skill install"
			}
			return "skill"
		default:
			return "pitstop"
		}
	}
	return "pitstop"
}

func splitFlag(arg string) (string, bool) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i], true
	}
	return arg, false
}

func jsonMode(args []string) bool {
	valueFlags := map[string]bool{"--repo": true, "--job": true, "--arch": true, "--timeout": true, "--command": true, "--preset": true, "--image": true, "--agent": true, "--scope": true, "--version": true}
	mode := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		name, hasValue := splitFlag(a)
		if name == "--json" {
			if hasValue {
				parsed, err := strconv.ParseBool(strings.TrimPrefix(a, "--json="))
				mode = err == nil && parsed
			} else {
				mode = true
			}
			continue
		}
		if valueFlags[name] && !hasValue && i+1 < len(args) && args[i+1] != "--" {
			i++
		}
	}
	return mode
}

func writeCommandResult(out io.Writer, e model.Envelope, jsonOut bool) error {
	var err error
	if jsonOut {
		err = report.WriteEnvelope(out, e)
	} else {
		err = report.Human(out, e)
	}
	if err != nil {
		return outputError{err: err}
	}
	return nil
}
func newRoot(ctx context.Context, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{Use: "pitstop", Short: "Run repository checks in isolated Docker containers.", Long: "Run configured checks in fresh Docker containers using the current working-tree snapshot.\n\nFeatures:\n  - Run selected checks without installing their toolchain on the host.\n  - Keep steps in one job filesystem while isolating jobs from one another.\n  - Capture step evidence, logs, reports, timeout, cancellation, and cleanup.\n\nConfigure .pitstop.json with schemaVersion 2, a Docker image, and checks.", Example: "  pitstop doctor --repo /path/to/repo\n  pitstop list --repo /path/to/repo\n  pitstop run --repo /path/to/repo --json\n  pitstop skill install --agent codex --scope project --dry-run --json", SilenceUsage: true, SilenceErrors: true}
	root.SetContext(ctx)
	root.SetOut(stdout)
	root.SetErr(stderr)
	var repo string
	root.PersistentFlags().StringVar(&repo, "repo", ".", "target repository path")
	root.AddCommand(versionCmd(stdout), doctorCmd(stdout, &repo), listCmd(stdout, &repo), runCmd(ctx, stdout, stderr, &repo), validateCmd(stdout, &repo), initCmd(stdout, &repo), skillCmd(stdout, &repo), updateCmd(stdout, stderr, &repo))
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
			return writeCommandResult(out, e, true)
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
		e = writeCommandResult(out, env, jsonOut)
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
		return writeCommandResult(out, e, jsonOut)
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
		if werr := writeCommandResult(out, e, jsonOut); werr != nil {
			return werr
		}
		if e.ExitCode != 0 {
			return exitError{e.ExitCode}
		}
		if err != nil {
			return exitError{2}
		}
		return nil
	}}
	c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	c.Flags().StringSliceVar(&jobs, "job", nil, "configured check ID (repeatable or comma-separated)")
	c.Flags().DurationVar(&timeout, "timeout", 20*time.Minute, "per-check timeout")
	c.Flags().StringVar(&arch, "arch", "", "override configured container architecture (linux/arm64 or linux/amd64)")
	return c
}
