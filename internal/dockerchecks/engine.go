package dockerchecks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"strconv"
	"strings"
	"time"
)

const imageTimeout = 2 * time.Minute

func Prepare(ctx context.Context, env Environment, runner model.RunnerConfig, progress io.Writer) (Environment, error) {
	prepareCtx, cancel := context.WithTimeout(ctx, imageTimeout)
	defer cancel()
	env.Info.Name, env.Info.Image, env.Info.Architecture = "docker", runner.Image, runner.Architecture
	wantArch := strings.TrimPrefix(runner.Architecture, "linux/")
	targetInfo, err := inspectImage(prepareCtx, env, runner.Image, runner.Architecture)
	if err != nil && !imageMissing(err) {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot inspect Docker image", err)
	}
	if err != nil || targetInfo.Os != "linux" || targetInfo.Architecture != wantArch {
		if progress != nil {
			fmt.Fprintf(progress, "preparing image %s\n", runner.Image)
		}
		if e := command(prepareCtx, env, "pull", "--platform", runner.Architecture, runner.Image).Run(); e != nil {
			return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot prepare Docker image", e)
		}
		targetInfo, err = inspectImage(prepareCtx, env, runner.Image, runner.Architecture)
	}
	if err != nil {
		return Environment{}, model.Err("DOCKER_UNAVAILABLE", "cannot inspect Docker image", err)
	}
	if targetInfo.Os != "linux" || targetInfo.Architecture != wantArch {
		return Environment{}, model.Err("CONFIG_INVALID", "Docker image does not match runner architecture", nil)
	}
	// A platform-qualified inspect may expose a platform config ID that the
	// daemon cannot accept as a create reference for a multi-platform tag.
	// Prefer its immutable repository digest, and fall back to the config ID
	// for images without a repository digest.
	env.Info.ImageID, env.Info.ImageDigest = targetInfo.ID, imageDigest(targetInfo)
	env.ImageReference = imageReference(targetInfo, runner.Image)
	return env, nil
}

func imageReference(info imageInspect, image string) string {
	for _, digest := range info.RepoDigests {
		if strings.Contains(digest, "@sha256:") {
			return digest
		}
	}
	if info.ID != "" {
		return info.ID
	}
	return image
}

