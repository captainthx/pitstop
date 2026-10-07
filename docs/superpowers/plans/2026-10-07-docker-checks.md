# Docker Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ให้ Pitstop เปิด Docker environment แล้วรันคำสั่ง/shared script ของ repo บน snapshot โดยถอด act ออกจาก Pitstop ทั้งหมด และไม่รัน checks บน host

**Architecture:** เปลี่ยน backend เป็น Docker checks เพียงตัวเดียว ใช้ config v2, Docker CLI และ result files เป็น execution evidence นำ endpoint/cleanup logic เดิมมาใช้ แล้วลบ actengine/workflow parser ใช้ snapshot, repo lock, report models, exit precedence และ atomic persistence เดิม ไม่สร้าง engine selector/interface/factory

**Tech Stack:** Go 1.26.5, Cobra v1.10.2, Go stdlib, Git, Docker CLI/daemon; ไม่เพิ่ม dependency

**Spec:** [Docker Checks Design](../specs/2026-10-07-docker-checks-design.md) — อ่านก่อนเริ่มทุก task

## Global Constraints

- Final state: config v2 Docker checks เท่านั้น; v1 ต้องได้ actionable migration error; JSON envelope schemaVersion ยังคง 1
- Architecture เป็น `linux/arm64` หรือ `linux/amd64`; default timeout 20m ต่อ job; image prepare 2m; cleanup 10s
- ไม่รัน checks บน host ไม่ใช้ bind mount หรือ Docker socket และไม่ส่ง source ขึ้น GitHub
- fresh container ต่อ job; steps ใช้ `/bin/sh -e`, cwd `/workspace`, แชร์ files แต่ไม่แชร์ shell session
- ใช้ image ที่มีอยู่/pull เมื่อจำเป็น; ไม่มี auto-build, services, matrix, needs, uses, env/secret manager หรือ cache volumes รุ่นแรก
- jobs/step IDs: `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`; selection และ ID scopes unique
- exit codes 0/1/2/130 ใช้ precedence ใน report.ExitCode เดิม
- logs/progress บน stderr เมื่อ --json; stdout มี JSON envelope เดียว
- shell commands บนเครื่องต้องขึ้นต้น `rtk`; commands ภายใน repo config/container เป็นคำสั่งของผู้ใช้ ไม่ถูกเติม RTK
- ทุก task เพิ่ม failing tests ก่อน implement; รักษา tests สำหรับพฤติกรรมที่ยังใช้ และย้าย tests ของ old backend เป็น Docker equivalents ก่อนลบ; commit เฉพาะไฟล์ใน task บน `feature/docker-checks`

## Review Focus

1. Docker client exit กับ command exit คนละอย่าง: evidence ขาด/OOM/launch error ต้องไม่กลายเป็น passed — Tasks 3/4
2. Ctrl+C/timeout หยุด container จริง และ cleanup ไม่ลบ container อื่น — Tasks 2/3/5
3. `--repo` ที่มี spaces, dirty/new/deleted files, script writes: copy ตรง snapshot และ original ไม่เปลี่ยน — Tasks 3/5
4. Config version/unknown fields/unsafe IDs/overrides: reject ก่อนรัน command; v1 ได้ข้อความ migration โดยไม่เปิด container — Tasks 1/4
5. Logs/report เขียนไม่ได้หลังตรวจพบ failure: รักษา failure precedence และรายงาน storage error — Tasks 3/4

## Current Flow and Files

`cli.runCmd → run.Execute → config.Load → workflow.Inspect/Plan → actengine.Check → snapshot.CaptureContext → actengine.Run → cleanup → report/writeMetadata` ตอนนี้ผูกกับ act แม้ doctor/list ก็ assume workflow

