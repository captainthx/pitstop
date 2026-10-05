# Pitstop MVP 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. If the user explicitly chooses delegation, use the available subagent workflow instead. Steps use checkboxes for tracking. No delegation is currently authorized.

**Goal:** Build a Go CLI that runs selected existing GitHub Actions jobs through act, tests the current source snapshot, and returns trustworthy human/JSON results.

**Architecture:** Inspect a strict repo config and workflow; build an isolated source snapshot; run eligible independent jobs sequentially through external act; persist evidence and reports. The CLI coordinates small config/workflow/snapshot/engine/report modules. Avoid embedding act or interpreting Actions expressions.

**Tech Stack:** Go 1.26.5, Cobra, go.yaml.in/yaml/v3, Go standard library; external Git, act 0.2.89 and Docker.

**Spec:** [pitstop-spec.md](pitstop-spec.md). **Context/evidence:** [pitstop-context.md](pitstop-context.md), [act-react-ci-report.md](act-react-ci-report.md).

## Global Constraints

- Executable is `pitstop`; initial platform is macOS ARM64; no product repo edits without authorization.
- Keep workflow execution steps in original YAML; no copied pnpm/go validation commands in product code.
- Require act 0.2.89 initially; no automatic installer, secret retrieval or arbitrary act argument passthrough.
- Default event pull_request, per-act timeout 20m, graceful stop allowance 5s.
- Config schemaVersion=1; supported architectures linux/arm64 and linux/amd64, no silent fallback.
- Exit codes 0/1/2/130; cancellation > executed failure > incomplete/preflight > success.
- No generic conditional/dependency/matrix/reusable-workflow execution in MVP; explicit integration adapter is the exception.
- Source execution uses independent snapshot and act local-copy checkout, never bind original repo.
- JSON stdout is one envelope; progress stderr; reports/logs retain raw evidence.
- Use `rtk` before shell commands in this user's environment. Never print credentials or token-bearing remote URLs.

## Review Focus

1. Working-tree changes differ from index or change mid-copy: snapshot must use current content or stop, not run stale input. Owned by Task 4.
2. Global/project actrc supplies secrets or extra jobs: isolation must neutralize it without editing the original. Owned by Task 5.
3. Engine exits 0 after skipping unsupported work: preflight must block no-op semantics and report exit 2. Owned by Tasks 3/6.
4. Cancellation leaves an act container running: stop owned resources and leave unrelated containers alone. Owned by Tasks 5/8.
5. Partial output, malformed JSONL or a huge log line: preserve logs, bound memory and emit valid final JSON. Owned by Tasks 5/6.

## Workspace and deliverable layout

Create a dedicated Git repo at a user-selected project path. Do not put product code inside recruit100-react/service. Copy these documents and evidence into `docs/design/` and `docs/evidence/` in that repo before execution. No implementation repo has been created yet.

```text
cmd/pitstop/main.go               process entry/exit only
internal/cli/root.go              Cobra commands, options, output modes
internal/model/types.go           shared config/plan/report contracts
internal/config/config.go         strict JSON and repo/path resolution
internal/workflow/inspect.go      YAML inventory/capability checks
internal/workflow/integration.go  guarded service adapter
internal/snapshot/snapshot.go     independent source capture/manifest
internal/actengine/engine.go      argv/env, subprocess execution/logs
internal/actengine/environment.go isolated HOME/config/Docker endpoint
internal/actengine/process_unix.go process groups/owned resource cleanup
internal/report/report.go         envelope/rendering/exit aggregation
internal/run/run.go               coordinator and per-repo lock
examples/recruit100-react.json
examples/recruit100-service.json
testdata/workflows/               supported and blocked YAML fixtures
testdata/engine/                  captured act JSONL/stderr fixtures
testdata/repos/                   generated Git repos in tests, not real secrets
scripts/smoke.sh                  explicit opt-in real engine checks
docs/design/                     portable spec/plan/context
docs/evidence/                   measured trial reports
README.md
go.mod / go.sum
```

