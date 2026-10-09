# Pitstop configuration response

Created and validated [.pitstop.json](.pitstop.json) for the requested Yarn Berry check scope.

- Selected job: `verify`
- Runner: `node:22-bookworm`, `linux/arm64`
- Steps: install Yarn 4.6.0 with Corepack, `yarn install --immutable`, then `yarn run check`
- Validation: `valid: true`, one check, three steps, no issues, exit `0`
- Runtime status: blocked pending a real `test` target; the repository's `check` script delegates to `yarn test`, and no `test` script or source target is present.

Evidence is in [evidence.md](evidence.md). Raw CLI output and exit status files are in [raw/](raw/). No project commands, Docker, image pulls, or checks were run.