| Task | ไฟล์ที่รับผิดชอบ |
|---|---|
| 1 | Modify `internal/model/types.go`, `internal/model/types_test.go`, `internal/config/config_test.go`; `config.go` เฉพาะเมื่อ strict decoder ต้องปรับ |
| 2 | Create `internal/dockerchecks/environment.go`, `internal/dockerchecks/environment_test.go`; นำ endpoint/owned cleanup logic เดิมมาใช้ |
| 3 | Create `internal/dockerchecks/engine.go`, `internal/dockerchecks/engine_test.go` |
| 4 | Modify `internal/model/types.go`, `internal/model/types_test.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/run/run.go`, `internal/run/run_test.go`, `internal/cli/root.go`, `internal/cli/root_test.go`, `internal/report/report.go`, `internal/report/report_test.go`; Delete `internal/actengine/`, `internal/workflow/`, `testdata/workflows/`; Modify `go.mod`, `go.sum` |
| 5 | Create `scripts/docker-smoke.sh`, `examples/docker-react.json`, `examples/docker-go.json`; Modify README และ user guides ทั้งสองภาษา, requirements ทั้งสองภาษา, `.github/workflows/ci.yml` |

ไม่แตะ snapshot algorithm; ลบ workflow adapter พร้อม engine เก่า ไม่ split/refactor ไฟล์อื่นที่ไม่เกี่ยวข้อง

Sequencing: Tasks 1–3 เตรียม v2 models/engine ใน branch โดยคง legacy fields/validation ชั่วคราวเพื่อให้ intermediate commits build/test ได้ Task 4 เปลี่ยน routing แล้วลบ legacy ทั้งหมดก่อน Task 5/ส่งงาน อย่า merge intermediate commits เป็นรุ่นพร้อมใช้

## Task 1: Versioned Docker Checks Config

**Interfaces:** Consumes `config.Load(repo) (model.Config,error)`; produces additions to `model.Config`, `model.EngineInfo`, `model.ListResult` and validation โดยไม่เปลี่ยน existing fields/types

- [ ] **1. เพิ่ม failing validation tests**

เพิ่มตัวอย่าง JSON จาก spec ใน config_test แล้ว assert Load สำเร็จ version2 โดยไม่มี workflow/event/label และยัง reject unknown field/trailing JSON แบบเดิม เพิ่ม test ใน model:

```go
func TestDockerConfigValidation(t *testing.T) {
    valid := func() Config {
        return Config{SchemaVersion: 2, Jobs: []string{"verify"},
            Runner: RunnerConfig{Image: "node:22-bookworm", Architecture: "linux/arm64"},
            Checks: []Check{{ID: "verify", Steps: []CheckStep{{ID: "check", Run: "npm run check"}}}}}
    }
    if err := valid().Validate(); err != nil { t.Fatal(err) }
    for _, mutate := range []func(*Config){
        func(c *Config) { c.Workflow = "ci.yml" },
        func(c *Config) { c.Event = "pull_request" },
        func(c *Config) { c.Adapter = "recruit100-service-integration-v1" },
        func(c *Config) { c.Runner.Label = "ubuntu-latest" },
        func(c *Config) { c.Runner.Image = "--privileged" },
        func(c *Config) { c.Runner.Image = "image\nother" },
        func(c *Config) { c.Runner.Architecture = "darwin/arm64" },
        func(c *Config) { c.Checks[0].ID = "../escape" },
        func(c *Config) { c.Checks[0].Steps[0].ID = "a/b" },
        func(c *Config) { c.Checks[0].Steps[0].Run = " " },
        func(c *Config) { c.Checks[0].Steps[0].Run = "echo\x00x" },
        func(c *Config) { c.Checks[0].Steps = nil },
        func(c *Config) { c.Checks = append(c.Checks, c.Checks[0]) },
        func(c *Config) { c.Checks[0].Steps = append(c.Checks[0].Steps, c.Checks[0].Steps[0]) },
        func(c *Config) { c.Jobs = []string{"missing"} },
        func(c *Config) { c.Jobs = []string{"verify", "verify"} },
    } {
        c := valid(); mutate(&c)
        if c.Validate() == nil { t.Fatalf("accepted %#v", c) }
    }
}
```

Run: `rtk go test ./internal/model ./internal/config -count=1` → build fails on missing fields/types until implemented; rerun after declarations to observe validation assertion failures before adding validation

- [ ] **2. เพิ่ม minimum model definitions**