Test files live beside owned Go files. Split a file only when it serves a separate responsibility; do not create a framework of interfaces for each function. No module path has been selected: choose the actual repository import path when creating it; use a temporary local module name if there is no hosted repo. Do not invent a GitHub owner.

## Shared data contracts

Define these in `internal/model/types.go`. Exact JSON property names are in the spec; use explicit tags, not Go default casing.

```go
type Options struct {
    Repo string
    Jobs []string
    JSON bool
    Timeout time.Duration
    Architecture string
    ForceIntegration bool
}
type Config struct {
    SchemaVersion int
    Workflow string
    Event string
    Jobs []string
    Runner RunnerConfig
    Adapter string
}
type RunnerConfig struct { Label, Image, Architecture string }
type Reason struct { Code, Message string }
type JobInfo struct {
    ID, Name, RunsOn, If string
    Needs []string
    SecretReferences []string
    CapabilityReasons []Reason
}
type Inventory struct { WorkflowPath string; Jobs []JobInfo }
type PlannedJob struct { ID string; Eligible bool; Reasons []Reason }
type Plan struct {
    Repo string
    Config Config
    Inventory Inventory
    Jobs []PlannedJob
    ForceIntegration bool
}
type SourceInfo struct {
    Head string
    Dirty bool
    SnapshotSHA256 string
    ExcludedPaths []string
    CheckoutMode string
}
type Snapshot struct { Path string; Source SourceInfo; ManifestPath string }
type EngineInfo struct {
    Name, Version, Architecture, Image string
    ImageDigest *string
}
type Adjustment struct { JobID, Operation, YAMLPath string; Before, After any }
type StepResult struct {
    ID, Name, Stage string
    Status *string
    DurationMS *int64
    EvidenceRef string
}
type JobResult struct {
    ID, Status, Provenance string
    DurationMS int64
    EngineExitCode *int
    FailureCategory, ReasonCode, Reason *string
    Steps []StepResult
    LogPath string
}
type RunReport struct {
    RunID, Repo string
    Source SourceInfo
    Engine EngineInfo
    Jobs []JobResult
    SelectedCoverage, WorkflowCoverage string
    Adjustments []Adjustment
    Warnings []string
    DurationMS int64
    ReportPath string
}
type ErrorInfo struct { Code, Message string }
type Envelope struct {
    SchemaVersion int
    Command string
    ExitCode int
    Result any
    Error *ErrorInfo
}
```

Empty collections serialize as `[]`, unavailable scalar values as null. RunReport does not duplicate envelope exitCode. UTC start/end timestamps and original/prepared workflow hashes are persisted in run metadata (`metadata.json`), referenced by reportPath's run directory. The final stdout envelope and persisted report.json must match.

Use `errors.Is` or a small typed error for stable reason codes; no string matching of arbitrary error messages for exit mapping. Include CONFIG_INVALID, TOOL_MISSING, ACT_VERSION_UNSUPPORTED, DOCKER_UNAVAILABLE, WORKFLOW_UNSUPPORTED, SECRET_REQUIRED, ADAPTER_SHAPE_CHANGED, SNAPSHOT_CHANGED, ENGINE_OUTCOME_UNKNOWN, TIMEOUT, CANCELLED, RUN_ALREADY_ACTIVE and CLEANUP_INCOMPLETE.

---

### Task 1: Close the engine feasibility gaps before building the wrapper

**Files:** Create `docs/evidence/engine-contract.md`, `testdata/engine/{success,failure,cancel}.jsonl`, `testdata/engine/*.stderr`, temporary fixture workflow under the implementation repo's ignored work directory.

**Interfaces:** Consumes existing raw React evidence; produces measured engine behavior, fixture logs and supported runtime settings for later tasks.