func writeScripts(dir string, check model.Check) error {
	if err := os.MkdirAll(filepath.Join(dir, "results"), 0700); err != nil {
		return err
	}
	for i, step := range check.Steps {
		body := "#!/bin/sh\n" + step.Run
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("step-%d.sh", i)), []byte(body), 0600); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	for i, step := range check.Steps {
		fmt.Fprintf(&b, "printf '%%s\\n' %s\n", shellQuote("step: "+step.ID+" "+stepName(step)))
		fmt.Fprintf(&b, "sh -e /pitstop/step-%d.sh\ncode=$?\nprintf '%%s\\n' \"$code\" > /pitstop/results/step-%d.exit || exit 125\nif [ \"$code\" -ne 0 ]; then exit \"$code\"; fi\n", i, i)
	}
	b.WriteString("printf 'complete\\n' > /pitstop/results/completed || exit 125\n")
	return os.WriteFile(filepath.Join(dir, "run.sh"), []byte(b.String()), 0700)
}
func stepName(s model.CheckStep) string {
	if s.Name != "" {
		return s.Name
	}
	return s.ID
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

type containerState struct {
	Running   bool
	OOMKilled bool
	ExitCode  int
	Error     string
}

func Run(ctx context.Context, env Environment, snap model.Snapshot, check model.Check, runDir string, timeout time.Duration, progress io.Writer) (result model.JobResult, runErr error) {
	start := time.Now()
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result = model.JobResult{ID: check.ID, Provenance: "engine_exit", LogPath: filepath.Join(runDir, check.ID+".stdout")}
	scriptsDir := filepath.Join(runDir, "scripts-"+check.ID)
	evidenceDir := filepath.Join(runDir, "evidence-"+check.ID)
	result.Steps = stepResults(check, evidenceDir, -1)
	defer os.RemoveAll(scriptsDir)
	if err := writeScripts(scriptsDir, check); err != nil {
		return result, err
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		return result, err
	}
	stdoutFile, err := os.OpenFile(result.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return result, err
	}
	defer stdoutFile.Close()
	stderrPath := filepath.Join(runDir, check.ID+".stderr")
	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return result, err
	}
	defer stderrFile.Close()
	label := "com.pitstop.run=" + filepath.Base(runDir)
	name := "pitstop-" + filepath.Base(runDir) + "-" + check.ID
	meta, _ := json.Marshal(map[string]string{"dockerHost": env.DockerHost, "label": label})
	if err := os.WriteFile(filepath.Join(runDir, "engine-meta.json"), append(meta, '\n'), 0600); err != nil {
		return result, err
	}
	imageRef := env.Info.ImageID
	if env.ImageReference != "" {
		imageRef = env.ImageReference
	}
	createArgs := []string{"create", "--name", name, "--label", label, "--platform", env.Info.Architecture, "--pull", "never", "--init", "--user", "0:0", "--workdir", "/workspace", "--no-healthcheck", "--entrypoint", "/bin/sh", imageRef, "/pitstop/run.sh"}
	if err := saveArgv(runDir, check.ID+"-create", createArgs); err != nil {
		return result, err
	}
	createOut, err := command(jobCtx, env, createArgs...).CombinedOutput()
	if err != nil {
		if jobCtx.Err() != nil || ctx.Err() != nil {
			return cancelResult(result, check, cancellationError(jobCtx, ctx), start), nil
		}
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "container did not launch", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(createOut))))
	}
	id := strings.TrimSpace(string(createOut))
	if id == "" {
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "Docker returned no container ID", nil)
	}
	removed := false
	remove := func() {
		if removed {
			return
		}
		recordOwnedResource(runDir, id)
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := removeOwnedContainer(cleanupCtx, env, id, label); err != nil {
			if runErr == nil {
				runErr = model.Err("CLEANUP_INCOMPLETE", "owned container cleanup failed", err)
			} else {
				runErr = model.Err("CLEANUP_INCOMPLETE", runErr.Error()+"; owned container cleanup failed", err)
			}
		} else {
			removed = true
		}
	}
	defer remove()
	copyArgs := []string{"cp", snap.Path + string(os.PathSeparator) + ".", id + ":/workspace"}
	if err := saveArgv(runDir, check.ID+"-copy-source", copyArgs); err != nil {
		return result, err
	}
	if err := command(jobCtx, env, copyArgs...).Run(); err != nil {
		if jobCtx.Err() != nil || ctx.Err() != nil {
			return cancelResult(result, check, cancellationError(jobCtx, ctx), start), nil
		}
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "cannot copy source into container", err)
	}
	copyArgs = []string{"cp", scriptsDir + string(os.PathSeparator) + ".", id + ":/pitstop"}
	if err := saveArgv(runDir, check.ID+"-copy-scripts", copyArgs); err != nil {
		return result, err
	}
	if err := command(jobCtx, env, copyArgs...).Run(); err != nil {
		if jobCtx.Err() != nil || ctx.Err() != nil {
			return cancelResult(result, check, cancellationError(jobCtx, ctx), start), nil
		}
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "cannot copy check scripts into container", err)
	}
	startCmd := command(jobCtx, env, "start", "--attach", id)
	stdout, err := startCmd.StdoutPipe()
	if err != nil {
		return result, err
	}
	stderr, err := startCmd.StderrPipe()
	if err != nil {
		return result, err
	}
	if err := startCmd.Start(); err != nil {
		if jobCtx.Err() != nil || ctx.Err() != nil {
			return cancelResult(result, check, cancellationError(jobCtx, ctx), start), nil
		}
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "container start failed", err)
	}
	streams := make(chan error, 2)
	go func() { streams <- copyLog(stdout, stdoutFile, progress) }()
	go func() { streams <- copyLog(stderr, stderrFile, nil) }()
	wait := make(chan error, 1)
	go func() { wait <- startCmd.Wait() }()
	var clientErr error
	timedOut, cancelled := false, false
	select {
	case clientErr = <-wait:
	case <-jobCtx.Done():
		if ctx.Err() != nil {
			cancelled = true
		} else {
			timedOut = true
		}
		stopCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		_ = command(stopCtx, env, "stop", "--time", "5", id).Run()
		stop()
		select {
		case clientErr = <-wait:
		case <-time.After(10 * time.Second):
			forceCtx, forceCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := removeOwnedContainer(forceCtx, env, id, label); err != nil {
				runErr = model.Err("CLEANUP_INCOMPLETE", "owned container cleanup failed", err)
			} else {
				removed = true
			}
			forceCancel()
			select {
			case clientErr = <-wait:
			case <-time.After(5 * time.Second):
				clientErr = fmt.Errorf("Docker client did not exit after container removal")
			}
		}
	}
	streamErr := collectStreams(streams, stdout, stderr, 5*time.Second)
	if cancelled || timedOut {
		if timedOut {
			result.Status = "failed"
			result.ReasonCode = model.Reason("TIMEOUT")
			result.Reason = model.Reason("container check timed out")
			result.FailureCategory = model.Category("engine")
		} else {
			result.Status = "skipped"
			result.Provenance = "unknown"
			result.ReasonCode = model.Reason("CANCELLED")
		}
		return finishResult(result, start), nil
	}
	evidenceCtx, evidenceCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer evidenceCancel()
	state, inspectErr := inspectState(evidenceCtx, env, id)
	if inspectErr != nil {
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "cannot inspect container outcome", inspectErr)
	}
	if err := command(evidenceCtx, env, "cp", id+":/pitstop/results/.", evidenceDir).Run(); err != nil {
		if ctx.Err() != nil {
			return cancelResult(result, check, ctx.Err(), start), nil
		}
		return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "cannot copy step evidence", err)
	}
	if clientErr != nil && state.Error == "" && !state.OOMKilled {
		if code, ok := clientExitCode(clientErr); !ok || code != state.ExitCode {
			return result, model.Err("ENGINE_OUTCOME_UNKNOWN", "Docker start did not provide trustworthy terminal evidence", clientErr)
		}
	}
	result, err = readResult(check, state, evidenceDir, result)
	if err != nil {
		return result, err
	}
	if streamErr != nil {
		if result.Status == "passed" {
			result.Status, result.Provenance, result.ReasonCode, result.Reason = "skipped", "unknown", model.Reason("ENGINE_OUTCOME_UNKNOWN"), model.Reason("log capture failed")
		}
		return result, model.Err("LOG_CAPTURE_FAILED", "log capture failed", streamErr)
	}
	return finishResult(result, start), nil
}