```go
// Add Checks to Config; legacy fields survive only until Task 4.
Checks []Check `json:"checks,omitempty"`
// Task 4 removes Workflow/Event/Adapter/Runner.Label entirely.

type Check struct {
    ID string `json:"id"`
    Steps []CheckStep `json:"steps"`
}
type CheckStep struct {
    ID string `json:"id"`
    Name string `json:"name,omitempty"`
    Run string `json:"run"`
}
// EngineInfo addition:
ImageID string `json:"imageId,omitempty"`
```

Keep `model.SchemaVersion = 1` for envelopes. Temporarily retain existing v1 validation only to keep intermediate commits green; v2 applies every Config Contract rule from spec without an engine field; default rejects. Task 4 replaces this switch with v2-only validation and version-first migration errors. Use regexp compiled once for IDs and `strings` for NUL/whitespace checks. No renderer/parser/framework additions.

- [ ] **3. GREEN and commit**

Run `rtk go test ./internal/model ./internal/config -count=1`, then `rtk go test ./...` → all pass, including original v2-as-invalid test (its config lacks checks). Commit: `feat(config): define Docker checks config v2`.

## Task 2: Docker Environment and Owned Cleanup

**Interfaces:** Create concrete helpers inside the single engine package:

```go
package dockerchecks
// Environment defined here; Task 3 extends preparation using Info.
type Environment struct {
    DockerPath string
    DockerHost string
    Info model.EngineInfo
}
func Check(ctx context.Context) (Environment, error)
func command(ctx context.Context, env Environment, args ...string) *exec.Cmd
func Cleanup(ctx context.Context, runDir string) error
```

`Check` ports Docker resolution/version logic from actengine without checking act; `command` prepends --host and removes DOCKER_CONTEXT; `Cleanup` uses existing metadata format {dockerHost,label}, validates ownership and records removed IDs. This is a concrete engine package, not a generic Docker/backend framework. Leave old actengine source intact until Task 4 switches routing.

- [ ] **1. RED: endpoint and cleanup tests**

Use test-local fake Docker executables as existing tests do and record argv one argument per line. Cases: context overrides conflicting host; explicit host wins when no context; neither supplied resolves active context; empty/nonzero daemon version gives DOCKER_UNAVAILABLE; missing CLI gives TOOL_MISSING; no act installed still succeeds; invalid ownership errors before rm; only owned containers removed and anonymous volumes cleaned.

```go
func TestCommandPinsEndpoint(t *testing.T) {
    t.Setenv("DOCKER_CONTEXT", "other")
    env := Environment{DockerPath: "/fake/docker", DockerHost: "unix:///chosen.sock"}
    cmd := command(context.Background(), env, "version")
    if !reflect.DeepEqual(cmd.Args, []string{"/fake/docker", "--host", "unix:///chosen.sock", "version"}) {
        t.Fatal(cmd.Args)
    }
    for _, e := range cmd.Env {
        if strings.HasPrefix(e, "DOCKER_CONTEXT=") { t.Fatal(e) }
    }
}
```

Run `rtk go test ./internal/dockerchecks -count=1` → fail before helper implementation.

- [ ] **2. Port the existing Docker utilities into this package**

```go
func command(ctx context.Context, env Environment, args ...string) *exec.Cmd {
    argv := append([]string{"--host", env.DockerHost}, args...)
    cmd := exec.CommandContext(ctx, env.DockerPath, argv...)
    for _, e := range os.Environ() {
        if !strings.HasPrefix(e, "DOCKER_CONTEXT=") { cmd.Env = append(cmd.Env, e) }
    }
    return cmd
}
```

Check uses exec.LookPath("docker"), original endpoint resolution and `version --format {{.Server.Version}}`; Info.Name=docker, Info.Version=daemon. Cleanup uses label-filtered ps, validates each container's exact label via inspect before `rm --force --volumes`, and never uses prune. Already-removed containers are harmless; daemon/inspect failures are not cleanup success. Keep retained owned-resource evidence available across job cleanups, not just the last empty cleanup pass.

- [ ] **3. GREEN and commit**

Run `rtk go test ./internal/dockerchecks -count=1` and full suite. Commit: `feat(docker): add daemon checks and owned cleanup`.

