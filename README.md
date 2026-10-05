# Pitstop

Pitstop runs a configured subset of an existing GitHub Actions workflow against a local working-tree snapshot through `act` and Docker.

Build it with Go 1.26.5 or newer, then run `pitstop doctor`, `pitstop list`, and `pitstop run --json`. The repository must contain a strict `.pitstop.json`; copy one of the profiles in `examples/`. Pitstop requires Git, act 0.2.89, and a reachable Docker daemon. It never installs tools or reads cloud credentials.

`run` accepts `--job ID` repeatedly, `--timeout 20m`, `--arch linux/arm64|linux/amd64`, and the guarded `--force-integration` adapter. Exit codes are 0 for a complete successful selection, 1 for a launched failure or timeout, 2 for invalid or blocked work, and 130 for cancellation. With `--json`, progress goes to stderr and stdout contains one envelope. Unknown values are represented by JSON `null`; arrays remain `[]`.

Snapshots are stored under the user cache directory at `pitstop/runs`, with 0700 run directories and 0600 reports/logs. Finished runs are retained for evidence and may be removed manually. Source snapshots use the working tree, including nonignored untracked files, and report excluded conventional local files such as `.env`.

The generic runner supports independent, unconditional, nonmatrix jobs on the configured runner. Dependencies, job conditions, matrices, reusable workflows, custom containers/services, secret references, and deploy-like work are blocked. The service adapter only removes the guarded integration job's `needs` and `if` after verifying its expected shape; it reports the workflow fidelity adjustment.

See `docs/evidence/` for measured runtime evidence and limitations.
