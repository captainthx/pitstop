# Evidence

## Selected scope

One `verify` job with three ordered steps on `node:22-bookworm` and explicit `linux/arm64` architecture:

1. `corepack install --global pnpm@9.15.4`
2. `pnpm install --frozen-lockfile`
3. `pnpm run check`

## Repository evidence

- `package.json` declares `packageManager: pnpm@9.15.4`.
- `package.json` declares the existing quality script `check: pnpm test`.
- `pnpm-lock.yaml` is present with lockfile version `9.0`.
- The installed `node-pnpm` example supplies the Node image, Corepack bootstrap shape, frozen install, and check-step structure.

## Assumptions and gaps

- `linux/arm64` follows the installed pnpm starter example; the CLI parser accepts it, but image availability and runtime behavior were not tested.
- Validation confirms schema and selection only. It does not prove dependency installation, check success, Docker isolation, registry availability, or CI parity.
- No workflows, secrets, deployment, publish, service, matrix, cache, or artifact behavior was selected; none was needed for this manifest-only fixture.
- Client version was unavailable. Skill reading was limited to the installed `.agents/skills/pitstop-config/SKILL.md` and its directly referenced configuration, examples, and workflow references.
- Actual Codex subagent: `luna_worker`, model `gpt-5.6-luna`, reasoning `high`.

## Commands and results

- `pitstop version` — exit `0`, `pitstop 0.1.0`.
- `pitstop -h` — exit `0`.
- `pitstop validate -h` — exit `0`.
- `pitstop validate --repo <fixture> --json` — exit `0`; `valid: true`, selected job `verify`, 1 check, 3 steps, no issues.

## Project command execution

No project commands, dependency installs, Docker commands, or deployed/published actions ran. The only executed binary was the provided Pitstop CLI for version/help and validation.
