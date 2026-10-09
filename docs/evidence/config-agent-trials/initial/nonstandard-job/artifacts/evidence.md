# Evidence

## Selected scope

- One ordinary verification job: `review-gate`.
- Runner: `node:22-bookworm`, `linux/arm64`.
- One step: `npm run check`.
- Release work was excluded; `npm publish` is not selected.

## Evidence inspected

- `package.json`: `packageManager` is `npm@10.9.2`; `check` is `npm test`; `release` is `npm publish`.
- `package-lock.json`: lockfile version 3 is present.
- `.github/workflows/quality.yml`: job `cobalt` uses `node:22-bookworm` and runs `npm run check`.
- Installed `.agents/skills/pitstop-config` instructions and its nonstandard-name mapping reference.

## Assumptions and gaps

- The workflow's explicit verification command is preserved as the Pitstop step, while the package script evidence shows that it delegates to `npm test`.
- The pinned npm version is recorded as a prerequisite from `package.json`; the selected image tag is taken from the workflow. Image availability and runtime success were not tested.
- `validate` proves config shape and selection only. It does not prove dependency installation, image availability, command success, Docker isolation, or CI parity.
- No project commands ran. No `pitstop run`, Docker command, dependency install, test, or release command was executed.

## Agent metadata

- Actual Codex subagent: `luna_worker`, model `gpt-5.6-luna`, reasoning `high`.
- Client version: unavailable.
- Skill use: read the installed `pitstop-config` skill and only its directly needed references.

## Command list

All shell invocations were prefixed with `rtk`; CLI calls used `rtk proxy`:

1. `pitstop-phase-a version` — exit `0`.
2. `pitstop-phase-a -h` — exit `0`.
3. `pitstop-phase-a validate -h` — exit `0`.
4. `pitstop-phase-a validate --repo <fixture> --json` — exit `0`; `valid: true`, one job, one check, one step, no issues.
