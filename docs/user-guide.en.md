# Pitstop user guide

[ไทย](user-guide.md) | [Quick start](../README.md) | [Documentation index](README.md)

This guide describes Docker checks configured with `.pitstop.json` schema version 2.

Status 2026-10-09: the current checkout provides `init`, `validate`, and `skill install` alongside `doctor`, `list`, `run`, and `version`. The [AI-agent guide](ai-agent.en.md) covers the portable skill. The [config-agent trial evidence](evidence/config-agent-trials/README.md) records mapping and prerequisite behavior from 23 evaluator attempts; it does not establish runtime or CI parity. See the [roadmap](roadmap.md) for deferred scope and the limits of the current evidence.

The checkout also includes a [release installer](install.md) for macOS/Linux arm64/amd64. Publication is pending; source builds work now. Prebuilt installs need curl and a SHA-256 utility, not Go.

## Prerequisites

| Component | Requirement |
| --- | --- |
| Host running Pitstop | Git, Docker CLI and a reachable daemon |
| Building the CLI | Go 1.26.5, as specified in `go.mod` |
| Target repository | A Git repository; `run` requires at least one commit and `.pitstop.json` at its root, while `init` and `validate` can work before the first commit |
| Docker image | A Linux image containing `/bin/sh` and every tool used by its steps |
| Connectivity | Registry access for missing images; package registry access when a step installs dependencies |

Real execution has been validated on macOS ARM64 with a Linux ARM64 daemon. GitHub CI includes Docker smoke. The code uses Unix file locking; native Windows support is not verified. See [current validation](evidence/docker-checks-validation.md).

The Docker CLI must support `docker image inspect --platform`; check `docker image inspect --help` if using an older installation.