## Task 3: Execute Checks in a Fresh Container

**Interfaces:** consumes Task 1 model and the same-package Environment/command/Cleanup from Task 2; produces:

```go
package dockerchecks
// Environment comes from environment.go, defined in Task 2.
func Prepare(ctx context.Context, env Environment, runner model.RunnerConfig, progress io.Writer) (Environment, error)
func Run(ctx context.Context, env Environment, snap model.Snapshot, check model.Check, runDir string, timeout time.Duration, progress io.Writer) (model.JobResult, error)
```

- [ ] **1. RED: generated script order and fail-fast**

Define unexported `writeScripts(dir string, check model.Check) error` returning generated files for `/pitstop`; test generated scripts in a temporary host directory only as a unit test (not production checks). Parameterize generator with its runtime directory internally if needed to make unit test run paths local without modifying user's run commands. Assert two steps share a file, failure skips third, command quotes/newlines are preserved, names printed as data. Container behavior is tested separately below.

Use this generated runner shape (one section per step):

```sh
#!/bin/sh
# Shell must continue after a failed inner step to save its exit evidence.
sh -e /pitstop/step-0.sh
code=$?
printf '%s\n' "$code" > /pitstop/results/step-0.exit || exit 125
if [ "$code" -ne 0 ]; then exit "$code"; fi
sh -e /pitstop/step-1.sh
code=$?
printf '%s\n' "$code" > /pitstop/results/step-1.exit || exit 125
if [ "$code" -ne 0 ]; then exit "$code"; fi
printf 'complete\n' > /pitstop/results/completed || exit 125
```

`step-N.sh` contains run text verbatim + final newline (0600). Write runner/results directories before copying. Do not run wrapper with `-e`: only inner step uses `-e`. Names/IDs must not be interpolated as executable shell syntax. Script prints progress headings with safely quoted printf arguments; status comes only from copied records and terminal Docker state.

- [ ] **2. RED: fake-Docker lifecycle and evidence tests**

Create test-local fake executable with `os.WriteFile` as existing engine tests do; it records argv, returns a known image inspect JSON, container ID, terminal state and copies configured step exit files when `cp CID:/pitstop/results/. DEST` is requested. A fake is not proof Docker works; Task 5 supplies that gate.

Add table-driven tests named `TestPrepareImage`, `TestRunEvidence`, `TestRunCancellationCleanup`, `TestRunLogFailure` with these actual fixtures:

```go
// Terminal-state fixtures for Docker inspect --format '{{json .State}}':
const passedState = `{"Running":false,"ExitCode":0,"Error":"","OOMKilled":false}`
const failedState = `{"Running":false,"ExitCode":7,"Error":"","OOMKilled":false}`
const oomState = `{"Running":false,"ExitCode":137,"Error":"","OOMKilled":true}`
// Result parser contract used directly by tests:
// readResult(check model.Check, state containerState, evidenceDir string) (model.JobResult, error)
// containerState is internal and has Running, OOMKilled bool; ExitCode int; Error string.
```

| Test input | Assert |
|---|---|
| matching local image ID/platform | no pull; create uses ID rather than tag |
| missing/wrong-platform image | one bounded pull → inspect; reject if still wrong |
| all exit records zero + completed + passedState | passed, all steps passed |
| step0=0, step1=7, failedState | failed, steps after step1 skipped |
| passedState but missing step1 record/completed | skipped ENGINE_OUTCOME_UNKNOWN |
| Docker start client exit=125 without evidence | unknown, never ordinary test failure/pass |
| command exit=125 with matching terminal/step record | legitimate failed check |
| OOM state or nonempty State.Error | failed CONTAINER_RUNTIME_FAILED, partial coverage |
| timeout/cancel while attach blocks | stop/rm happens under fresh context; TIMEOUT/CANCELLED |
| failing log writer | not passed; streams drain and process goroutines finish |
| repo snapshot path contains spaces | single cp source argument; no shell expansion |
| fake output prints success-like JSON | ignored as evidence |

Run: `rtk go test ./internal/dockerchecks -count=1` → fail before implementation.