- [ ] Record act version, Docker platform/context endpoint and tested image identity without recording credentials.
- [ ] Run a small unconditional success/failure fixture using `act --json`; success exits 0, intentional `exit 7` makes act nonzero. Record actual log fields, stdout/stderr, selection evidence and cleanup behavior.
- [ ] Run an infinite/sleep fixture with a unique container label via `--container-options`; send interrupt and identify any leftover labeled runner resources. Confirm label behavior before relying on it.
- [ ] Repeat React verify with isolated HOME/config and explicit Docker endpoint to verify that neutralizing actrc does not break the proven workflow or cache connection.
- [ ] Trial service lint/test unchanged. Record environment failures separately from real code failures; do not fix unrelated product code during this task.
- [ ] Trial service forced integration in a temporary YAML copy. Validate Docker socket, container-to-PostgreSQL network, test results and cleanup.
- [ ] Trial paths-filter v3 without token using a local event payload containing base SHA/default branch. Record whether it includes dirty/new files. Native conditional support remains deferred even if this trial succeeds.
- [ ] Write a go/no-go table. React failure under isolation blocks the core design until resolved. Unsupported integration is allowed only with explicit blocking/reporting as specified.
- [ ] Commit evidence/fixtures using explicit filenames, never secrets or snapshot workspaces.

This task changes no product validation commands. Do not spend days reproducing GitHub internals: record a limitation and keep the supported subset when appropriate.

### Task 2: Establish the CLI, strict config and JSON envelope

**Files:** `go.mod`, `go.sum`, `cmd/pitstop/main.go`, `internal/cli/root.go`, `internal/model/types.go`, `internal/config/config.go`, `internal/report/report.go`, adjacent tests and examples.

**Interfaces:**

- `config.ResolveRepo(ctx context.Context, path string) (string, error)`
- `config.Load(repo string) (model.Config, error)`
- `report.WriteEnvelope(w io.Writer, envelope model.Envelope) error`
- `cli.Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int`

- [ ] Write failing tests `TestConfigRejectsUnknownFields`, `TestConfigRejectsDuplicateJobs`, `TestWorkflowPathCannotEscapeRepo`, `TestResolveNestedRepoWithSpaces`, `TestJSONErrorHasOneDocument`.
- [ ] Assert config schema 1, architectures exactly linux/arm64/linux/amd64, positive timeout default 20m, event pull_request, and invalid CLI/config exit 2.
- [ ] Run `rtk go test ./internal/config ./internal/cli ./internal/report`; confirm the intended red tests before implementing behavior.
- [ ] Implement types/config/path validation and Cobra commands. At this stage run may return a clear NOT_IMPLEMENTED error; never emit success for a stub.
- [ ] Implement envelope rendering; `version --json` returns version/build fields; missing config diagnostics do not leak usage to JSON stdout.
- [ ] Run the focused tests, `rtk go build ./cmd/pitstop`; verify version/help manually once.
- [ ] Commit `feat: add pitstop CLI and strict config contract` with only owned files.

### Task 3: Inventory workflows and build an honest execution plan

**Files:** `internal/workflow/inspect.go`, tests, `testdata/workflows/*`.

**Interfaces:**

- `workflow.Inspect(repo, workflowPath string) (model.Inventory, error)`
- `workflow.Plan(repo string, cfg model.Config, inv model.Inventory, opts model.Options) (model.Plan, error)`

