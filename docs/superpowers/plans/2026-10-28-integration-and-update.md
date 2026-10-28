# Integration and update implementation plan

> **For agentic workers:** Implement one task/branch at a time with `luna_worker`; use TDD and verification-before-completion. The user approved the design and execution method. Do not ask for that approval again.

**Goal:** One-command CLI/skill upgrades and evidence-backed Go integration setup with real Testcontainers execution.

**Architecture:** Reuse the installer and skill ownership protocol for updates. Extend individual Docker checks with an opt-in dedicated DinD service. Extend the portable skill only after the runner supports it.

**Tech Stack:** Go 1.26.5, Cobra, existing shell installer, Docker, embedded Markdown assets.

**Spec:** [Approved design](../specs/2026-10-28-integration-and-update-design.md).

## Global constraints

- Every shell command starts with `rtk`; use `rtk proxy` for raw tools/output.
- Separate branches, sequential implementation; parent reviews/checks each before merge and the next task. No main-branch product edits.
- Never modify, execute, copy or publish the private example repository; use new generic fixtures and isolated HOME/bin/project directories.
- Preserve unknown/user-modified skills, existing validation and error handling. No dependencies for trivial helpers.
- No host Docker socket sharing, automatic privileged execution, generic services/compose engine, new release or tag movement.
- Capture actual RED/GREEN and verification output in task-local temporary logs, then retain useful evidence in docs.
- Follow source-build versus published-binary distinctions. Published v0.1.0 cannot acquire an update command without its initial installer upgrade.

## Review focus

- Old-process assets must never overwrite a newly downloaded binary's skill bundle (Task 1 subprocess regression).
- Same-version source builds must not be silently replaced by older released bundle bytes (Task 1 preservation test).
- Untrusted config cannot enable privileged containers by itself (Task 2 authorization test before any allocations).
- Readiness failures/cancellation must leave neither owned containers/volumes nor networks (Task 2 failure/runtime tests).
- Tagged/env-gated/nested-module tests must not disappear behind a passing smaller scope (Task 3 agent fixtures).

## Task 1 — Self-update

**Branch:** `feature/self-update`.

**Files:** Create `installer_assets.go` (small root asset package embedding `scripts/install.sh`), `internal/update/update.go`, `internal/update/update_test.go`, `internal/cli/update.go`, `internal/cli/update_test.go`. Modify `internal/cli/root.go`, `internal/model/types.go`, `internal/report/human.go` or its actual existing human formatter, README/Thai README, both user guides, `docs/install.md`, `docs/ai-agent*.md` if behavior needs a note, docs index/roadmap. Retain `scripts/install.sh` as the single script source.

**Interfaces:** `update.Execute(ctx context.Context, opts Options) (model.UpdateResult, error)`; options carry current version, actual executable path, repository/default-versus-explicit repo, optional target version and progress writer. Result carries executable/version/CLI status, per-existing-skill outcomes and partial state. CLI owns the JSON envelope and command exit mapping.

- [x] Write updater regression tests before implementation: staged/new binary supplies skills, all four existing known scope/agent targets, absent targets untouched, unknown/modified preflight refusal and CLI byte preservation, default same/newer version behavior, malformed versions/redirects, failed download/checksum/version, unsafe target/lock contention, cancellation and late partial completion. Keep test hooks internal and deterministic.
- [x] Run focused tests RED; retain output and check failures concern missing updater behavior.
- [x] Embed and reuse the checked-in installer; implement bounded latest-tag resolution, same-filesystem private staging, verified new-binary preflight, locked atomic replacement and actual new-process skill installation. No generic updater framework or recursive skill search.
- [x] Implement `pitstop update [--version vX.Y.Z] [--json]`, inheriting `--repo`. Outside implicit non-Git cwd refresh user skills only; explicit invalid repo blocks before changes. Return 0/2/130 per spec; blocked dry-run action is not success; JSON progress stays on stderr.
- [x] Run focused updater/CLI tests GREEN; validate actual subprocess argv/result validation, all preservation and partial-state cases. Capture structured output from isolated end-to-end temp binaries without touching installed tools.
- [x] Update bilingual command/help/installation docs; describe one-command automatic behavior, scope limits, first upgrade from v0.1.0 and no automatic release publication. Cross-check help/links.
- [x] Run `rtk proxy env GOTOOLCHAIN=go1.26.5 go test -race -count=1 ./...`, `go vet ./...` with the same toolchain, and fresh-build/offline smoke. Avoid toolchain downloads into the smoke's disposable HOME by using the already installed Go toolchain bin directory with `GOTOOLCHAIN=local`.
- [x] Parent reviews full branch diff against the design, verifies checks, commits and merges before Task 2.

## Task 2 — Testcontainers runner

**Branch:** `feature/testcontainers-runner`, from the merged Task 1 base.

**Files:** Modify `internal/model/types.go`, strict shape validation in `internal/config/parse.go` and related parser/model tests, `internal/run/run.go`/tests, `internal/cli/root.go`/tests, `internal/dockerchecks/{engine,environment}.go`/tests. Add focused `internal/dockerchecks/testcontainers.go`/tests if it keeps provisioning/cleanup clear. Add `scripts/testcontainers-smoke.sh` and a generic fixture below `scripts/testdata/testcontainers/` (its own pinned Go module), and update CI/docs/command reference accordingly.