- [ ] **3. Implement the exact lifecycle**

Prepare does inspect/pull/inspect within context.WithTimeout(2m), verifies image OS/architecture, records immutable ID and RepoDigest; missing image reason and daemon error must be distinguished (inspect JSON empty/nonzero needs actionable error, not infinite pull). Persist engine info once per run.

Construct create argv as individual strings:

```go
args := []string{"create", "--name", containerName,
    "--label", "com.pitstop.run=" + filepath.Base(runDir),
    "--platform", env.Info.Architecture, "--pull", "never", "--init",
    "--user", "0:0", "--workdir", "/workspace", "--no-healthcheck",
    "--entrypoint", "/bin/sh", env.Info.ImageID, "/pitstop/run.sh"}
```

Container name = `pitstop-<runID>-<check.ID>`; store command argv records under `runDir/argv-<job>-<operation>.json`. Before create write ownership metadata. No `--rm`: terminal inspect and result copy require retaining stopped container until evidence collection.

Copy source directory contents and generated directory:

```go
command(jobCtx, env, "cp", snap.Path+string(os.PathSeparator)+".", containerID+":/workspace")
command(jobCtx, env, "cp", scriptsDir, containerID+":/pitstop")
command(jobCtx, env, "start", "--attach", containerID)
command(evidenceCtx, env, "inspect", "--format", "{{json .State}}", containerID)
command(evidenceCtx, env, "cp", containerID+":/pitstop/results/.", evidenceDir)
```

Ensure `/workspace` is available by workdir/create contract and source cp behavior; real smoke must verify directory nesting explicitly. `scriptsDir` must be copied to nonexistent `/pitstop` so its contents become step scripts/run.sh/results directly, not an extra nested directory.

Attach stdout/stderr to separate 0600 log files plus progress; drain both to EOF even when log write fails, remembering the error. Use actengine.Run's drain-before-Wait pattern without its JSON parser. No bufio.Scanner token ceilings; raw bytes preserved. Set result LogPath to stdout log; keep stderr evidence too.

Apply spec status mapping after terminal inspection; read numeric status records strictly (0..255, one line), compare terminal code with first failed step, require completed marker for success. Populate StepResult with ID/name/status, null DurationMs and evidence reference. User script failure exits current job and skips remaining steps; caller schedules later jobs.

Cancellation/timeout use a job context covering create/copy/start. After context ends kill client and stop owned container under a fresh bounded context; inspect/copy evidence only after stopped. Before return, cleanup owner containers and volumes with fresh context (10s); if cleanup fails return result plus CLEANUP_INCOMPLETE error without erasing recorded status. This cleanup is repeated safely by run's final defer as a fallback.

- [ ] **4. GREEN and commit**

Run `rtk go test ./internal/dockerchecks -count=1`, full suite and vet. Commit: `feat(docker): execute checks with container evidence and cleanup`.

## Task 4: Replace act Routing and Delete Legacy Backend

**Interfaces:** consumes model.Config + dockerchecks.Check/Prepare/Run. Keep run.Execute/Options except removing ForceIntegration. The final Config struct has only SchemaVersion, Jobs, Runner, Checks; RunnerConfig has only Image/Architecture. There is no engine selector field or second backend.

- [ ] **1. RED: Docker entry points and legacy migration**

Extend run tests using current Git fixture/fake-tool pattern. Fake act must fail/record if invoked; Docker provides Task 3 lifecycle/evidence fixtures. Fixture repo contains no workflow. Verify:

```go
e, err := Execute(context.Background(), Options{Repo: repo, Jobs: []string{"verify"}, Timeout: time.Second})
if err != nil || e.ExitCode != 0 { t.Fatalf("%#v %v", e, err) }
r := e.Result.(model.RunReport)
if r.Engine.Name != "docker" || r.Engine.ImageID == "" ||
    r.Coverage.Selected != "complete" || r.Coverage.Workflow != "not_applicable" {
    t.Fatalf("%#v", r)
}
// Compare persisted report vs emitted envelope via reflect.DeepEqual,
// using existing TestExecuteRetainsHashesAndCoverageAfterSnapshotCleanup pattern.
```

