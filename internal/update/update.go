package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pitstop"
	"pitstop/internal/config"
	"pitstop/internal/model"
)

const officialRepo = "https://github.com/captainthx/pitstop"

type Options struct {
	CurrentVersion  string
	ExecutablePath  string
	Repo            string
	RepoExplicit    bool
	Version         string
	ExplicitVersion bool
	Progress        io.Writer
}

type stableVersion [3]int

func parseStableVersion(value string) (stableVersion, error) {
	if !strings.HasPrefix(value, "v") {
		return stableVersion{}, errors.New("stable version must start with v")
	}
	parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
	if len(parts) != 3 {
		return stableVersion{}, errors.New("stable version must be vMAJOR.MINOR.PATCH")
	}
	var out stableVersion
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return stableVersion{}, errors.New("stable version contains an invalid number")
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return stableVersion{}, errors.New("stable version contains an invalid number")
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return stableVersion{}, errors.New("stable version contains an invalid number")
		}
		out[i] = n
	}
	return out, nil
}

func versionText(value string) string {
	if strings.HasPrefix(value, "v") {
		return value
	}
	return "v" + value
}

func (v stableVersion) compare(other stableVersion) int {
	for i := range v {
		if v[i] < other[i] {
			return -1
		}
		if v[i] > other[i] {
			return 1
		}
	}
	return 0
}

type updateTarget struct {
	model.UpdateTarget
	repo string
}

type childEnvelope struct {
	SchemaVersion int              `json:"schemaVersion"`
	Command       string           `json:"command"`
	ExitCode      int              `json:"exitCode"`
	Result        map[string]any   `json:"result"`
	Error         *model.ErrorInfo `json:"error"`
}

type childStartError struct{ err error }

func (e childStartError) Error() string { return e.err.Error() }
func (e childStartError) Unwrap() error { return e.err }

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.Host != "github.com" {
			return errors.New("latest release redirected outside github.com")
		}
		return nil
	},
}

var runInstaller = runEmbeddedInstaller
var resolveLatestRelease = resolveLatest

