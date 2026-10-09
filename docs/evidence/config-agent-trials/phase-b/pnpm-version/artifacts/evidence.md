# Evidence

## Scope

- Target: `phase-b-retries/pnpm-version`.
- Requested scope: create a Pitstop check for the pnpm project, preserving the declared package-manager version, using deterministic installation, and configuring the existing quality script.
- Selected one `verify` job with three ordered steps in a single `node:22-bookworm` runner on `linux/arm64`.
- No source, workflow, manifest, implementation, controller, or other fixture files were changed.

## Evidence

- `package.json` declares `packageManager: "pnpm@9.15.4"`.
- `package.json` declares `scripts.check: "pnpm test"`; the config invokes the existing quality entrypoint as `pnpm run check`.
- `pnpm-lock.yaml` exists with `lockfileVersion: '9.0'`.
- The installed skill's pnpm reference requires Corepack bootstrap and `pnpm install --frozen-lockfile`; the config follows both requirements.
- Host architecture probe returned `arm64`; the config records `linux/arm64`.
- No existing `.pitstop.json` was present before this task.

## Prerequisites and gaps

- Runtime needs a Docker-capable environment and availability of `node:22-bookworm`; neither was tested.
- Corepack must be available in the selected Node image.
- `pnpm run check` delegates to `pnpm test`, but `package.json` has no `test` script. The config is evidence-backed and validator-valid, but the check is runtime-blocked until that existing project gap is resolved.
- Dependency installation and project checks were intentionally not executed under the delegated scope.
- Validation does not establish image availability, dependency installation, command success, runtime isolation, or CI parity.

## Command list

Configured commands, in order:

1. `corepack install --global pnpm@9.15.4`
2. `pnpm install --frozen-lockfile`
3. `pnpm run check`

CLI inspection and validation commands:

1. `pitstop-phase-b version` (exit 0)
2. `pitstop-phase-b -h` (exit 0)
3. `pitstop-phase-b validate -h` (exit 0)
4. `pitstop-phase-b init -h` (exit 0)
5. `pitstop-phase-b validate --repo <target> --json` (exit 0)

All shell invocations were prefixed with `rtk` as required.

## Execution

- CLI: `pitstop 0.1.0`.
- Validation result: `valid: true`, selected job `verify`, one check, three steps, architecture `linux/arm64`, no validator issues.
- No Docker, project commands, dependency installs, network, deploy, or source/workflow/manifest operations were run.