**Interfaces:** Optional `model.Check.Testcontainers bool` serialized as `testcontainers,omitempty`; `run.Options.AllowTestcontainers bool` bound to `run --allow-testcontainers`. Add optional per-job daemon identity/diagnostics metadata without changing ordinary results. Existing `dockerchecks.Run` receives the effective authorized check; authorization occurs before all runtime allocations in `run.Execute`.

- [x] Write/run parser tests for optional bool, omitted/false compatibility, null/wrong type/unknown fields; write/run RED authorization test asserting no Docker/resource calls for any selected flagged job without authorization, including mixed ordered selections.
- [x] Add the schema/flag/plumbing; preserve override selection semantics and ordinary checks. Document doctor readiness limitations without running privileged probes.
- [x] Write/run lifecycle and recovery tests for separate labelled network/service, private unpublished port, unprivileged runner/no host binds, fixed environment only, startup failure, copy failure, timeout/cancel and ownership-preserving cleanup.
- [x] Implement fixed `docker:29.8.2-dind` per-check provisioning, immutable platform image identity, fresh run-labelled ephemeral data, private network alias, bounded readiness and daemon diagnostics. Include provisioning in the per-check deadline and keep image preparation bounded separately.
- [x] Extend per-check cleanup and final label-based recovery for containers, labelled ephemeral volumes and networks. Record ownership before vulnerable operations. Preserve unrelated resources and current exit/coverage/warning precedence.
- [x] Run parser/run/CLI/engine tests GREEN. Add/run real Go Testcontainers service round-trip, passing and failing assertions, missing authorization, daemon-startup failure, timeout and SIGINT smoke; assert labelled containers/volumes/networks removed. Retain raw JSON/reports/diagnostics.
- [x] Add Linux amd64 CI smoke and document independently observed local ARM64 results. Update bilingual config/command/limits references, including privileged opt-in and public-image requirements.
- [x] Run full race/vet, fresh-build, existing offline/Docker smoke and the new Testcontainers smoke. Capture only generic fixture data in docs evidence.
- [x] Parent reviews failure paths and trust boundaries, verifies runtime evidence, commits and merges before Task 3.

## Task 3 — Go integration setup and bundle 3

**Branch:** `feature/go-integration-setup`, from the merged Task 2 base.

**Files:** `internal/agentassets/bundle/SKILL.md`, existing Go/example/workflow references, generic source/workflow/mapping fixtures, `manifest.json`; `internal/agentassets/assets.go`/tests/history; installer upgrade tests; bilingual guides/README/index/roadmap; generic agent trial evidence below `docs/evidence/go-integration-setup/`.

**Interfaces:** The skill consumes the actual `update` and `run --allow-testcontainers` surfaces and optional per-check capability from Tasks 1–2. Bundle version becomes `3`; existing bundle-2 and actual v0.1.0 installs remain authenticated upgrade sources, not destination-supplied trust.

- [x] Preserve exact current bundle-2 manifest/changed bytes if no equivalent history exists. Write/run RED upgrade/protection tests before changing bundle content. Read skill-creator/writing-for-agents/writing-skills guidance.
- [x] Run a direct-read baseline on generic Go fixtures containing tagged tests, external/env skips, nested-module evidence, and an existing unit-only config; preserve raw prompts and actual results without claiming a failure unless observed.
- [x] Extend instructions to inventory test groups/helpers/CI/build tags and compare requested versus configured coverage. Preserve existing checks while adding only evidence-backed authorized gaps. Keep the generic Go starter generic.
- [x] Teach Testcontainers opt-in and actual authorization requirement, no host/secret fallback, Go skip/empty-target reporting and separation of unit/integration/external-service results. Include installed-command compatibility and current-versus-released bundle notes.
- [x] Bump to bundle 3, update allowlist and regenerate manifest hashes in existing formatting. Run focused asset/ownership/upgrade tests GREEN.
- [x] Run isolated direct-read agent trials after the change, including real permitted Testcontainers execution, unit-only existing config, skipped external suite and refusal to silently elevate without capability authorization. Confirm original fixture tests/source stay unchanged.
- [x] Update docs and roadmap with actual evidence; run full race/vet, fresh-build/offline smoke and relevant runtime smoke after final changes. Check manifests, help, bilingual links/examples and public evidence contents.
- [x] Parent reviews the final change and evidence, commits/merges, pushes the completed work to main and checks CI. No binary release is created.

## Progress

- Design approved; user explicitly chose `luna_worker` implementation on 2026-10-28.
- Task 1 verified with full race/vet/offline smoke plus real compiled CLI/installer acceptance; [evidence](../../evidence/self-update/README.md). Separate existing-test timing fix: `feature/captured-config-test`, commit `1678567`. Task 1 merged locally in commit `333e131`; Task 2 passed full race/vet, offline/Docker smoke and all eight real ARM64 Testcontainers cases; [evidence](../../evidence/testcontainers/README.md). Parent review has no remaining findings. Task 3 remains pending.

- Task 2 merged locally as `eed3d67`. Task 3 bundle-3 upgrade, full race/vet/offline checks and actual direct-read Go execution trials passed; [evidence](../../evidence/go-integration-setup/README.md). Merged/pushed as `c530cf4`; [final CI](https://github.com/captainthx/pitstop/actions/runs/40696364818) passed Verify Pitstop (including real Linux amd64 Testcontainers) and both Linux/macOS installers. No release was created.
