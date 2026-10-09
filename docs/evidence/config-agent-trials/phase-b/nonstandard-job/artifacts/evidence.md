# Evidence

## Scope

The request is the repository's normal quality check. The only workflow is `.github/workflows/quality.yml`; its job key is `cobalt`, display name `Review gate`, container `node:22-bookworm`, and verification command `npm run check`. The release script was out of scope.

## Evidence and prerequisites/gaps

- `package.json` declares `packageManager: npm@10.9.2`, `check: npm test`, and `release: npm publish`.
- `package-lock.json` is present with lockfile version 3; `npm ci` is the normal dependency prerequisite, but the selected workflow command was preserved exactly.
- The selected image and `linux/arm64` architecture follow the installed nonstandard-names mapping and provide an explicit runner.
- `npm run check` delegates to `npm test`; no `test` script is declared, so runtime execution is blocked by a missing target.
- The validator proves schema and selection validity only; it does not prove image availability, dependency installation, command success, or CI parity.

## Command list

1. `pitstop version` — exit 0 (`pitstop 0.1.0`)
2. `pitstop -h` — exit 0
3. `pitstop validate -h` — exit 0
4. `pitstop validate --repo <repo> --json` — exit 0, `valid: true`, one selected job, one check, one step

Raw output and exits are in `raw-cli.txt`.

## Project execution

No project commands, Docker commands, dependency installation, tests, deployment, or release commands were run. Only configuration validation was run.
