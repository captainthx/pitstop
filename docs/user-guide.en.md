# Pitstop user guide

**English** | [ไทย](user-guide.md)

**First run: build → configure → check readiness → run → read results.**

Use Pitstop to run checks before committing or pushing, including lint, unit tests, and builds already defined in GitHub Actions. A **job** is an entry under `jobs:` in the workflow file.

[Home](../README.md) · [Requirements and scope](requirements.en.md)

## 1. Prepare your tools and build

Install Git, act **0.2.89**, and Docker, and start the Docker daemon. Check them with:

```sh
git --version
act --version
docker version
```

To build from source, use Go **1.26.5 or newer** and run these commands from the Pitstop directory:

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
"$PITSTOP_BIN" version
```

`PITSTOP_BIN` stores the executable's path so you can use it against other repositories. A prebuilt binary does not require Go to start Pitstop. Project jobs still need the tools their workflow specifies.

The binary delivered in this local workspace is `.pitstop-work/bin/pitstop`. To use it, set `PITSTOP_BIN="$PWD/.pitstop-work/bin/pitstop"` from the Pitstop directory. This ignored workspace binary is not included in a GitHub clone; build from source after cloning.

## 2. Configure the target project

The target must be a Git repository with at least one commit and an existing workflow. Set its **repository root**:

```sh
TARGET_REPO="/absolute/path/to/your/repo"
```

Create `.pitstop.json` at that root, or edit the existing file. This example selects a job named `verify`:

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

Adapt `workflow`, `jobs`, and `runner` to your project. `jobs` contains **job IDs** under `jobs:`, not display text from `name:`.

| Setting | Value |
|---|---|
| `schemaVersion` | `1` |
| `workflow` | A `.yml` or `.yaml` file inside the repository |
| `event` | `pull_request` |
| `jobs` | At least one job ID, with no duplicates |
| `runner.label` | Matches the job's `runs-on`, such as `ubuntu-latest` |
| `runner.image` | Docker image used to run jobs |
| `runner.architecture` | `linux/arm64` or `linux/amd64` |
| `adapter` | Optional; use `recruit100-service-integration-v1` only for the supported service profile |

Use valid JSON without comments or unknown fields. The workflow must declare the `pull_request` event.

Example profiles are available for [recruit100-react](../examples/recruit100-react.json) and [recruit100-service](../examples/recruit100-service.json). Their job IDs belong to those projects; check and adjust them when using another repository.

## 3. Check readiness

```sh
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
```

- `doctor` checks the repository, configuration, workflow, act version, and Docker connection. Unsupported selected work produces `ready=false` and exit code `2`.
- `list` shows the workflow's jobs, which ones are selected, and reasons for blocked work.

`doctor` is a preflight check. Dependency availability and actual lint/test results are established during execution. A service profile selecting integration may report not ready because integration is blocked by default; you can still select and run lint/test.

## 4. Run checks

Run every job selected in `.pitstop.json`:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO"
```

Run a subset of the configured selection, for example with the service profile:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --job lint --job test
```

Set a time limit per job:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --timeout 10m
```

The default is **20 minutes per job**, rather than for the whole run. Jobs execute in selection order. When one fails or is blocked, Pitstop continues with later independent jobs. Press **Ctrl+C** to cancel.

`--job` can only select IDs already in `.pitstop.json`. Edit the configuration before selecting a new job.

Override architecture for one run with, for example, `--arch linux/amd64`. The image and Docker environment must support it. ARM64 has actual runtime evidence; AMD64 has not been exercised.

## 5. Read results

| Job status | Meaning |
|---|---|
| `passed` | act completed successfully with evidence for the selected job |
| `failed` | The engine/job failed or timed out |
| `blocked` | Preflight found unsupported work |
| `skipped` | Execution cannot be confirmed, for example after cancellation, a launch error, or an uncertain outcome |

| Exit code | Next action |
|---|---|
| `0` | All selected jobs passed under supported semantics |
| `1` | Read the log for a failure or timeout |
| `2` | Inspect configuration errors, blocked jobs, or uncertain results |
| `130` | The run was cancelled |

A failed job together with a blocked job produces exit `1`. Cancellation produces `130`, even if a job already failed.

**Coverage describes completeness, not whether every check passed:**