func Execute(ctx context.Context, opts Options) (result model.UpdateResult, retErr error) {
	result.ExecutablePath = opts.ExecutablePath
	result.PreviousVersion = opts.CurrentVersion
	result.Targets = []model.UpdateTarget{}
	result.CLIStatus = "not_attempted"
	if opts.Progress == nil {
		opts.Progress = io.Discard
	}
	if err := ctx.Err(); err != nil {
		return result, model.Err("CANCELLED", "update cancelled", err)
	}
	if opts.ExplicitVersion {
		if opts.Version == "" {
			return result, model.Err("UPDATE_INVALID_VERSION", "invalid target version", nil)
		}
		if _, err := parseStableVersion(opts.Version); err != nil {
			return result, model.Err("UPDATE_INVALID_VERSION", "invalid target version", err)
		}
	}
	exe, err := validateExecutable(opts.ExecutablePath)
	if err != nil {
		return result, err
	}
	result.ExecutablePath = exe
	lock, err := acquireLock(filepath.Join(filepath.Dir(exe), ".pitstop-update.lock"))
	if err != nil {
		return result, err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); _ = lock.Close() }()
	initialInfo, err := os.Stat(exe)
	if err != nil {
		return result, model.Err("UPDATE_UNSAFE_DESTINATION", "could not inspect running executable", err)
	}

	repo := ""
	if opts.RepoExplicit {
		if opts.Repo == "" {
			return result, model.Err("CONFIG_INVALID", "--repo requires a path", nil)
		}
		repo, err = config.ResolveRepo(opts.Repo)
		if err != nil {
			return result, err
		}
	} else if opts.Repo != "" && opts.Repo != "." {
		repo, _ = config.ResolveRepo(opts.Repo)
	} else {
		repo, _ = config.ResolveRepo(".")
	}
	targets, err := inventory(repo)
	if err != nil {
		return result, err
	}
	for _, target := range targets {
		result.Targets = append(result.Targets, target.UpdateTarget)
	}

	current := versionText(opts.CurrentVersion)
	currentVersion, currentErr := parseStableVersion(current)
	targetVersion := opts.Version
	if !opts.ExplicitVersion {
		if currentErr != nil {
			return result, model.Err("UPDATE_VERSION_UNKNOWN", "development builds require --version", currentErr)
		}
		targetVersion, err = resolveLatestRelease(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return result, model.Err("CANCELLED", "update cancelled", ctx.Err())
			}
			return result, model.Err("UPDATE_RESOLVE_FAILED", "could not resolve latest release", err)
		}
	}
	result.TargetVersion = targetVersion
	targetStable, _ := parseStableVersion(targetVersion)
	useCurrent := !opts.ExplicitVersion && currentVersion.compare(targetStable) >= 0
	updateExe := exe
	if !useCurrent {
		stageDir, stageCleanup, stageErr := stageRelease(ctx, exe, targetVersion, opts.Progress)
		if stageErr != nil {
			return result, stageErr
		}
		defer stageCleanup()
		updateExe = filepath.Join(stageDir, "pitstop")
		if err := verifyChildVersion(ctx, updateExe, targetVersion); err != nil {
			return result, err
		}
	}
	if err := preflightTargets(ctx, updateExe, targets, opts.Progress); err != nil {
		for i := range targets {
			result.Targets[i] = targets[i].UpdateTarget
		}
		return result, err
	}
	if !useCurrent {
		if err := recheckExecutable(exe); err != nil {
			return result, err
		}
		if ctx.Err() != nil {
			return result, model.Err("CANCELLED", "update cancelled", ctx.Err())
		}
		currentInfo, statErr := os.Stat(exe)
		if statErr != nil || !os.SameFile(initialInfo, currentInfo) || currentInfo.Size() != initialInfo.Size() || !currentInfo.ModTime().Equal(initialInfo.ModTime()) {
			return result, model.Err("UPDATE_UNSAFE_DESTINATION", "running executable changed during update", statErr)
		}
		if err := os.Rename(updateExe, exe); err != nil {
			return result, model.Err("UPDATE_PUBLISH_FAILED", "could not replace executable", err)
		}
		updateExe = exe
		result.CLIStatus = "updated"
	} else {
		result.CLIStatus = "up_to_date"
	}
	for i := range targets {
		if err := ctx.Err(); err != nil {
			result.Partial = result.CLIStatus == "updated"
			for _, target := range targets[:i] {
				result.Partial = result.Partial || target.Status == "updated"
			}
			return result, model.Err("CANCELLED", "update cancelled", err)
		}
		if err := installTarget(ctx, updateExe, &targets[i], opts.Progress); err != nil {
			if targets[i].Status != "updated" {
				targets[i].Status = "blocked"
				targets[i].Message = err.Error()
			}
			result.Targets[i] = targets[i].UpdateTarget
			result.Partial = result.CLIStatus == "updated" || !isChildStartError(err)
			for _, target := range targets[:i] {
				result.Partial = result.Partial || target.Status == "updated"
			}
			if !isChildStartError(err) && targets[i].Status != "updated" {
				targets[i].Message = "skill installation outcome unknown after child started: " + err.Error()
				result.Targets[i] = targets[i].UpdateTarget
			}
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "update cancelled") {
				result.Partial = true
			}
			return result, err
		}
		targets[i].Status = "updated"
		result.Targets[i] = targets[i].UpdateTarget
	}
	if ctx.Err() != nil {
		result.Partial = result.CLIStatus == "updated"
		for _, target := range targets {
			result.Partial = result.Partial || target.Status == "updated"
		}
		return result, model.Err("CANCELLED", "update cancelled", ctx.Err())
	}
	return result, nil
}