- [ ] Write failing fixture tests `TestReactVerifyEligible`, `TestServiceConditionalIntegrationBlocked`, `TestNeedsNeverBroadensSelection`, `TestUnknownJobRejected`, `TestDependencyCycleReported`, `TestMatrixAndReusableWorkflowBlocked`, `TestConditionalNoOpNotPassed`, `TestLocalCompositeSecretReference`.
- [ ] Assert React verify eligible; service lint/test eligible; conditional integration blocked unless handled by the guarded adapter; missing/cyclic dependencies are errors. Keep excluded jobs in inventory.
- [ ] Run `rtk go test ./internal/workflow`; confirm failure for absent capabilities.
- [ ] Implement node-based parsing with string/list needs support, runner checks, secret reference traversal including local composites, and the MVP capability boundaries. Do not parse arbitrary Actions expressions.
- [ ] Keep source bytes unchanged for normal execution. Resolve local composite paths within the repo and prevent recursive composite cycles.
- [ ] Wire list to inventory/selection; no act process should start when calling list.
- [ ] Run focused tests plus CLI list fixture; JSON differentiates eligible/blocked/excluded.
- [ ] Commit `feat: inspect workflow capabilities and select local jobs`.

### Task 4: Capture the working tree without changing it

**Files:** `internal/snapshot/snapshot.go`, tests, manifest fixtures.

**Interfaces:**

- `snapshot.Capture(ctx context.Context, repo, runDir string) (model.Snapshot, error)`
- `snapshot.Remove(snapshot model.Snapshot) error`

- [ ] Generate disposable Git repos in tests; add failing `TestWorkingTreeOverridesStagedContent`, `TestDeletionPreserved`, `TestUntrackedSourceIncluded`, `TestIgnoredAndConventionalEnvExcluded`, `TestExternalSymlinkBlocked`, `TestSnapshotChangedDuringCapture`, `TestOriginalCheckoutUnchanged`.
- [ ] Verify contents and modes with deterministic SHA-256; verify originals with Git status plus content hashes. Include spaces/newlines in filenames through NUL-delimited commands.
- [ ] Run `rtk go test ./internal/snapshot`; observe red tests.
- [ ] Implement independent clone plus overlay, deleted-file reconciliation, exclusions, symlink rules, source-stability check and deterministic manifest.
- [ ] Reject merge conflicts and submodules instead of partially copying them. Never change original index or HEAD. Preserve usable origin internally without printing credentials.
- [ ] Run unit tests and one actual act fixture reading an uncommitted marker from the snapshot. Confirm that local checkout-copy mode exposes the marker.
- [ ] Commit `feat: execute current source through isolated snapshots`.

### Task 5: Implement isolated act execution, logs and cancellation

**Files:** `internal/actengine/{engine,environment,process_unix}.go`, tests and Task 1 log fixtures.

**Interfaces:**

- `actengine.Check(ctx context.Context) (model.EngineInfo, error)`
- `actengine.Run(ctx context.Context, snapshot model.Snapshot, plan model.Plan, job model.PlannedJob, runDir string, timeout time.Duration, progress io.Writer) (model.JobResult, error)`
- `actengine.Cleanup(ctx context.Context, runID string) error`

`Run` returns an error only if it cannot launch/collect an outcome; nonzero engine exit is a JobResult with failed status. Populate actual engine identity in run metadata. Tests use a helper subprocess or fake executable, not an elaborate injectable backend framework.

- [ ] Write failing tests `TestArgumentsAreNotShellInterpolated`, `TestActrcCannotInjectSecretOrJob`, `TestExplicitDockerEndpointSurvivesIsolatedHome`, `TestToolVersionMismatchBlocked`, `TestLargeMalformedLogPreserved`, `TestTimeoutStopsProcessGroup`, `TestCancelPreservesUnrelatedResource`.
- [ ] Use a hostile HOME/XDG/invocation actrc fixture adding `--secret` and a second platform; assert that none appears in child argv/config. Assert credentials absent from report/progress.
- [ ] Run `rtk go test ./internal/actengine`; confirm relevant failures.
- [ ] Implement engine argv, neutral launcher cwd, isolated home/config paths, explicit caches and resolved Docker endpoint. Use the exact version/features proved by Task 1.
- [ ] Add unique runner-container labels verified in Task 1. Track only owned resource IDs; default React socket disabled. No global Docker prune.
- [ ] Drain stdout/stderr concurrently to disk, keep bounded parser buffers and optional progress; do not deadlock when one stream fills. Use max parsed line 1 MiB; preserve larger lines as raw evidence without parsing.
- [ ] Implement timeout/process-group stop, 5s grace, forced kill and owned-resource cleanup. Surface cleanup warnings even when validation passes.
- [ ] Run unit tests and real cancellation smoke fixture; verify unrelated preexisting containers still running.
- [ ] Commit `feat: run act with isolated settings and bounded lifecycle`.

