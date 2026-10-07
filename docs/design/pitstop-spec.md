> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../README.th.md) สำหรับการใช้งาน Docker checks.

# Pitstop MVP 1 — Product and Technical Specification

Date: 2026-10-05 (Asia/Bangkok). Revision 2, after the successful React experiment.

## 1. Purpose

Pitstop runs a repo's existing GitHub Actions validation jobs locally before opening a PR. The user wants fast feedback without maintaining another set of lint/test/build commands. Both a developer and an AI agent use the same CLI. The chosen name is **Pitstop**, executable `pitstop`, implemented in **Go**, delegating execution to **act + Docker**.

Success: `pitstop run` selects configured jobs, tests the current working-tree snapshot, reports what actually happened, and points to evidence. `--json` and stable exit codes let an agent fix code and rerun without interactive prompts. Local success does not replace required GitHub checks or prove deployment readiness.

## 2. Measured evidence

React `verify` ran twice from the original, unchanged `.github/workflows/ci.yml` in an independent local clone. Both exited 0.

| Attribute | Observed value |
|---|---|
| Source repo | `/Users/thanyatab/Desktop/project/techcombine/recruit100-react` |
| Commit | `c65e803404e1538c96d2162014eb7e03ee942881` |
| Engine | act 0.2.89, installed through Homebrew |
| Platform / image | linux/arm64 / catthehacker/ubuntu:act-latest |
| Image digest | sha256:c58e2b364da03b0c804c7d660f2ecbedf2f221a382b9baa0b344b0144780ff43 |
| Node / pnpm | 20.20.2 / 12.5.1 |
| First / cached run | 111.08 / 45.86 seconds |
| Tests | 2 files, 15 tests passed |
| Secret/env files passed | None |

Passed: ATS lint/typecheck/Vite build and Recruit100 lint/typecheck/tests/OpenNext Worker build. Original checkout remained clean. pnpm cache restored on run two; Next.js still had no build cache. Act used its default local-source copy handling for checkout.

Evidence files accompany this document: `act-react-ci-report.md`, both `.log` files and both `*-result.json` files.

Not proven: Go CI, integration/Testcontainers networking, dirty/untracked source capture, paths-filter local behavior, failure JSON format, cancellation cleanup, AMD64, or speed relative to GitHub. Do not advertise these as supported until tested.

## 3. Scope and stack

Initial target: macOS ARM64. Build Linux artifacts if practical, but platform support claims require real tests.

- Go toolchain baseline **1.26.5**; this is Pitstop's build choice, independent of target repo languages.
- **Cobra**, **go.yaml.in/yaml/v3**, standard library for JSON/files/processes/hashing/tests. Pin resolved dependency versions in go.mod/go.sum.
- External dependencies: **act 0.2.89**, Docker, Git. Do not embed act internal packages.
- No Docker SDK, Viper, custom Actions expression interpreter, database, daemon, generic plugin system or additional scheduler.

Included: doctor/list/run/version, selected jobs, config, snapshots, bounded execution, human/JSON reports, logs and two example repo profiles.

Excluded: GUI/TUI, MCP server, auto-fix, PR creation, GitHub check publication, secret retrieval, deploy/migrate/seed/backfill, host-native execution, general matrix/reusable workflows, Windows and build-cache tuning.

## 4. Repository context

### React

`ci.yml` is triggered by `pull_request`, has one unconditional `verify` job on `ubuntu-latest`:

1. checkout@v4; pnpm/action-setup@v4; setup-node@v4 with Node 20 and pnpm cache.
2. `pnpm install --frozen-lockfile`.
3. ATS lint, typecheck, build.
4. Recruit100 lint, typecheck, test, build:worker.

Root packageManager is pnpm@12.5.1. ATS is Vite; Recruit100 is Next.js/OpenNext Cloudflare. `build:worker` builds without deploying. Do not add ATS tests merely because the package has a test command: they are absent from this workflow. Deploy workflows use Firebase/Cloudflare credentials and are excluded.

### Service

Root: `/Users/thanyatab/Desktop/project/techcombine/recruit100-service`. Observed commit `e58f8403f675e49e3e9681ad9492d158622bb0c2`. go.mod declares Go 1.26.5. Pull-request `ci.yml` contains:

- **lint**: checkout, local composite setup-go, gofmt check, `go vet ./...`, golangci-lint-action@v8 with `version: latest`.
- **test**: checkout, composite setup-go, `go test -race ./...`.
- **changes**: dorny/paths-filter@v3 outputs persistence.
- **integration**: needs changes, conditional on persistence, `go test -race -tags integration ./...`.

Persistence paths: migrations, infrastructure, database/pgtest, go.mod/go.sum and `.github/**`. Composite setup-go uses setup-go@v5 plus cache@v4. Integration uses testcontainers-go/PostgreSQL in `internal/platform/pgtest`. Cloud deploy/migrate/seed/backfill uses GCP identity and external databases and is excluded.

## 5. CLI

```sh
pitstop doctor [--repo PATH] [--json]
pitstop list [--repo PATH] [--json]
pitstop run [--repo PATH] [--job ID ...] [--json]
            [--timeout 20m] [--arch linux/arm64|linux/amd64]
            [--force-integration]
pitstop version [--json]
```

Resolve PATH or cwd to Git top-level; nested app directories and spaces work. `--job` is repeatable and must remain within configured selection. No arbitrary act flag passthrough.

No prompts or automatic installations. Missing config allows list inventory, but blocks run with exit 2 and a suggested example. Doctor reports tools/config readiness without executing jobs. Help remains conventional text. CLI errors under `--json` produce one error document without usage banners on stdout.

Human mode streams progress and finishes with scope, result, duration and log paths. JSON mode sends progress to stderr and exactly one document to stdout. Default timeout: 20 minutes per act invocation, positive Go duration override. No automatic retries or architecture fallback.

## 6. Config

React `.pitstop.json`:

```json
{
  "schemaVersion": 1,
  "workflow": ".github/workflows/ci.yml",
  "event": "pull_request",
  "jobs": ["verify"],
  "runner": {
    "label": "ubuntu-latest",
    "image": "catthehacker/ubuntu:act-latest",
    "architecture": "linux/arm64"
  }
}
```

Service example selects `lint`, `test`, `integration` and adds `adapter: "recruit100-service-integration-v1"`. Declaring an adapter does not force integration; the flag opts in. Conditional integration remains visibly blocked until supported. The rest can still run.

Strict JSON: reject unknown fields/schema versions, empty/duplicate/unknown jobs, missing runner fields, unsupported architecture/event. MVP event is pull_request. Exactly one workflow per config. Workflow path is repo-relative .yml/.yaml; block traversal and resolved symlinks escaping repo. Config contains selection/settings only, never execution commands/secrets. Example profiles are not permission to edit product repos automatically.

## 7. Workflow capabilities and planning

Use YAML nodes to inspect jobs, static needs, conditions, runner, steps, env and local composite references. Execute ordinary workflow bytes unchanged. No generic expression interpreter.

MVP generic run supports unconditional, nonmatrix, independent jobs on the configured static runner. Block dependency-bearing jobs, dynamic runner/needs, job-level continue-on-error, matrices, reusable workflows, custom job containers/services and nontrivial job-level conditions. The explicit service adapter is the sole dependency/condition exception. This narrower subset makes per-job process exit a defensible outcome.

Inventory all jobs and dependencies, detect missing jobs/cycles, and report excluded jobs. `list` distinguishes selected, excluded and blocked. Do not infer that lack of `secrets.*` means a job is safe or runnable. Detect secret references in selected YAML and local composite actions; mark unsupported credential requirements blocked. Remote action internals remain delegated to act; static scanning is not proof of readiness.

Run eligible selected jobs sequentially, one act invocation per job. No dependency jobs are silently added. Continue independent jobs when another job fails/blocks, so the report contains useful results. Do not execute deploy-like jobs by default: selection is explicit and reviewed per profile.

## 8. Snapshot of the current source

The user's checkout is input, never execution workspace.

1. Record Git HEAD/status and NUL-delimited inventories of tracked and nonignored untracked files.
2. Make an independent local clone with --no-hardlinks. No linked worktree pointing into the original repo; no remote fetch required.
3. Overlay current working-tree contents, modes, symlinks, deletions and nonignored untracked source. Working tree wins over staged/index content. Never reset/add/commit in the original.
4. Reject unresolved merge state and submodules in MVP with a clear error.
5. Do not copy ignored local env/dependencies/build output. Exclude conventional untracked `.env`, `.env.*`, `.secrets`, `.vars`, `.input`, `.actrc`, even if not ignored; report their paths. Tracked files remain source input: Pitstop is not a general secret scanner.
6. Preserve symlinks within repo; block resolved targets escaping it. Validate source inventory/content before and after capture; if it changed mid-copy, stop and ask the caller to rerun.
7. Hash relative paths, modes, symlink targets and contents in a deterministic manifest. Record snapshot SHA-256; HEAD alone cannot identify dirty source.

