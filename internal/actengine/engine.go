package actengine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"syscall"
	"time"
)

func Run(ctx context.Context, env Environment, snapshot model.Snapshot, plan model.Plan, job model.PlannedJob, runDir string, timeout time.Duration, progress io.Writer) (model.JobResult, error) {
	start := time.Now()
	if env.Home == "" {
		env.Home = filepath.Join(runDir, "home")
		if err := os.MkdirAll(filepath.Join(env.Home, ".config"), 0700); err != nil {
			return model.JobResult{}, err
		}
	}
	stdoutPath := filepath.Join(runDir, job.ID+".stdout.jsonl")
	stderrPath := filepath.Join(runDir, job.ID+".stderr")
	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return model.JobResult{}, err
	}
	defer stdoutFile.Close()
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return model.JobResult{}, err
	}
	defer stderrFile.Close()
	workflow := plan.ExecutionWorkflowPath
	if workflow != "" && !filepath.IsAbs(workflow) {
		workflow = filepath.Join(snapshot.Path, workflow)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return model.JobResult{}, err
	}
	cache := filepath.Join(cacheRoot, "pitstop", "cache")
	meta, err := json.Marshal(map[string]string{"dockerHost": env.DockerHost, "label": "com.pitstop.run=" + filepath.Base(runDir)})
	if err != nil {
		return model.JobResult{}, err
	}
	if e := os.WriteFile(filepath.Join(runDir, "engine-meta.json"), meta, 0600); e != nil {
		return model.JobResult{}, e
	}
	if e := os.MkdirAll(filepath.Join(cache, "actions"), 0700); e != nil {
		return model.JobResult{}, e
	}
	if e := os.MkdirAll(filepath.Join(cache, "dependencies"), 0700); e != nil {
		return model.JobResult{}, e
	}
	socket := "-"
	if plan.Config.Adapter == "recruit100-service-integration-v1" && plan.ForceIntegration && job.ID == "integration" && strings.HasPrefix(env.DockerHost, "unix://") {
		socket = env.DockerHost
	}
	args := []string{"pull_request", "-e", filepath.Join(runDir, "event.json"), "-C", snapshot.Path, "-W", workflow, "-j", job.ID, "-P", plan.Config.Runner.Label + "=" + plan.Config.Runner.Image, "--container-architecture", plan.Config.Runner.Architecture, "--env-file", "/dev/null", "--secret-file", "/dev/null", "--var-file", "/dev/null", "--input-file", "/dev/null", "--container-daemon-socket", socket, "--container-options", "--label=com.pitstop.run=" + filepath.Base(runDir), "--rm", "--pull=false", "--action-offline-mode", "--action-cache-path", filepath.Join(cache, "actions"), "--cache-server-path", filepath.Join(cache, "dependencies"), "--json"}
	b, err := json.Marshal(args)
	if err != nil {
		return model.JobResult{}, err
	}
	if err := os.WriteFile(filepath.Join(runDir, "argv-"+job.ID+".json"), append(b, '\n'), 0600); err != nil {
		return model.JobResult{}, err
	}
	cmd := exec.Command(env.ActPath, args...)
	cmd.Dir = runDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + env.Home, "XDG_CONFIG_HOME=" + filepath.Join(env.Home, ".config"), "DOCKER_HOST=" + env.DockerHost, "LANG=C", "TMPDIR=" + os.TempDir()}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return model.JobResult{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return model.JobResult{}, err
	}
	if err := cmd.Start(); err != nil {
		return model.JobResult{}, model.Err("ENGINE_OUTCOME_UNKNOWN", "act did not launch", err)
	}
	type streamResult struct {
		observed bool
		err      error
	}
	streams := make(chan streamResult, 2)
	go func() {
		ok, e := parseStreamResult(stdout, stdoutFile, progress, job.ID)
		streams <- streamResult{ok, e}
	}()
	go func() { ok, e := parseStreamResult(stderr, stderrFile, nil, job.ID); streams <- streamResult{ok, e} }()
	wait := make(chan error, 1)
	var observed bool
	var logErr error
	go func() {
		// Drain before Wait: Wait closes StdoutPipe/StderrPipe.
		for i := 0; i < 2; i++ {
			r := <-streams
			observed = observed || r.observed
			if r.err != nil {
				logErr = r.err
			}
		}
		wait <- cmd.Wait()
	}()
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	timedOut, cancelled := false, false
	var waitErr error
	select {
	case waitErr = <-wait:
		timer.Stop()
	case <-timer.C:
		timedOut = true
		signalGroup(cmd, syscall.SIGINT)
		select {
		case waitErr = <-wait:
		case <-time.After(5 * time.Second):
			signalGroup(cmd, syscall.SIGKILL)
			waitErr = <-wait
		}
	case <-ctx.Done():
		cancelled = true
		signalGroup(cmd, syscall.SIGINT)
		select {
		case waitErr = <-wait:
		case <-time.After(5 * time.Second):
			signalGroup(cmd, syscall.SIGKILL)
			waitErr = <-wait
		}
	}
	code := 0
	if waitErr != nil {
		if x, ok := waitErr.(*exec.ExitError); ok {
			code = x.ExitCode()
		} else {
			code = -1
		}
	}
	result := model.JobResult{ID: job.ID, DurationMs: time.Since(start).Milliseconds(), EngineExitCode: &code, Steps: []model.StepResult{}, LogPath: stdoutPath, Provenance: "engine_exit"}
	if cancelled {
		result.Status = "skipped"
		result.Provenance = "unknown"
		result.ReasonCode = model.Reason("CANCELLED")
		return result, nil
	}
	if timedOut {
		result.Status = "failed"
		result.ReasonCode = model.Reason("TIMEOUT")
		result.Reason = model.Reason("act timed out")
		result.FailureCategory = model.Category("engine")
		return result, nil
	}
	if code == 0 && (!observed || logErr != nil) {
		result.Status = "skipped"
		result.Provenance = "unknown"
		result.ReasonCode = model.Reason("ENGINE_OUTCOME_UNKNOWN")
		result.Reason = model.Reason("act exited without trustworthy selected-job evidence or complete logs")
		return result, nil
	}
	if code == 0 {
		result.Status = "passed"
	} else {
		result.Status = "failed"
		result.FailureCategory = model.Category("unknown")
		result.Reason = model.Reason(fmt.Sprintf("act exited with code %d", code))
	}
	return result, nil
}
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, sig)
	}
}
func parseStream(r io.Reader, log io.Writer, progress io.Writer, job string) bool {
	observed, err := parseStreamResult(r, log, progress, job)
	return observed && err == nil
}
func parseStreamResult(r io.Reader, log io.Writer, progress io.Writer, job string) (bool, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	observed := false
	for {
		line, err := readLineBounded(br, log)
		if len(line) > 0 {
			if progress != nil {
				fmt.Fprintln(progress, string(line))
			}
			var rec struct {
				JobID     string `json:"jobID"`
				JobResult string `json:"jobResult"`
				Raw       bool   `json:"raw_output"`
			}
			if json.Unmarshal(line, &rec) == nil && rec.JobID == job && !rec.Raw && rec.JobResult == "success" {
				observed = true
			}
		}
		if err == io.EOF {
			return observed, nil
		}
		if err != nil {
			io.Copy(io.Discard, br)
			return observed, err
		}
	}
}
func readLineBounded(r *bufio.Reader, w io.Writer) ([]byte, error) {
	var line []byte
	oversized := false
	for {
		part, err := r.ReadSlice('\n')
		if len(part) > 0 {
			n, e := w.Write(part)
			if e != nil {
				return nil, e
			}
			if n != len(part) {
				return nil, io.ErrShortWrite
			}
			if !oversized {
				if len(line)+len(part) > 1024*1024 {
					oversized = true
					line = nil
				} else {
					line = append(line, part...)
				}
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return []byte(strings.TrimSpace(string(line))), err
	}
}
func Cleanup(ctx context.Context, runDir string) error {
	meta, err := os.ReadFile(filepath.Join(runDir, "engine-meta.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var m struct{ DockerHost, Label string }
	if err := json.Unmarshal(meta, &m); err != nil {
		return err
	}
	if m.DockerHost == "" || m.Label != "com.pitstop.run="+filepath.Base(runDir) {
		return model.Err("CLEANUP_INCOMPLETE", "invalid runner ownership metadata", nil)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	q := dockerCommand(ctx, docker, "--host", m.DockerHost, "ps", "-aq", "--filter", "label="+m.Label)
	idsOut, err := q.Output()
	if err != nil {
		return err
	}
	ids := strings.Fields(string(idsOut))
	if ids == nil {
		ids = []string{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "owned-resources.json"), append(b, '\n'), 0600); err != nil {
		return err
	}
	for _, id := range ids {
		if err := dockerCommand(ctx, docker, "--host", m.DockerHost, "rm", "--force", id).Run(); err != nil {
			return model.Err("CLEANUP_INCOMPLETE", "owned runner cleanup failed", err)
		}
	}
	return nil
}