### Task 6: Coordinate runs and render trustworthy reports

**Files:** `internal/run/run.go`, `internal/report/report.go`, `internal/cli/root.go`, tests.

**Interfaces:**

- `run.Execute(ctx context.Context, opts model.Options, progress io.Writer) (model.Envelope, error)`
- `report.ExitCode(jobs []model.JobResult, cancelled bool) int`
- `report.Human(w io.Writer, envelope model.Envelope) error`

Execute resolves repo/config, obtains per-repo lock, creates runDir/snapshot, plans jobs, runs independent eligible jobs sequentially and always attempts cleanup. Command errors become typed envelopes at the CLI boundary. run/report.json contains the final stdout envelope, including cleanup warnings.

- [ ] Write failing `TestExitPrecedence`, `TestBlockedIntegrationDoesNotPreventLintAndTest`, `TestNoEligibleJobsReturnsTwo`, `TestNoOpCannotReturnSuccess`, `TestJSONStdoutOneDocumentDuringFailureAndCancel`, `TestSameRepoLockReturnsImmediately`, `TestUnknownStepRemainsNull`.
- [ ] Assert failure+blocked exits 1 with selectedCoverage partial; blocked-only exits 2; cancel exits 130; complete supported jobs exit 0. A task-generated fake act success string without a valid exit/selection cannot prove success.
- [ ] Run `rtk go test ./internal/run ./internal/report ./internal/cli`; confirm red.
- [ ] Implement per-repo lock, unique run IDs and artifact persistence. Human output includes selected scope and blocked jobs, not just a green overall line.
- [ ] Adapt JSONL structured routing fields for optional step detail; use engine exit for supported unconditional job outcome. Preserve unknown records. If Task 1 lacks reliable selection evidence, block rather than inventing an event.
- [ ] Add doctor and list reports with concrete command result types. Doctor checks act/Docker/config and never launches validation.
- [ ] Run focused tests, `rtk go test ./...`, and CLI fixtures. Verify every exit code and persisted/stdout report equality.
- [ ] Commit `feat: report job outcomes and partial validation coverage`.

### Task 7: Add the guarded integration adapter and example profiles

**Files:** `internal/workflow/integration.go`, tests, examples, service-shaped workflow fixture.

**Interfaces:**

- `workflow.PrepareIntegration(sourcePath, destinationPath string) ([]model.Adjustment, error)`

Only called when config adapter name matches, --force-integration is supplied, and integration is selected. Prepared YAML path becomes engine -W; source bytes/hash remain intact.

- [ ] Write failing `TestIntegrationOnlyNeedsAndIfRemoved`, `TestSourceWorkflowUnchanged`, `TestShapeDriftRejected`, `TestForceFlagWithoutAdapterRejected`, `TestForcedRunReportsPartialWorkflowFidelity`.
- [ ] Run `rtk go test ./internal/workflow`; confirm red.
- [ ] Implement node edits removing only expected integration needs/if. Require expected integration command/condition. Record adjustments and hashes; unknown shape returns ADAPTER_SHAPE_CHANGED.
- [ ] Wire runtime availability from Task 1. If integration is unsupported, --force-integration still returns blocked with a meaningful reason; do not imply the flag guarantees support.
- [ ] Create the exact React/service example JSON profiles. Service default makes unsupported integration visible in partial coverage; `--job lint --job test` explicitly narrows scope when desired.
- [ ] Run adapter tests and actual integration only if runtime was proven. Record engine failures verbatim.
- [ ] Commit `feat: add recruit100 profiles and explicit integration mode`.