Install the CLI using the [README](../README.md#1-install-the-cli), then check:

```sh
git --version
docker version
pitstop version
```

Use Linux containers. Check the architecture of the **daemon you will use**, not just the host CPU:

```sh
docker version --format '{{.Server.Arch}}'
```

`arm64`/`aarch64` → `linux/arm64`; `amd64`/`x86_64` → `linux/amd64`. Cross-architecture execution depends on the Docker environment's emulation and is not guaranteed by the local validation.

## First repository setup

`pitstop init` creates a named starter without automatic manifest detection. Preview it before writing:

```sh
pitstop init --repo /absolute/path/to/your/repo --preset base --dry-run --json
```

Available presets are listed by `pitstop init -h`; `custom` requires `--image` and at least one repeated `--command`. Existing `.pitstop.json` files are preserved because init never overwrites them. For a manual path, copy the [Node example](../examples/docker-react.json) or [Go example](../examples/docker-go.json), then adapt it.

This complete configuration calls a project-owned script:

```json
{
  "schemaVersion": 2,
  "runner": {
    "image": "node:22-bookworm",
    "architecture": "linux/arm64"
  },
  "jobs": ["verify"],
  "checks": [
    {
      "id": "verify",
      "steps": [
        {"id": "install", "name": "Install dependencies", "run": "npm ci"},
        {"id": "check", "name": "Check project", "run": "sh scripts/check.sh"}
      ]
    }
  ]
}
```

Reuse an existing check script, or create `scripts/check.sh` in the **target repository**:

```sh
#!/bin/sh
set -eu
npm run lint
npm test
npm run build
```

These commands must exist in the project's `package.json`; `npm ci` also requires a lockfile. Adapt them to your package manager/toolchain. Commit the configuration and shared script so the team uses the same definitions.

Example project layout:

```text
your-project/
├── .pitstop.json
├── package.json
├── package-lock.json
└── scripts/
    └── check.sh
```

You do not need to commit every edit before running checks: Pitstop tests the current working tree, but the repository must already have a HEAD commit.

## Validate and install the agent skill

Run validation before `doctor` or `run`. It reads `.pitstop.json` without Docker, image pulls, cache writes, project commands, or shell evaluation. A valid result has `valid: true`, selected jobs, architecture, definition counts, and `issues: []`; invalid config returns exit `2`, one bounded issue, and error code `CONFIG_INVALID`.

```sh
pitstop validate --repo "$TARGET_REPO" --json
```

Install the portable skill with an explicit agent and scope. Project scope resolves the Git root; user scope uses the user's home and does not accept `--repo`. Use `--dry-run` in an isolated target to inspect files. `--force` replaces only an owned Pitstop bundle whose manifest is current or explicitly trusted historical content.

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
pitstop skill install --agent claude --scope user --dry-run --json
```

Codex user installs live at `~/.agents/skills/pitstop-config`; Claude user installs live at `~/.claude/skills/pitstop-config`. Project installs use the matching hidden directory under the Git root. Installation never edits `AGENTS.md`, `CLAUDE.md`, workflow files, or project scripts. See the [AI-agent guide](ai-agent.en.md) for the workflow and manual prompt fallback.

## Configuration reference

All fields below are required except `name`. Image, architecture and jobs have no automatic defaults.

| Field | Format and meaning |
| --- | --- |
| `schemaVersion` | Must be `2` |
| `runner.image` | Image/tag/digest containing all required tools; nonempty, no whitespace/control characters, no leading `-` |
| `runner.architecture` | `linux/arm64` or `linux/amd64` |
| `jobs` | Nonempty, unique default selection of IDs defined in `checks`; determines execution order |
| `checks` | Nonempty array with unique check IDs |
| `checks[].id` | Check ID referenced by `jobs` and `--job` |
| `checks[].steps` | Nonempty array, executed in order |
| `steps[].id` | Step ID, unique within its check |
| `steps[].name` | Optional readable log/report name; defaults to the ID |
| `steps[].run` | Nonblank shell command without NUL; use JSON `\n` for multiple lines |

IDs contain 1–64 characters. Start with an ASCII letter or digit; remaining characters may also include `_` and `-`. Examples: `verify`, `unit-test`, `build_web`.

Use one JSON document, without comments. Unknown fields such as `env`, `uses` or `engine` are rejected. Set environment variables inside a step's command/script if needed; host shell variables are not automatically forwarded into containers.

Step `run` values intentionally contain shell code executed in containers. `pitstop validate` applies the shared strict parser and bounded checks. It rejects duplicate or unknown fields, malformed Unicode, unsafe file types, unsupported schema versions, and values beyond documented limits. Validation does not certify command/image safety, dependency availability, runtime success, or GitHub CI coverage. See the [validation design](superpowers/specs/2026-10-08-config-agent-design.md).

**`jobs` selects checks, not steps.** You may define several checks and select only some:

```json
"jobs": ["lint", "test"]
```

This is a configuration fragment, not a complete file. Corresponding `lint` and `test` checks must exist. `--job test` selects only `test`; `--job build` is rejected when `build` is outside `jobs`, even if defined in `checks`. Edit the configuration selection first.

### Separate lint and test checks

To select each group independently, use this configuration, assuming the project defines `lint` and `test` scripts:

```json
{
  "schemaVersion": 2,
  "runner": {
    "image": "node:22-bookworm",
    "architecture": "linux/arm64"
  },
  "jobs": [
    "lint",
    "test"
  ],
  "checks": [
    {
      "id": "lint",
      "steps": [
        {
          "id": "install",
          "run": "npm ci"
        },
        {
          "id": "lint",
          "run": "npm run lint"
        }
      ]
    },
    {
      "id": "test",
      "steps": [
        {
          "id": "install",
          "run": "npm ci"
        },
        {
          "id": "test",
          "run": "npm test"
        }
      ]
    }
  ]
}
```

Both checks install their dependencies because they use separate fresh containers. To install once and perform every check, keep the steps in one `verify` check as in the first example.

## Step and container behavior

1. Validate config/environment and lock the repo against overlapping runs.
2. Capture the working tree. Changes during capture cause `SNAPSHOT_CHANGED`.
3. Inspect/pull the image when missing or on the wrong platform, with a two-minute preparation budget.
4. Create a fresh container per check and copy the snapshot into `/workspace`, without mounting the repo or Docker socket.
5. Run each step through `/bin/sh -e` at `/workspace`, as user `0:0` (root inside the container).
6. Collect step records, terminal state and logs; remove containers and temporary snapshots/scripts.

Steps in one check share files: dependencies installed by `npm ci` remain available to later tests. **Every step starts a new shell**, so `cd` and `export` do not carry over:

```json
{"id":"test","run":"cd frontend && npm test"}
```

Put commands in one step/script when they need the same shell variables or directory. `/bin/sh -e` does not enable `pipefail`; explicitly check pipeline exits or invoke a shell available in the image with the desired options.

A failed step skips the rest of that check. Later independent checks still run from fresh copies of the starting snapshot. Cancellation prevents scheduling the remaining checks.

There are no persistent cache volumes across checks/runs. Generated container build artifacts are removed after execution; exporting artifacts back to the repo is not supported yet.

## Choose or build an image

Start with an image containing your project's tools, such as the Node or Go examples. Dependency installation belongs in steps; Pitstop does not infer whether to run `npm ci`, `pnpm` or another command.

When more tools are needed, create `Dockerfile.checks` in the target repository. For example, a Node project that needs Git:

```dockerfile
FROM node:22-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
```

Build it manually before running Pitstop:

```sh
docker build -t my-project-checks:dev -f Dockerfile.checks .
```

Set `runner.image` to `my-project-checks:dev` and choose the image's architecture. Pitstop uses the daemon's image; it does not build it. Rebuild after changing the Dockerfile.

Tags can move. For reproducible toolchains, use an actual registry digest such as `your-image@sha256:<digest>`, replacing `<digest>` with the real value. Reports record the resolved platform image ID and the digest when available.

Log in to private registries with the host Docker CLI before running. Host credentials are not mounted inside check containers.

## Commands and flags

`--repo` defaults to `.` and resolves to the Git root. Configuration is read there. Quote paths containing spaces.

| Command | Purpose | Additional flags |
| --- | --- | --- |
| `pitstop doctor` | Probe repository/config, Git/Docker and image readiness | `--json` |
| `pitstop list` | List selected/excluded checks from config; does not require Docker | `--json` |
| `pitstop run` | Execute selected checks | `--job`, `--timeout`, `--arch`, `--json` |
| `pitstop version` | Version and build information when available | `--json` |
| `pitstop -h` | Help | Every command supports `-h`/`--help` |

```sh
pitstop doctor --repo "/path/to/project"
pitstop list --repo "/path/to/project" --json
pitstop run --repo "/path/to/project" --job lint --job test
pitstop run --repo "/path/to/project" --job lint,test --timeout 10m
pitstop run --repo "/path/to/project" --arch linux/amd64
pitstop run --repo "/path/to/project" --json > result.json 2> progress.log
```

Repeat `--job` or separate IDs with commas; IDs must be unique and in the configured selection. `--arch` overrides this run without editing config. `--timeout` must be positive and covers each check's create/copy/start/execution phases; default 20 minutes. Image preparation and cleanup have separate budgets.

Ctrl+C or SIGTERM cancels the run. Bounded stop/remove can take additional time after a timeout. Check warnings for `CLEANUP_INCOMPLETE` when the daemon cannot respond.

`doctor` does not pull images or execute project commands. A missing image may be `ready` with a message that run will pull it. Readiness does not guarantee registry access, dependencies or passing tests.

### Docker endpoint

Endpoint precedence is `DOCKER_CONTEXT` → `DOCKER_HOST` → active Docker context. These host variables select the daemon; they do not configure the step environment. Check with:

```sh
docker context show
docker version
```

## Reports and logs

Human mode writes progress and results to stdout. With `--json`, stdout is one JSON envelope and progress goes to stderr, allowing separate output files.

The output envelope uses `schemaVersion` **1**; configuration uses **2**. They are separate schemas. Read `exitCode`, `result.jobs`, coverage and warnings together.

| Exit code | Meaning and precedence |
| --- | --- |
| `130` | Cancellation; highest priority |
| `1` | At least one failed job, including timeout/runtime failure; preserved despite additional unknown/storage/cleanup errors |
| `2` | No failed job, but invalid config, unavailable environment, unknown outcome or incomplete logs/reports/cleanup |
| `0` | All selected jobs passed without result-integrity problems |

| Run result field | Interpretation |
| --- | --- |
| `jobs[].status` | `passed`, `failed` or `skipped`; inspect reasons and steps |
| `jobs[].engineExitCode` | Container exit code when evidenced; may be `null` |
| `jobs[].steps` | Per-step status and `evidenceRef`; step duration is currently `null` |
| `selectedCoverage` | `complete` for evidenced ordinary checks, including normal test failures; `partial` for interrupted/runtime/unknown outcomes |
| `workflowCoverage` | `not_applicable`; workflow YAML is not assessed |
| `warnings` | Additional log/cleanup problems; job status alone is insufficient |
| `source` | HEAD, dirty state, snapshot hash and excluded paths |
| Envelope `error` | Errors such as invalid config or failed report persistence; may accompany a run result |
| `engine` | Image, platform, resolved image ID/digest and Docker version |
| `reportPath` | Report location on the host running Pitstop |

`complete` does not mean tests passed. `skipped` can indicate an unknown outcome rather than an intentionally unselected step.

Reports are stored under `<user-cache>/pitstop/runs/<runId>/`. On macOS this is normally `~/Library/Caches/pitstop/runs/`; on Linux, `~/.cache/pitstop/runs/` or the configured cache directory. Use the returned `reportPath` as the authoritative location.

| File | Contents |
| --- | --- |
| `report.json` | Run JSON envelope |
| `<check-id>.stdout`, `<check-id>.stderr` | Raw check output |
| `evidence-<check-id>/` | Step exit records and completion marker |
| `config.json`, `effective-config.json` | Captured config and config after overrides |
| `manifest.txt`, `metadata.json` | Source identity, hashes and command arguments |
| `argv-*.json`, `engine-meta.json`, `owned-resources.json` | Arguments and container ownership records |

Failure before checks begin may have no `result` or `reportPath`; inspect `error.code` and `error.message`. There is no automatic report retention policy yet.

## Snapshot contents

- Tracked files use current contents, including edits and deletions.
- New files not ignored by Git are included.
- Ignored untracked files and untracked `.env`, `.env.*`, `.secrets`, `.vars`, `.input` files are excluded.
- Tracked files remain included even with those names; exclusion is not a secret scanner.

Install ignored dependencies inside the container instead of relying on host `node_modules`. Submodules, unresolved merge conflicts, absolute symlinks, broken symlinks and symlinks escaping the repository are unsupported.

## Share scripts with GitHub CI

Put check logic in a project script and call it from both Pitstop and CI. After CI's normal checkout/toolchain setup:

```yaml
- name: Install dependencies
  run: npm ci
- name: Check project
  run: sh scripts/check.sh
```

Pitstop prepares tools through an image and source through a snapshot. CI prepares its own environment. Shared scripts reduce duplicated commands; matching scripts do not guarantee matching OS/tool versions/network behavior.

## Migrate an older configuration

Schema version 1 is rejected. Start from a v2 example; changing the version number alone is insufficient.

| Previous definition | New definition |
| --- | --- |
| `workflow`, `event`, `adapter`, `runner.label` | Remove |
| Checkout step | Pitstop automatically captures the source |
| Toolchain setup step | Choose an image containing those tools |
| Workflow shell commands | Move into `checks[].steps[].run` or a shared script |
| Job selection | Set `jobs` to the new check IDs |

## Troubleshooting

| Symptom | Check or fix |
| --- | --- |
| `CONFIG_INVALID` / missing config | Create `.pitstop.json` at Git root, use version 2 and follow field/ID rules |
| `TOOL_MISSING` | Put Git/Docker CLI in the invoking process's PATH |
| `unknown flag: --platform` | Update the Docker CLI/daemon to support platform-qualified image inspection |
| `DOCKER_UNAVAILABLE` | Start the daemon; check context/host, endpoint access, registry login and image/platform |
| doctor ready, pull fails | Doctor does not pull; check connectivity, credentials and image/tag |
| Command not found / failed step | Install tools in the image or correct the command; inspect check stderr/stdout |
| `RUN_ALREADY_ACTIVE` | Wait for the previous run in this repository; separate repositories can run independently |
| `SNAPSHOT_CHANGED` | Pause file/index writers; resolve merge/submodule/symlink issues and retry |
| `TIMEOUT` | Inspect the unfinished step; fix the command or adjust `--timeout` |
| `ENGINE_OUTCOME_UNKNOWN` | Evidence is incomplete; inspect Docker/logs/evidence and retry, never assume passed |
| `CONTAINER_RUNTIME_FAILED` | Inspect OOM/runtime reason and Docker resources |
| `LOG_CAPTURE_FAILED` / `REPORT_WRITE_FAILED` | Check output/cache space and permissions; existing failures retain priority |
| `CLEANUP_INCOMPLETE` | Inspect warnings/ownership metadata and exact run labels before manual cleanup |

Inspect one run's containers, replacing `RUN_ID` with `result.runId`:

```sh
docker ps -a --filter "label=com.pitstop.run=RUN_ID"
```

## Supported scope

Prebuilt Docker images, shell/shared scripts, ordered steps, selection, snapshots, timeout/cancellation and reports are supported. There is no workflow YAML executor, `uses:`/expression translation, matrix, job dependencies (`needs`), service containers, secret injection, cache volumes, automatic image builds, artifact export, self-hosted runner registration, automatic workflow conversion, or automatic manifest detection.

See [requirements](requirements.en.md) and [validation](evidence/docker-checks-validation.md). Earlier designs and evidence are listed as historical in the [documentation index](README.md).