Add explicit old-config rejection:

```go
func TestLegacyConfigNeedsMigration(t *testing.T) {
    dir := t.TempDir()
    old := `{"schemaVersion":1,"workflow":".github/workflows/ci.yml","event":"pull_request","jobs":["verify"],"runner":{"label":"ubuntu-latest","image":"old-image","architecture":"linux/arm64"}}`
    if err := os.WriteFile(filepath.Join(dir, ".pitstop.json"), []byte(old), 0600); err != nil { t.Fatal(err) }
    _, err := Load(dir) // Put this test in package config.
    var me *model.Error
    if !errors.As(err, &me) || me.Code != "CONFIG_INVALID" ||
        !strings.Contains(me.Message, "schemaVersion 2") {
        t.Fatalf("missing migration guidance: %v", err)
    }
}
```

Config.Load uses encoding/json RawMessage + a second Decode expecting io.EOF to validate exactly one JSON document, probes schemaVersion from RawMessage, then rejects version1 with message `schemaVersion 1 act workflows are no longer supported; migrate to schemaVersion 2 Docker checks`. Only supported version2 then gets strict typed decoding. Test version0/3, malformed JSON, trailing document, and v2 unknown workflow/event/adapter/runner.label/engine/uses fields. All reject before Docker invocation; no YAML parsing or automatic conversion.

Exercise invalid job/duplicate override, invalid --arch, captured config changes, cancelled preflight, failure followed by unknown job, report failure after known failure and cleanup failure after success. Preserve 1/2/130 precedence. Captured config comparison occurs before applying overrides to captured config.

CLI: list works with Docker/act absent, no config returns CONFIG_INVALID rather than reading ci.yml; doctor uses Docker and never act; run --json gives one envelope + EOF with progress on stderr; --force-integration returns unknown flag exit2 even with --json. Replace old help text assertions with Docker purpose/checks/examples and removal of adapter flag.

Run `rtk go test ./internal/config ./internal/run ./internal/cli ./internal/report -count=1` → new route/migration/help tests fail before implementation.

- [ ] **2. Rewrite run.Execute to the single Docker flow**

Reuse current fail/errorEnvelope, acquireLock, config.Load, snapshot.CaptureContext, writeAtomic and report.ExitCode. Remove workflow path/inspection/plan, prepared/original YAML copies, event.json, act Home/action cache and adapter logic. The flow is exactly spec.Execution:

1. Resolve repo, acquire lock, load config; retain normalized original bytes.
2. Apply valid selected job/arch overrides and Validate, reject nonpositive timeout.
3. dockerchecks.Check; create runDir and cleanup fallback defer before any owned container can exist.
4. Capture snapshot; reload config and compare original bytes; use captured steps as source of execution.
5. dockerchecks.Prepare once; run selected Check objects in cfg.Jobs order.
6. Preserve known results if later calls return errors; cancel marks later jobs skipped.
7. Cleanup under fresh context; remove snapshot/scripts; compute selectedCoverage and workflowCoverage=not_applicable.
8. Persist metadata and envelope atomically, report storage errors without erasing known failure.

Save config.json from captured config and effective-config.json after overrides. Add `writeDockerMetadata(dir, runID string, started time.Time, cfg model.Config, source model.Snapshot) error` to run.go; hash these configs and retain argv/container/image identity. Replace old writeMetadata rather than keeping unused workflow-only hashing. Retain tests asserting saved hashes and stdout report equality, adapted to config hashes.

Update doctor/list to check .pitstop.json checks directly. ListResult uses `ConfigPath string json:"configPath"` instead of WorkflowPath; Jobs/Selected/Eligible/Reasons remain. Human output prints `checks: .pitstop.json`; run output includes engine=docker. Retain report workflowCoverage=not_applicable and adjustments=[] for result compatibility without generating YAML adjustments. model.Adjustment may remain solely as the existing report wire type.

