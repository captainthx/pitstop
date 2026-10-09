# Evidence

## Selected scope

One `verify` job with three ordered steps in `node:22-bookworm` on `linux/arm64`:

1. Bootstrap Yarn Classic with `corepack install --global yarn@1.22.22`.
2. Install dependencies with `yarn install --frozen-lockfile`.
3. Run the existing project check with `yarn run check`.

## Evidence

- `package.json` declares `packageManager: yarn@1.22.22`.
- `yarn.lock` is present and begins with `# yarn lockfile v1`.
- `package.json` declares the existing verification script `check: yarn test`.
- The fixture contains no workflow files or additional project manifests.
- The host architecture is `arm64`; the config records `linux/arm64` explicitly.
- The installed skill’s Yarn Classic example specifies the same image, bootstrap, frozen install, and check sequence.

## Assumptions and gaps

- `node:22-bookworm` is selected from the installed skill’s evidence-backed Yarn Classic starter; image availability and runtime compatibility were not tested.
- Corepack and Yarn bootstrap are prerequisites for a runtime check; no project commands were run.
- Validation proves only schema and selection validity. It does not prove dependency installation, image availability, command success, isolation, or CI parity.
- No workflow parity claim is made because the fixture has no workflow files.

## Command list

- `pitstop-phase-a version`
- `pitstop-phase-a -h`
- `pitstop-phase-a validate -h`
- `pitstop-phase-a validate --repo <fixture> --json`

## Project commands

Project commands did not run. The only execution was the provided Pitstop CLI’s read-only configuration validation.

## Agent metadata

- Codex subagent: `luna_worker`
- Model/reasoning: `gpt-5.6-luna/high`
- Client version: unavailable
- Skill use: explicit installed `.agents/skills/pitstop-config/SKILL.md` and its three relative references only