func validateExecutable(path string) (string, error) {
	if path == "" {
		return "", model.Err("UPDATE_UNSAFE_DESTINATION", "could not resolve running executable", nil)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", model.Err("UPDATE_UNSAFE_DESTINATION", "could not resolve running executable", err)
	}
	info, err := os.Lstat(abs)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", model.Err("UPDATE_UNSAFE_DESTINATION", "executable destination must be a regular file", err)
	}
	return abs, nil
}

func recheckExecutable(path string) error {
	_, err := validateExecutable(path)
	return err
}

func acquireLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, model.Err("UPDATE_BUSY", "another update is already running", err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, model.Err("UPDATE_BUSY", "update lock is not a regular file", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, model.Err("UPDATE_BUSY", "another update is already running", err)
	}
	return f, nil
}

func inventory(repo string) ([]updateTarget, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, model.Err("UPDATE_UNSAFE_DESTINATION", "could not resolve user home", err)
	}
	locations := []updateTarget{
		{UpdateTarget: model.UpdateTarget{Agent: "codex", Scope: "user", TargetPath: filepath.Join(home, ".agents", "skills", "pitstop-config")}},
		{UpdateTarget: model.UpdateTarget{Agent: "claude", Scope: "user", TargetPath: filepath.Join(home, ".claude", "skills", "pitstop-config")}},
	}
	if repo != "" {
		locations = append(locations,
			updateTarget{UpdateTarget: model.UpdateTarget{Agent: "codex", Scope: "project", TargetPath: filepath.Join(repo, ".agents", "skills", "pitstop-config")}, repo: repo},
			updateTarget{UpdateTarget: model.UpdateTarget{Agent: "claude", Scope: "project", TargetPath: filepath.Join(repo, ".claude", "skills", "pitstop-config")}, repo: repo},
		)
	}
	var out []updateTarget
	for _, target := range locations {
		info, err := os.Lstat(target.TargetPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, model.Err("UPDATE_UNSAFE_DESTINATION", "skill destination is not a regular directory: "+target.TargetPath, err)
		}
		target.Status = "not_attempted"
		out = append(out, target)
	}
	return out, nil
}

func resolveLatest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, officialRepo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("latest release returned HTTP %d", resp.StatusCode)
	}
	prefix := officialRepo + "/releases/tag/"
	if !strings.HasPrefix(resp.Request.URL.String(), prefix) {
		return "", errors.New("unexpected latest release URL")
	}
	tag := strings.TrimPrefix(resp.Request.URL.String(), prefix)
	if _, err := parseStableVersion(tag); err != nil {
		return "", err
	}
	return tag, nil
}