ก่อนลบ legacy fields ปรับ TestDockerConfigValidation ให้ลบ mutation cases ที่อ้าง c.Workflow/c.Event/c.Adapter/c.Runner.Label แล้วแทนด้วย strict-decoder unknown-field tests ที่เพิ่มใน task นี้ เพื่อไม่เหลือ compile references. Remove Config legacy fields, Runner.Label, model.JobInfo/Inventory/PlannedJob/Plan, Options.ForceIntegration, CLI --force-integration, config.ResolveWorkflow. Delete internal/actengine and internal/workflow (including tests), testdata/workflows. Move still-relevant failure/log/cleanup test cases into dockerchecks/run tests before deleting their former files. Snapshot/report/lock/JSON regressions remain, not discarded merely because their fixtures previously used act.

Run `rtk proxy go mod tidy` to remove go.yaml.in/yaml/v3 and unused transitive entries. Cobra remains. Existing Pitstop GitHub CI YAML is still used by GitHub itself; do not delete .github/workflows/ci.yml.

- [ ] **3. GREEN, no-act verification and commit**

```sh
rtk go test -race -count=1 ./...
rtk go vet ./...
rtk proxy go list -deps ./cmd/pitstop
rtk rg -n 'actengine|pitstop/internal/workflow|go.yaml.in/yaml|ForceIntegration|ResolveWorkflow' internal cmd go.mod go.sum
rtk git diff --check
```

Expected: tests/vet pass; dependency list has no old engine/workflow/YAML package; rg finds nothing (exit1 is expected). Legacy v1 JSON exists only in migration tests, not runtime paths. Commit: `feat!: replace act workflow execution with Docker checks`.

## Task 5: Real Docker Acceptance and User Docs

**Interfaces:** consumes `pitstop doctor/list/run --repo ... --json`; produces a small repeatable smoke script that CI and local users can run. It must exercise real Docker; no mocked success claims.

- [ ] **1. Add real smoke script with explicit gates**

Create `scripts/docker-smoke.sh` as `#!/bin/sh; set -eu`; accept PITSTOP_BIN pointing to a built binary, use mktemp for fixture repo/evidence and trap cleanup. Generate repo with initial commit, ignored file, tracked file modified after commit, deleted file and new untracked file. Put root source path under a directory containing spaces.