### Task 8: Acceptance, packaging and handoff

**Files:** `scripts/smoke.sh`, README.md, docs/evidence/final-validation.md, build outputs outside source control.

**Interfaces:** Consumes the complete CLI/contracts. Produces reproducible binaries, user/agent instructions and verified support matrix.

- [ ] Run `rtk go test ./...`, `rtk go test -race ./...`, `rtk go vet ./...`; tests requiring real Docker are explicit opt-in and never deploy.
- [ ] Build macOS ARM64 binary, smoke `version`, `doctor`, `list` and JSON errors. Build Linux AMD64 artifact if desired; distinguish compiled from actually tested.
- [ ] Real React run in the original repo input via snapshot: exact workflow, no secrets; cold/warm evidence with cache state. Do not delete shared user caches just to obtain an artificial cold number.
- [ ] Real Go lint/test run unchanged; report any validation/setup defects. Product-code fixes need separate scope.
- [ ] Real dirty/new/deleted input fixture plus timeout/cancel smoke. Check original content/status unchanged and no owned resources left running. Verify unrelated PostgreSQL/MinIO remain running.
- [ ] Confirm default/service mode's incomplete integration is visible; forced mode's adjustments and workflow fidelity are visible.
- [ ] Document install/build, external runtime prerequisites, config placement, exact commands, JSON schema, exit codes, artifact location and manual cleanup. Include an agent instruction: inspect blocked/coverage, do not assume exit 0 checks all workflows.
- [ ] Record support matrix with measured and blocked capabilities; link logs. No blanket claim of GitHub equivalence or faster CI.
- [ ] Review the complete implementation against spec and test evidence. Fix material defects before declaring MVP ready.
- [ ] Commit `docs: document pitstop usage and verified support`.

## Agent usage example

```sh
rtk proxy pitstop doctor --repo /path/to/repo --json
rtk proxy pitstop list --repo /path/to/repo --json
rtk proxy pitstop run --repo /path/to/repo --json
```

Interpret exit 1 as a launched failure and read the log; exit 2 as blocked/config/preflight/incomplete and inspect reasons; 130 as cancelled. Inspect selectedCoverage/workflowCoverage and adjustments even on success. Do not automatically add credentials, edit CI conditions, or deploy to make a run green.

## Plan self-review and traceability

| Spec concern | Owning tasks / verification |
|---|---|
| Actual act behavior, Go unknowns | 1; recorded raw runtime trials |
| CLI/config/JSON conventions | 2, 6; envelope/exit tests |
| Supported subset and secret/dependency checks | 3; capability fixtures |
| Current source and original checkout | 4; dirty/deleted/new/symlink tests |
| actrc/env isolation and Docker endpoint | 5; hostile settings fixture |
| Logs, unknown detail and no-op outcomes | 1, 3, 5, 6; engine fixtures/selection tests |
| Timeout/cancellation/ownership | 5, 8; real interruption plus unrelated containers |
| Service forced integration | 1, 7; real trial and guarded YAML edits |
| Human/agent docs and measured speed | 8; final report and support matrix |

Self-review completed for interface names, type references, dependency order and review-focus ownership. Detailed signatures are proposals for this plan; if changed during execution, update callers/tests/docs together. Recheck source workflow hashes at execution time because repos may have advanced.

## Execution handoff

This request is for planning artifacts, not implementation. Recommended execution is **native, sequential**, because engine feasibility informs all subsequent tasks and the modules share a small contract. The user may instead authorize delegated execution explicitly. Before starting code, review the spec/plan, select the new Pitstop repo location and confirm the execution method. Use executing-plans for native execution; do not silently create a hosted repository or edit the two application repositories.