Use act's default copy mode, not --bind or --no-skip-checkout. A real test must demonstrate that an uncommitted marker reaches the runner. Report checkoutMode=local-source-copy and the fact that it differs from GitHub's merge-ref checkout.

## 9. Engine and inherited settings

Use os/exec with argv, not shell interpolation. Initially require act 0.2.89; reject other versions with a tested-version message until adapter tests expand the range. Resolve image digest if possible, otherwise report null. Capture real engine/architecture identity, never assume.

Base invocation for React:

```text
act pull_request -C <snapshot> -W <absolute-workflow> -j verify
-P ubuntu-latest=catthehacker/ubuntu:act-latest
--container-architecture linux/arm64
--env-file /dev/null --secret-file /dev/null
--var-file /dev/null --input-file /dev/null
--container-daemon-socket - --rm
--action-offline-mode --pull=false --json
--action-cache-path <pitstop-cache>/actions
--cache-server-path <pitstop-cache>/dependencies
```

Offline mode only reuses assets; setup/package downloads can still access the network. Let act choose a dynamic cache-server port.

Act reads XDG/home/invocation actrc files before CLI args; array flags accumulate. `/dev/null` alone is insufficient. Run from a neutral directory with an isolated subprocess HOME/XDG config (including macOS config location), and stable explicit cache paths. Test hostile actrc injection; never edit the real actrc.

Resolve the user's Docker endpoint before isolating HOME. Pass it explicitly as DOCKER_HOST and remove inherited DOCKER_CONTEXT so it cannot override the resolved endpoint. Pass the minimum necessary PATH/locale/TMPDIR/Docker connection settings; preserve DOCKER_CONFIG only if needed for registry access, and never log credentials. Do not inherit cloud/token env variables or obtain gh auth tokens automatically. React disables Docker socket mounting; service integration may enable only the resolved local socket after its trial.

## 10. Integration adapter

`recruit100-service-integration-v1` checks that integration needs changes, uses the expected persistence if, and contains `go test -race -tags integration ./...`. A shape change fails closed with ADAPTER_SHAPE_CHANGED.

With --force-integration, create a YAML artifact and remove only integration's job-level needs/if. Keep all steps unchanged and select integration. Do not run changes or edit the original workflow. Report original/prepared hashes and precise adjustments. An adjusted run proves integration steps, not fidelity of the PR filtering logic.

Source inspection of paths-filter@v3 confirms Git-diff fallback without a token. Trial it separately before adding native conditional support; assess base SHA vs merge base and dirty/untracked behavior. Do not invent a separate glob evaluator in MVP.

If Testcontainers networking fails, report INTEGRATION_RUNTIME_UNSUPPORTED and ship the proven subset with integration blocked. Do not substitute host-native go test. The runtime trial and adapter tests are separate acceptance checks.

## 11. Report and exit contract

Selected-job states: passed/failed/skipped/blocked. Every state has provenance (engine_exit/preflight/adapter/unknown). Failure category: validation/environment/engine/unknown, with unknown when evidence is insufficient.

Coverage fields:

- selectedCoverage: complete/partial for selected jobs.
- workflowCoverage: complete/partial for original jobs/semantics. Forced integration retains partial workflow fidelity and adjustments even if selected steps finish.

Exit precedence: cancellation > executed failure > partial/preflight > success.

| Exit | Meaning |
|---|---|
| 0 | Selected jobs executed under supported semantics without failure or uncertainty |
| 1 | Launched engine/job failed; timeout is engine failure with reason TIMEOUT |
| 2 | Invalid flags/config/preflight, blocked job or uncertain/no-op execution |
| 130 | User cancelled |

One versioned JSON envelope contains schemaVersion=1, command, exitCode, error when applicable, and the command result. Run result includes runId, repo, source, engine, jobs, coverage, adjustments, warnings, durationMs and reportPath. A selected blocked job is not passed; an unsupported skip is partial coverage.

Run result fields:

```text
source: head, dirty, snapshotSha256, excludedPaths[], checkoutMode
engine: name, version, architecture, image, imageDigest|null
jobs[]: id, status, provenance, durationMs, engineExitCode|null,
        failureCategory|null, reasonCode|null, reason|null,
        steps[], logPath
steps[]: id, name, stage, status|null, durationMs|null, evidenceRef
adjustments[]: jobId, operation, yamlPath, before, after
```

Also persist UTC timestamps, workflow hashes and argv without credentials. Timing uses a monotonic clock. Null means unavailable, not success/zero. Schema example and Go data types live in the implementation plan. doctor/list/version use the same envelope, with their own concrete result structures.

## 12. Logs and outcome evidence

`act --json` produces logs, not a documented typed completion API. Job/step/stage fields help routing; lifecycle text may be needed for optional diagnostics. Preserve raw stdout JSONL and stderr.

For a preflight-approved independent unconditional job, engine exit 0 establishes job success and nonzero failure. Preflight blocks conditions that could make selected work a no-op. Do not establish success from English message matching alone. Step detail can remain unknown; do not fabricate pass counts/timing. Preserve malformed/unknown log records and support lines longer than Scanner's default limit without unbounded memory.

Capture success and intentionally failed engine fixtures during implementation. If evidence cannot confirm intended selection, report ENGINE_OUTCOME_UNKNOWN with exit 2. Lifecycle parser is version-specific and supplementary to process results.

## 13. Ownership, timeout, cancellation and storage

Root: os.UserCacheDir()/pitstop. Persistent actions/dependency caches; unique run directories with 0700 permissions, log/report files 0600. Retain manifests, workflow copies, event payload, report and logs; remove source snapshot/ephemeral HOME after execution. No historical-report auto-deletion in MVP; document manual cleanup.

Use a per-repo advisory lock; competing same-repo run exits 2 with RUN_ALREADY_ACTIVE. No long wait. Distinct repos have unique workspaces and engine resources.

On SIGINT/SIGTERM or timeout, signal the process group gracefully, wait 5 seconds, then kill it. Removing act alone may leave Docker resources: identify resources owned by the unique snapshot/run, remove only those exact IDs, and log cleanup failures. Never Docker-prune or stop unrelated PostgreSQL/MinIO. If integration leaves unidentifiable Testcontainers resources, report residual evidence instead of deleting by broad name matches. Real cancellation cleanup is an acceptance gate.

## 14. Acceptance criteria

1. One usable pitstop binary; predictable help/JSON/exit codes, no prompts.
2. React verify runs unchanged and passes on the user's machine, with persisted evidence.
3. Go lint/test attempted unchanged; distinguish actual validation defects from setup failures with logs.
4. Integration proven with actual Testcontainers/PostgreSQL or explicitly blocked across CLI/docs/report.
5. Deliberately failing fixture returns 1; unsupported/no-op/missing tools return 2; cancellation 130.
6. Dirty source, staged-vs-working changes, deletion, untracked source and spaced paths prove snapshot behavior without changing originals.
7. Inherited actrc/secret files cannot alter selection or inject secrets.
8. One JSON document in success/preflight/error/cancel; long logs do not deadlock/crash.
9. Cancellation removes owned runner resources and preserves unrelated containers.
10. Document measured cold/warm times without inventing a GitHub comparison.

## 15. References and design deltas

Primary references: [act usage](https://nektosact.com/usage/index.html), [runner images](https://nektosact.com/usage/runners.html), [limitations](https://nektosact.com/not_supported.html), [act 0.2.89 CLI/config](https://github.com/nektos/act/blob/v0.2.89/cmd/root.go), [logger](https://github.com/nektos/act/blob/v0.2.89/pkg/runner/logger.go), [paths-filter v3](https://github.com/dorny/paths-filter/blob/v3/src/main.ts), [Testcontainers inside Docker](https://golang.testcontainers.org/system_requirements/ci/dind_patterns/), [Cobra](https://github.com/spf13/cobra), [YAML v3](https://pkg.go.dev/go.yaml.in/yaml/v3).

Compared with the original draft: React ARM64 support is measured; Go selected; generic dependency/conditional execution is deferred; act JSON is treated as logs; snapshot identity includes dirty input; actrc isolation added; selected coverage separated from original workflow fidelity. This document supersedes `pitstop-mvp-spec.md`.