Use Alpine image with architecture derived from Docker daemon architecture; permit PITSTOP_SMOKE_IMAGE override for repeatability. Fixture checks:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "alpine:3.22", "architecture": "linux/arm64"},
  "jobs": ["verify", "independent"],
  "checks": [
    {"id":"verify","steps":[
      {"id":"source","run":"test \"$(cat tracked.txt)\" = dirty && test -f new.txt && test ! -e deleted.txt && test ! -e ignored.txt"},
      {"id":"write","run":"printf generated > generated.txt"},
      {"id":"reuse","run":"test \"$(cat generated.txt)\" = generated"}
    ]},
    {"id":"independent","steps":[
      {"id":"fresh","run":"test ! -e generated.txt"}
    ]}
  ]
}
```

Write fixture architecture from daemon, not hardcode ARM64 on CI. Use `git -c user.name=Test -c user.email=test@localhost commit` for fixture commit. Assertions use Python3 (available in CI/macOS) with actual JSON checks:

```python
import json, pathlib, sys
r = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert r['exitCode'] == 0, r
assert r['result']['engine']['name'] == 'docker'
assert r['result']['engine']['imageId'].startswith('sha256:')
assert r['result']['selectedCoverage'] == 'complete'
assert r['result']['workflowCoverage'] == 'not_applicable'
assert all(j['status'] == 'passed' for j in r['result']['jobs'])
assert all(s['status'] == 'passed' for j in r['result']['jobs'] for s in j['steps'])
```

Compare original repo tracked/new/deleted content and git status before/after; original generated.txt absent. Verify retained report/evidence exists and retained snapshot absent. Filter containers by exact returned run ID label and assert none remain; create one unrelated labeled fixture container and prove it survives until smoke script's own trap removes it.

Repeat with fail step `exit 7`, following step `touch must-not-run`, independent passing job; assert failure exit1, following step skipped, independent passed. Timeout scenario uses `sleep 30` with --timeout 1s; cancel scenario launches Pitstop then sends SIGINT after detecting its owned running container (poll with bounded 10s deadline, no fixed sleep assumption); assert exit130 and no owned containers. Synthetic unknown/log/persistence failures remain unit tests, not dangerous host filesystem changes in smoke.

Run script against pre-implementation binary first → fails because schema2 unsupported; after Tasks 1–4 completed → real Docker run must pass. If Docker daemon unavailable record that gate as unverified, not success.

- [ ] **2. Document config and reproducible environment**

Create examples/docker-react.json from spec. Create examples/docker-go.json with runner image `golang:1.26.5`, jobs verify, steps vet=`go vet ./...`, test=`go test -race ./...`, build=`go build ./...`. Images are example inputs, not claims that all registries/tools/platforms work without validation. Shared scripts remain in target repo; users may replace steps with `sh scripts/check.sh`. Remove active v1 examples/recruit100-react.json and examples/recruit100-service.json and update every README/guide link to new examples. Add a migration section mapping checkout/setup actions to snapshot/image and run steps to shared scripts. Keep docs/evidence and docs/design historical files intact, but link them as evidence of the former act implementation rather than current capabilities.

README/user-guide/requirements EN+TH explain Docker-only execution, v1 migration, prerequisites, setup once per repo, no uses translation, snapshot isolation, step/file persistence, image pinning, image preparation budget, timeout/cancel, reports and workflowCoverage not_applicable. Document no services/cache/auto-build yet and toolchain responsibility. Add a custom image example:

```dockerfile
FROM node:22-bookworm
# Add project-specific toolchain here only when required.
```

```sh
rtk proxy docker build -t my-checks:dev -f Dockerfile.checks .
rtk proxy go run ./cmd/pitstop doctor --repo /absolute/path/to/repo
rtk proxy go run ./cmd/pitstop list --repo /absolute/path/to/repo
rtk proxy go run ./cmd/pitstop run --repo /absolute/path/to/repo --json
```

Remove explanatory Dockerfile comment if example does not need additions; use public image directly as the first example. Build is a manual user action, not part of pitstop run. Do not promise CI parity or performance improvements without evidence.

- [ ] **3. Add Docker smoke to existing CI after build**

Keep formatting/dependency/vet/race/build/JSON steps intact. Add:

```yaml
- name: Check Docker checks on a real daemon
  run: PITSTOP_BIN="$RUNNER_TEMP/pitstop" sh scripts/docker-smoke.sh
```

CI ARM vs AMD platform from daemon must be recorded. Local acceptance records macOS ARM64/daemon Linux ARM64. Pulling Alpine in smoke is expected network access; published toolchain examples are verified separately when used on real projects.

- [ ] **4. Final validation and commit**

```sh
rtk proxy gofmt -w internal/model internal/config internal/dockerchecks internal/run internal/cli internal/report
rtk go test -race -count=1 ./...
rtk go vet ./...
rtk proxy go build -trimpath -o /tmp/pitstop-docker-checks ./cmd/pitstop
rtk proxy env PITSTOP_BIN=/tmp/pitstop-docker-checks sh scripts/docker-smoke.sh
rtk proxy go run ./cmd/pitstop -h
rtk proxy go run ./cmd/pitstop run -h
rtk git diff --check
```

Run version --json through scripts/validate-json.go with an RTK-prefixed shell invocation and confirm one document; existing JSON CI does this too. Re-run real source project shared check script at least once in image that matches its toolchain. Capture platform, image ID, exit/report paths and cleanup evidence; don't overwrite historical act validation evidence.

Commit: `test(docker): verify isolated checks and document config v2`.

## Completion / Review

- All acceptance criteria in spec must map to actual tests/evidence; review both normal and early-return cleanup paths.
- Review v1 migration errors, complete removal of act, false-success protection and ownership cleanup first, not cosmetic output.
- Run implementation on existing `feature/docker-checks`; no merge/push/release until user requests it.
- Recommended execution: sequential implementation because Tasks 1→2→3→4→5 share types and runtime contracts. User may choose luna_worker to implement task-by-task with parent review, as with help change.
- This turn creates design + implementation plan only; no product code changes or tests claiming the feature exists.
