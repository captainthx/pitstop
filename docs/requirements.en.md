# Pitstop MVP requirements and scope

**English** | [ไทย](requirements.md)

**Before running: prepare a Git repository, workflow, `.pitstop.json`, act 0.2.89, and a reachable Docker daemon.**

This document separates user prerequisites from requirements Pitstop must meet. [User guide](user-guide.en.md) · [Home](../README.md)

## 1. System requirements

| Requirement | Details |
|---|---|
| Git | Available on PATH |
| act | **0.2.89**; other versions are rejected in the MVP |
| Docker | Reachable daemon capable of running the configured image |
| Go | **1.26.5 or newer** for source builds; not needed just to start a prebuilt binary |
| Storage | Enough for a repository copy, images, caches, reports, and logs; no measured minimum is claimed |
| Network | Access to images, actions, and workflow dependencies when they are not cached |

Pitstop checks readiness but does not install tools. The workflow and runner image prepare project tools such as Node.js or Go inside the runner.

**Actually validated environment:** macOS ARM64, act 0.2.89, Docker 29.4.0/OrbStack, and a Linux ARM64 runner. Evidence covers React verify, Go lint/unit tests, and PostgreSQL/Testcontainers integration.

Configuration also accepts `linux/amd64`, but AMD64 application execution has not been validated. Host support for every operating system is not claimed.

## 2. Project requirements

| Item | Requirement |
|---|---|
| Repository | A Git repository with at least one commit for `run` |
| Workflow | One file per configuration, inside the repository, ending in `.yml` / `.yaml`, with a `pull_request` trigger |
| Configuration | Root-level `.pitstop.json`, schema version `1`, valid JSON without unknown fields |
| Job selection | At least one unique ID that exists in the workflow |
| Runner | Explicit label, image, and architecture; label matches the job's `runs-on` |
| Source | No unresolved merge, submodule, or rejected symlink |

Configuration selects jobs and the runner. Validation commands remain in the workflow.

## 3. Implemented features

| Feature | What it provides |
|---|---|
| `doctor` | Preflight checks for repository, configuration, workflow, and tools |
| `list` | Selected/blocked jobs and reasons |
| `run` | Sequential execution of selected jobs through act and Docker |
| `version` | Program version |
| Working-tree snapshot | Tests edited and uncommitted files |
| Job subset | Repeated `--job` flags select from the configuration |
| JSON output | Machine-readable results with progress on stderr |
| Reports and raw logs | Results, source identity, workflow evidence, and actual argv |
| Timeout and cancellation | Per-job time limit and Ctrl+C handling |
| Repository lock | Prevents overlapping runs for the same repository |
| Cache | Reuses action/dependency caches |
| Service integration adapter | Explicit opt-in integration with reported workflow adjustments |

## 4. Supported workflows

The generic runner supports **independent jobs on a static runner, without conditions that decide whether selected work executes**, such as lint, unit tests, or verify.

| Pattern | MVP behavior |
|---|---|
| Independent job, static `runs-on`, supported executable steps | Supported when the runner matches configuration |
| Local composite action | Metadata and nested composites are inspected; unsupported conditions/continue-on-error are blocked |
| `needs`, job-level `if` | Blocked except for the validated service adapter shape |
| Step condition | Conditions other than literal `true` are blocked |
| Matrix / strategy | Blocked |
| Dynamic runner / needs | Blocked |
| Reusable workflow | Blocked |
| Job `container` / `services` | Blocked |
| `continue-on-error` | Blocked at job or step level |
| Secret references | Blocked, including references in local composites |
| Empty jobs or jobs that only skip work | Never counted as a successful execution |

act manages remote action internals. Inspecting local metadata does not mean every action's internal behavior is inspected. Example profiles exclude deployment jobs; select the validation work you intend to execute.

## 5. MVP behavior requirements

| ID | Requirement | User-visible behavior |
|---|---|---|
| R01 | Reuse the original workflow | Lint/test/build commands stay in the workflow; config selects work |
| R02 | Test current source | Working tree overrides staged content; a snapshot hash is retained |
| R03 | Isolate original source | Execute a temporary copy and detect source changes during capture |
| R04 | Make selection explicit | Never silently add dependency jobs; explain unsupported work |
| R05 | Report evidence-backed outcomes | Exit 0 needs selected-job evidence; uncertainty never becomes passed |
| R06 | Use stable exit precedence | Cancelled `130` → executed failure `1` → partial/preflight `2` → success `0` |
| R07 | Preserve completed outcomes | An earlier failure survives a later job launch error |
| R08 | Provide machine-readable JSON | Schema version `1`, one stdout document, progress on stderr |
| R09 | Report completeness and changes | Separate selected/workflow coverage; forced integration reports adjustments |
| R10 | Isolate local configuration | Temporary HOME/config; host tokens, secrets, and actrc are not engine inputs |
| R11 | Bound execution and cleanup | Cancellation/timeout support and cleanup scoped to ownership-labeled runners |
| R12 | Retain inspection evidence | Reports, logs, manifests, workflow hashes, timing, and argv remain; temporary snapshot/HOME are removed |

Tracked `.env` files remain source inputs under R02. Configuration isolation under R10 does not remove files already tracked in Git.

## 6. Integration adapter scope

`recruit100-service-integration-v1` requires both `integration` in the configured selection and the explicit `--force-integration` flag. Declaring the adapter alone does not enable forced execution.

- Verify the supported integration job shape and command.
- Remove only integration's `needs` and `if` from a workflow copy.
- Require a local Unix Docker endpoint; integration receives the socket for Testcontainers.
- Keep `workflowCoverage=partial` on success because original conditions were changed.

The normal service profile can run lint/test while visibly blocking integration.

## 7. Current limitations

- Submodules and absolute source symlinks are unsupported. Relative links must resolve inside the repository.
- Local tokens/secrets are not supplied to make credential-dependent work pass.
- Native conditional paths-filter support for the service workflow remains deferred.
- `steps: []` cannot establish detailed step/test counts; only dependable evidence is reported.
- Reports/logs have no automatic retention expiry. Users remove completed runs manually.
- Local success still needs required GitHub checks and does not establish deployment readiness.

## 8. Evidence and source documents

[Actual validation](evidence/final-validation.md) · [Audit and resolutions](evidence/final-review.md) · [Original specification](design/pitstop-spec.md) · [Implementation plan](superpowers/plans/2026-10-05-pitstop-mvp-1.md)
