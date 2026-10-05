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
	cacheRoot, _ := os.UserCacheDir()
	cache := filepath.Join(cacheRoot, "pitstop", "cache")
	_ = os.WriteFile(filepath.Join(runDir, "engine-meta.json"), []byte(fmt.Sprintf(`{"dockerHost":%q,"label":%q}`, env.DockerHost, "com.pitstop.run="+filepath.Base(runDir))), 0600)
	_ = os.MkdirAll(filepath.Join(cache, "actions"), 0700)
	_ = os.MkdirAll(filepath.Join(cache, "dependencies"), 0700)
	socket := "-"
	if plan.ForceIntegration && job.ID == "integration" && strings.HasPrefix(env.DockerHost, "unix://") {
		socket = env.DockerHost
	}
	args := []string{"pull_request", "-e", filepath.Join(runDir, "event.json"), "-C", snapshot.Path, "-W", workflow, "-j", job.ID, "-P", plan.Config.Runner.Label + "=" + plan.Config.Runner.Image, "--container-architecture", plan.Config.Runner.Architecture, "--env-file", "/dev/null", "--secret-file", "/dev/null", "--var-file", "/dev/null", "--input-file", "/dev/null", "--container-daemon-socket", socket, "--container-options", "--label=com.pitstop.run=" + filepath.Base(runDir), "--rm", "--pull=false", "--action-offline-mode", "--action-cache-path", filepath.Join(cache, "actions"), "--cache-server-path", filepath.Join(cache, "dependencies"), "--json"}
	if b, e := json.Marshal(args); e == nil {
		_ = os.WriteFile(filepath.Join(runDir, "argv-"+job.ID+".json"), append(b, '\n'), 0600)
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
	done := make(chan struct{}, 2)
	found := make(chan bool, 2)
	go func() { found <- parseStream(stdout, stdoutFile, progress, job.ID); done <- struct{}{} }()
	go func() { found <- parseStream(stderr, stderrFile, nil, job.ID); done <- struct{}{} }()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	timer := time.NewTimer(timeout)
	timedOut, cancelled := false, false
	var waitErr error
	finished := false
	select {
	case waitErr = <-wait:
		timer.Stop()
		finished = true
	case <-timer.C:
		timedOut = true
		signalGroup(cmd, syscall.SIGINT)
		select {
		case waitErr = <-wait:
		case <-time.After(5 * time.Second):
			signalGroup(cmd, syscall.SIGKILL)
			waitErr = <-wait
		}
		finished = true
	case <-ctx.Done():
		cancelled = true
		signalGroup(cmd, syscall.SIGINT)
		select {
		case waitErr = <-wait:
		case <-time.After(5 * time.Second):
			signalGroup(cmd, syscall.SIGKILL)
			waitErr = <-wait
		}
		finished = true
	}
	if !cancelled && !finished {
		select {
		case waitErr = <-wait:
		case <-timer.C:
			timedOut = true
			signalGroup(cmd, syscall.SIGINT)
			select {
			case waitErr = <-wait:
			case <-time.After(5 * time.Second):
				signalGroup(cmd, syscall.SIGKILL)
				waitErr = <-wait
			}
		}
	}
	for i := 0; i < 2; i++ {
		<-done
	}
	close(found)
	code := 0
	if waitErr != nil {
		if x, ok := waitErr.(*exec.ExitError); ok {
			code = x.ExitCode()
		} else {
			code = -1
		}
	}
	observed := false
	for x := range found {
		observed = observed || x
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
	if code == 0 && !observed {
		result.Status = "skipped"
		result.Provenance = "unknown"
		result.ReasonCode = model.Reason("ENGINE_OUTCOME_UNKNOWN")
		result.Reason = model.Reason("act exited successfully without selected-job evidence")
		return result, nil
	}
	if code == 0 {
		result.Status = "passed"
	} else {
		result.Status = "failed"
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
	br := bufio.NewReaderSize(r, 64*1024)
	observed := false
	for {
		line, err := readLineBounded(br, log)
		if len(line) > 0 {
			if progress != nil {
				fmt.Fprintln(progress, string(line))
			}
			var rec struct {
				JobID      string `json:"jobID"`
				JobResult  string `json:"jobResult"`
				StepResult string `json:"stepResult"`
			}
			if json.Unmarshal(line, &rec) == nil && rec.JobID == job && rec.JobResult != "" {
				observed = true
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}
	return observed
}
func readLineBounded(r *bufio.Reader, w io.Writer) ([]byte, error) {
	var first []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(part) > 0 {
			_, _ = w.Write(part)
			if len(first) < 1024*1024 {
				n := 1024*1024 - len(first)
				if n > len(part) {
					n = len(part)
				}
				first = append(first, part[:n]...)
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return []byte(strings.TrimSpace(string(first))), err
	}
}
func Cleanup(ctx context.Context, runDir string) error {
	if meta, err := os.ReadFile(filepath.Join(runDir, "engine-meta.json")); err == nil {
		var m struct{ DockerHost, Label string }
		if json.Unmarshal(meta, &m) == nil && m.Label != "" {
			docker, e := exec.LookPath("docker")
			if e != nil {
				return e
			}
			q := exec.CommandContext(ctx, docker, "--host", m.DockerHost, "ps", "-aq", "--filter", "label="+m.Label)
			idsOut, e := q.Output()
			if e != nil {
				return e
			}
			for _, id := range strings.Fields(string(idsOut)) {
				if e := exec.CommandContext(ctx, docker, "--host", m.DockerHost, "rm", "--force", id).Run(); e != nil {
					return model.Err("CLEANUP_INCOMPLETE", "owned runner cleanup failed", e)
				}
			}
			return nil
		}
	}
	b, err := os.ReadFile(filepath.Join(runDir, "owned-resources.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var ids []string
	if err := json.Unmarshal(b, &ids); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	var failures []string
	for _, id := range ids {
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, " \t\n") {
			continue
		}
		if err := exec.CommandContext(ctx, docker, "rm", "--force", id).Run(); err != nil {
			failures = append(failures, id+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		_ = os.WriteFile(filepath.Join(runDir, "cleanup-errors.txt"), []byte(strings.Join(failures, "\n")), 0600)
		return model.Err("CLEANUP_INCOMPLETE", "one or more owned resources could not be removed", nil)
	}
	return nil
}
