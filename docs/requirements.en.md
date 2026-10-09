# Pitstop requirements and scope

[ไทย](requirements.md) | [User guide](user-guide.en.md) | [Index](README.md)

This document defines current Docker checks behavior. Installation and runnable examples are in the user guide. Config-agent mapping/prerequisite trial evidence is recorded separately in the [trial evidence index](evidence/config-agent-trials/README.md).

## Requirements

- Host Git, Docker CLI and a reachable daemon; build the CLI with Go 1.26.5 from `go.mod`.
- Target repository in Git; `run` needs a HEAD commit and schema version 2 `.pitstop.json` at Git root, while `init` and `validate` do not need HEAD.
- Configuration may be created manually, with `pitstop init`, or by the installed portable agent skill. `init` uses explicit presets and never auto-detects a project manifest.
- Linux `arm64` or `amd64` image containing `/bin/sh` and every command's tools.
- Registry/network access as required by image pulls and project commands.
- Local real execution is validated on macOS ARM64 / Docker Linux ARM64; native Windows and cross-architecture execution are unverified.

## Supported behavior

| Component | Contract |
| --- | --- |
| Config | One JSON document, version 2, strict fields/IDs; manual migration from earlier schema |
| Commands | `doctor`, `list`, `run`, `version`, `init`, `validate`, `skill install`; `-h`/`--help` and JSON output |
| Selection | `jobs` defines default selection/order; `--job` chooses a subset |
| Environment | Use a daemon image or pull when needed; two-minute preparation budget; no automatic build |
| Snapshot | Current tracked edits/deletions and new non-ignored files; original repo is not mounted |
| Isolation | Fresh container per check; no host bind mount, Docker socket mount or privileged mode |
| Steps | `/bin/sh -e`, cwd `/workspace`, user `0:0`; shared files and a new shell per step |
| Failure | Failed step skips the rest of that check; later independent checks still run |
| Timeout | Default 20 minutes per check, positive `--timeout` override; covers create/copy/start/run |
| Cancellation | Ctrl+C/SIGTERM stops scheduling and attempts bounded stop/removal of owned containers |
| Concurrency | Per-repository lock; other repositories may run independently |
| Evidence | Docker terminal state and step exit records; success text alone is insufficient |
| Reports | Logs, source/image identity, configs, arguments, exit evidence and ownership records in user cache |
| Cleanup | Remove owned containers/volumes, snapshots and temporary scripts; retain reports/logs; no image/global resource pruning |

## Result contract

Configuration schema is 2; JSON envelope schema is 1. `workflowCoverage` is always `not_applicable`. `selectedCoverage=complete` means ordinary selected checks have sufficient evidence, not that they passed. Interrupted/runtime/unknown outcomes produce partial coverage.

Exit precedence: cancellation `130` → failed job `1` (including timeout/runtime failure) → invalid/blocked/unknown/log/report/cleanup problem `2` → all passed `0`. Storage and cleanup problems do not hide established failures.

Step durations are `null`; check/run durations are measured. Pre-execution errors may have no report. There is no automatic report retention or artifact export.

## Unsupported features

- Execution of `.github/workflows/*.yml`, `uses:` actions or expressions.
- Matrix, job dependencies (`needs`) and service containers.
- Config-based secret/environment injection, host environment forwarding or cache volumes.
- Automatic image builds, workflow/config conversion, and automatic manifest/preset detection.
- Self-hosted runner registration or uploading source to GitHub.
- A provided installer link/release installation method.

Snapshots reject submodules, unresolved merges and absolute/broken/escaping symlinks. Ignored untracked files and untracked `.env`/`.env.*`/`.secrets`/`.vars`/`.input` files are excluded; tracked files remain included. Exclusion is not secret scanning.

CI can reuse project scripts, but matching results depend on the image/toolchain/OS/network. Workflow parity is not guaranteed.

## Config and AI agent tooling (2026-10-09)

The [design](superpowers/specs/2026-10-08-config-agent-design.md) and [implementation plan](superpowers/plans/2026-10-08-config-agent.md) document the implemented base/starters across languages/build tools, `init`, `validate`, and `skill install`. A user's agent reads manifests/scripts/workflows to create configuration; Pitstop does not embed an AI provider or execute workflow YAML. The [config-agent trial evidence](evidence/config-agent-trials/README.md) records 23 evaluator attempts for mapping and prerequisite reporting; runtime/toolchain checks are separate evidence.

Validation shares one acceptance pipeline with run/doctor/list, covering strict JSON, bounded input, safe file access, semantic limits and regression/fuzz checks. Passing validation means the config follows supported rules, not that shell/image execution is safe or checks cover all CI behavior. Selected runtime and whole-branch verification are recorded in [validation evidence](evidence/config-agent-validation.md) and [review with deferred test gaps](evidence/config-agent-review.md); see the [roadmap](roadmap.md) for deferred scope and development triggers.

## Validation

See [current Docker validation](evidence/docker-checks-validation.md) for tests, smoke and limits of the actual evidence. GitHub CI includes smoke; local validation does not establish remote CI success. Historical documents in the [index](README.md) do not define the current CLI's scope.