func stageRelease(ctx context.Context, exe, version string, progress io.Writer) (string, func(), error) {
	dir, err := os.MkdirTemp(filepath.Dir(exe), ".pitstop-update-")
	if err != nil {
		return "", func() {}, model.Err("UPDATE_STAGE_FAILED", "could not create update staging directory", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := runInstaller(ctx, version, dir, progress); err != nil {
		cleanup()
		if ctx.Err() != nil {
			return "", func() {}, model.Err("CANCELLED", "update cancelled", ctx.Err())
		}
		return "", func() {}, model.Err("UPDATE_DOWNLOAD_FAILED", "could not download release", err)
	}
	return dir, cleanup, nil
}

func runEmbeddedInstaller(ctx context.Context, version, dir string, progress io.Writer) error {
	script := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(script, pitstop.InstallerScript, 0700); err != nil {
		return err
	}
	cmd := exec.Command("sh", script, "--version", version, "--bin-dir", dir)
	cmd.Stdout = progress
	cmd.Stderr = progress
	return runProcessGroup(ctx, cmd)
}

func child(ctx context.Context, exe string, args []string, progress io.Writer) (childEnvelope, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(exe, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = progress
	err := runProcessGroup(ctx, cmd)
	var env childEnvelope
	decodeErr := json.Unmarshal(stdout.Bytes(), &env)
	if ctx.Err() != nil && err != nil {
		return env, ctx.Err()
	}
	if decodeErr != nil {
		if err != nil {
			return env, fmt.Errorf("child command failed: %w", err)
		}
		return env, fmt.Errorf("invalid child result: %w", decodeErr)
	}
	if err != nil {
		return env, fmt.Errorf("child command exited %d: %w", env.ExitCode, err)
	}
	return env, nil
}

func runProcessGroup(ctx context.Context, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return childStartError{err: err}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return ctx.Err()
	}
}

func isChildStartError(err error) bool {
	var startErr childStartError
	return errors.As(err, &startErr)
}

func verifyChildVersion(ctx context.Context, exe, want string) error {
	if info, statErr := os.Stat(exe); statErr != nil || !info.Mode().IsRegular() {
		return model.Err("UPDATE_VERIFY_FAILED", "staged executable is unavailable", statErr)
	}
	env, err := child(ctx, exe, []string{"version", "--json"}, io.Discard)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return model.Err("CANCELLED", "update cancelled", ctx.Err())
		}
		return model.Err("UPDATE_VERIFY_FAILED", "staged executable version check failed", err)
	}
	if env.SchemaVersion != model.SchemaVersion || env.Command != "version" || env.ExitCode != 0 || env.Error != nil {
		return model.Err("UPDATE_VERIFY_FAILED", "staged executable returned an invalid version result", nil)
	}
	got, _ := env.Result["version"].(string)
	if versionText(got) != want {
		return model.Err("UPDATE_VERIFY_FAILED", "staged executable version does not match target", nil)
	}
	return nil
}

func preflightTargets(ctx context.Context, exe string, targets []updateTarget, progress io.Writer) error {
	for i := range targets {
		target := &targets[i]
		args := []string{"skill", "install", "--agent", target.Agent, "--scope", target.Scope, "--force", "--dry-run", "--json"}
		if target.Scope == "project" {
			args = append(args, "--repo", target.repo)
		}
		env, err := child(ctx, exe, args, progress)
		if err != nil {
			target.Status = "blocked"
			target.Message = err.Error()
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return model.Err("CANCELLED", "update cancelled", ctx.Err())
			}
			return model.Err("UPDATE_PREFLIGHT_BLOCKED", "skill preflight failed for "+target.TargetPath, err)
		}
		if !validSkillResult(env, *target, true) {
			target.Status = "blocked"
			target.Message = "owned skill preflight was blocked"
			return model.Err("UPDATE_PREFLIGHT_BLOCKED", "skill preflight blocked for "+target.TargetPath, nil)
		}
	}
	return nil
}

func installTarget(ctx context.Context, exe string, target *updateTarget, progress io.Writer) error {
	args := []string{"skill", "install", "--agent", target.Agent, "--scope", target.Scope, "--force", "--json"}
	if target.Scope == "project" {
		args = append(args, "--repo", target.repo)
	}
	env, err := child(ctx, exe, args, progress)
	if err != nil {
		if ctx.Err() != nil && validSkillResult(env, *target, false) {
			target.Status = "updated"
		}
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return model.Err("CANCELLED", "update cancelled", ctx.Err())
		}
		return err
	}
	if !validSkillResult(env, *target, false) {
		return errors.New("skill installation did not complete")
	}
	return nil
}

func validSkillResult(env childEnvelope, target updateTarget, dryRun bool) bool {
	return env.SchemaVersion == model.SchemaVersion && env.Command == "skill install" && env.ExitCode == 0 && env.Error == nil && env.Result["agent"] == target.Agent && env.Result["scope"] == target.Scope && env.Result["targetPath"] == target.TargetPath && env.Result["dryRun"] == dryRun && env.Result["installed"] == !dryRun && env.Result["action"] == "replace"
}