- `selectedCoverage`: whether selected jobs were fully executed (`complete` / `partial`). A completed failure can still count as executed coverage.
- `workflowCoverage`: whether the original workflow was fully covered. Selecting a subset or forcing integration produces `partial`.

## 6. Use JSON with scripts or AI agents

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --json > result.json 2> progress.log
```

`result.json` contains one JSON document. Progress goes to `progress.log`. `doctor`, `list`, and `version` also accept `--json`.

Start with these fields:

| Field | What it tells you |
|---|---|
| `exitCode` | Overall command result |
| `error` | Command-level error, when present |
| `result.jobs` | Each job's result and reason |
| `result.selectedCoverage` / `result.workflowCoverage` | Completeness of validation |
| `result.adjustments` | Adapter changes to the original workflow |
| `result.warnings` | Additional limitations or run problems |
| `result.reportPath` | Location of the saved report |

An error before run creation may have no `result`; read `error` instead. `null` means unavailable information. `steps: []` does not prove that no tests ran; reliable step detail may be unavailable.

## 7. Run recruit100-service integration

Use a profile with `adapter: "recruit100-service-integration-v1"` and `integration` in its configured selection:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --job integration --force-integration
```

The adapter checks the expected job shape, then removes integration's `needs` and `if` from a **workflow copy** to run it directly. The original repository's workflow is preserved.

A local Unix Docker endpoint is required, such as the tested OrbStack environment. Integration receives the Docker socket so Testcontainers can create database containers.

Success still reports `workflowCoverage=partial` because the original conditions were bypassed. The changes appear in `adjustments`. Workflow drift causes `ADAPTER_SHAPE_CHANGED`. This adapter is specific to the supported service workflow.

## 8. Which source files are tested?

Pitstop creates an isolated repository copy from the working tree:

- Uncommitted changes are included; working-tree contents take precedence over staged contents.
- Deleted files stay deleted in the snapshot.
- New, nonignored files are included.
- Ignored files are excluded. Untracked `.env*`, `.actrc`, `.secrets`, `.vars`, and `.input` files are excluded too.
- Tracked versions remain source inputs, including tracked `.env` files.

You do not have to commit every edit before running Pitstop, but the repository needs an initial commit. If source changes during capture, Pitstop rejects the snapshot instead of accepting an inconsistent copy.

## 9. Where are reports and logs?

Find the actual path in `result.reportPath` or the final human summary. Runs live under the user cache directory:

```text
<user-cache>/pitstop/runs/<run-id>/
```

Each run retains `report.json`, job logs, `manifest.txt`, workflow copies, hashes, and actual argv. On macOS, this is normally under `~/Library/Caches/pitstop/runs/`.

The source snapshot and temporary HOME are removed when execution finishes. Reports and logs remain until manually removed. You may delete completed run directories whose evidence you no longer need.

## 10. Troubleshooting

| Problem | Check |
|---|---|
| `TOOL_MISSING` | Install the missing Git, act, or Docker tool and check PATH |
| `ACT_VERSION_UNSUPPORTED` | Use act 0.2.89 |
| `DOCKER_UNAVAILABLE` | Start Docker and check the active context/endpoint |
| `CONFIG_INVALID` | Check JSON, workflow path, job IDs, and runner settings |
| `WORKFLOW_UNSUPPORTED` | Read `list` and job reasons; select supported work |
| `SNAPSHOT_CHANGED` | Avoid source edits during capture; resolve pending merges and inspect submodules/symlinks |
| `RUN_ALREADY_ACTIVE` | Wait for or cancel the existing run for this repository |
| `TIMEOUT` | Read the log; increase `--timeout` when the job normally needs longer |
| `ENGINE_OUTCOME_UNKNOWN` | Inspect raw logs; uncertainty does not count as a pass |
| `INTEGRATION_RUNTIME_UNSUPPORTED` | Check the service adapter's local Unix Docker endpoint requirement |
| `ADAPTER_SHAPE_CHANGED` | The workflow no longer matches the supported integration shape |
| `REPORT_WRITE_FAILED` / `CLEANUP_INCOMPLETE` | Check cache space/permissions or Docker; completed results may remain in JSON |

See [requirements and scope](requirements.en.md) for the complete supported subset and limitations.