func finishResult(r model.JobResult, started time.Time) model.JobResult {
	r.DurationMs = time.Since(started).Milliseconds()
	return r
}
func cancelResult(r model.JobResult, check model.Check, err error, started time.Time) model.JobResult {
	r.Steps = stepResults(check, "", -1)
	if err == context.DeadlineExceeded {
		r.Status, r.ReasonCode, r.Reason = "failed", model.Reason("TIMEOUT"), model.Reason("container check timed out")
		r.FailureCategory = model.Category("engine")
	} else {
		r.Status, r.Provenance, r.ReasonCode = "skipped", "unknown", model.Reason("CANCELLED")
	}
	return finishResult(r, started)
}
func cancellationError(jobCtx, parent context.Context) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	return jobCtx.Err()
}
func copyLog(r io.Reader, f io.Writer, progress io.Writer) error {
	b := make([]byte, 32*1024)
	var first error
	for {
		n, err := r.Read(b)
		if n > 0 {
			if progress != nil {
				if written, e := progress.Write(b[:n]); e != nil {
					if first == nil {
						first = e
					}
				} else if written != n && first == nil {
					first = io.ErrShortWrite
				}
			}
			written, e := f.Write(b[:n])
			if e != nil && first == nil {
				first = e
			} else if e == nil && written != n && first == nil {
				first = io.ErrShortWrite
			}
		}
		if err == io.EOF {
			return first
		}
		if err != nil {
			return err
		}
	}
}
func collectStreams(streams <-chan error, stdout, stderr io.ReadCloser, timeout time.Duration) error {
	var first error
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for remaining := 2; remaining > 0; remaining-- {
		select {
		case err := <-streams:
			if err != nil && first == nil {
				first = err
			}
		case <-timer.C:
			_ = stdout.Close()
			_ = stderr.Close()
			return firstOr(first, io.ErrNoProgress)
		}
	}
	return first
}
func firstOr(first, fallback error) error {
	if first != nil {
		return first
	}
	return fallback
}
func inspectState(ctx context.Context, env Environment, id string) (containerState, error) {
	out, err := command(ctx, env, "inspect", "--format", "{{json .State}}", id).Output()
	if err != nil {
		return containerState{}, err
	}
	var s containerState
	if err := json.Unmarshal(out, &s); err != nil {
		return s, err
	}
	return s, nil
}
func clientExitCode(err error) (int, bool) {
	if x, ok := err.(*exec.ExitError); ok {
		return x.ExitCode(), true
	}
	return -1, false
}
func readResult(check model.Check, state containerState, evidenceDir string, r model.JobResult) (model.JobResult, error) {
	r.EngineExitCode = &state.ExitCode
	if state.Error != "" || state.OOMKilled {
		r.Status, r.FailureCategory, r.ReasonCode, r.Reason = "failed", model.Category("engine"), model.Reason("CONTAINER_RUNTIME_FAILED"), model.Reason(state.Error)
		if state.OOMKilled {
			r.Reason = model.Reason("container was OOM-killed")
		}
		r.Steps = stepResults(check, evidenceDir, -1)
		return r, nil
	}
	firstFail := -1
	failureCode := -1
	complete := false
	for i := range check.Steps {
		b, err := os.ReadFile(filepath.Join(evidenceDir, fmt.Sprintf("step-%d.exit", i)))
		if err != nil {
			if firstFail >= 0 {
				break
			}
			r.Status, r.Provenance, r.ReasonCode, r.Reason = "skipped", "unknown", model.Reason("ENGINE_OUTCOME_UNKNOWN"), model.Reason("missing step evidence")
			r.Steps = stepResults(check, evidenceDir, firstFail)
			return r, nil
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) != 1 {
			return r, model.Err("ENGINE_OUTCOME_UNKNOWN", "invalid step evidence", nil)
		}
		code, err := strconv.Atoi(lines[0])
		if err != nil || code < 0 || code > 255 {
			return r, model.Err("ENGINE_OUTCOME_UNKNOWN", "invalid step evidence", err)
		}
		if code != 0 && firstFail < 0 {
			firstFail = i
			failureCode = code
		}
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "completed")); err == nil {
		complete = true
	}
	r.Steps = stepResults(check, evidenceDir, firstFail)
	if firstFail >= 0 {
		if state.Running || state.ExitCode != failureCode {
			r.Status, r.Provenance, r.ReasonCode, r.Reason = "skipped", "unknown", model.Reason("ENGINE_OUTCOME_UNKNOWN"), model.Reason("terminal state does not match step evidence")
			return r, nil
		}
		r.Status, r.FailureCategory, r.Reason = "failed", model.Category("unknown"), model.Reason(fmt.Sprintf("check step exited with code %d", state.ExitCode))
		return r, nil
	}
	if !complete || state.Running || state.ExitCode != 0 {
		r.Status, r.Provenance, r.ReasonCode, r.Reason = "skipped", "unknown", model.Reason("ENGINE_OUTCOME_UNKNOWN"), model.Reason("container outcome or complete evidence is missing")
		return r, nil
	}
	r.Status = "passed"
	return r, nil
}
func stepResults(check model.Check, dir string, firstFail int) []model.StepResult {
	out := make([]model.StepResult, 0, len(check.Steps))
	for i, s := range check.Steps {
		status := "skipped"
		if firstFail < 0 || i <= firstFail {
			if b, e := os.ReadFile(filepath.Join(dir, fmt.Sprintf("step-%d.exit", i))); e == nil {
				status = "passed"
				if strings.TrimSpace(string(b)) != "0" {
					status = "failed"
				}
			}
		}
		out = append(out, model.StepResult{ID: s.ID, Name: stepName(s), Status: model.Reason(status), EvidenceRef: filepath.Join(dir, fmt.Sprintf("step-%d.exit", i))})
	}
	return out
}
func saveArgv(dir, name string, args []string) error {
	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "argv-"+name+".json"), append(b, '\n'), 0600)
}
func removeOwnedContainer(ctx context.Context, env Environment, id, label string) error {
	out, err := command(ctx, env, "inspect", "--format", "{{index .Config.Labels \"com.pitstop.run\"}}", id).Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != strings.TrimPrefix(label, "com.pitstop.run=") {
		return model.Err("CLEANUP_INCOMPLETE", "container ownership changed during cleanup", nil)
	}
	return command(ctx, env, "rm", "--force", "--volumes", id).Run()
}
func recordOwnedResource(dir, id string) {
	path := filepath.Join(dir, "owned-resources.json")
	b, _ := os.ReadFile(path)
	var ids []string
	_ = json.Unmarshal(b, &ids)
	for _, x := range ids {
		if x == id {
			return
		}
	}
	ids = append(ids, id)
	if out, err := json.Marshal(ids); err == nil {
		_ = os.WriteFile(path, append(out, '\n'), 0600)
	}
}
