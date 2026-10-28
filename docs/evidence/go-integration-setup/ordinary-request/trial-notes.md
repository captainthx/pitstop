# Pitstop trial notes

Repository: `ordinary-request`
CLI: `/tmp/pitstop-go-setup-final` (`pitstop 0.1.0`)

## Commands and exits

| Command | Exit | Evidence |
| --- | ---: | --- |
| `rtk /tmp/pitstop-go-setup-final version` | 0 | `pitstop 0.1.0` |
| `rtk /tmp/pitstop-go-setup-final -h` | 0 | CLI help inspected |
| `rtk /tmp/pitstop-go-setup-final init --repo . --preset go --dry-run --json` | 0 | starter preview only; no write |
| `rtk /tmp/pitstop-go-setup-final validate --repo . --json` | 0 | [trial-validate.json](trial-validate.json) |
| `rtk /tmp/pitstop-go-setup-final list --repo . --json` | 0 | [trial-list.json](trial-list.json) |
| `rtk /tmp/pitstop-go-setup-final doctor --repo . --json` (initial attempt) | 2 | archived at `initial-docker-block/trial-doctor.json` |
| `rtk /tmp/pitstop-go-setup-final run --repo . --job unit,external,tools --json > trial-run.json 2> trial-progress.log` (initial attempt) | 2 | archived at `initial-docker-block/trial-run.json` and `initial-docker-block/trial-progress.log` |
| `rtk /tmp/pitstop-go-setup-final doctor --repo . --json` (Docker available) | 0 | [trial-doctor.json](trial-doctor.json) |
| `rtk /tmp/pitstop-go-setup-final run --repo . --job unit,external,tools --json > trial-run.json 2> trial-progress.log` (Docker available) | 0 | [trial-run.json](trial-run.json), [trial-progress.log](trial-progress.log) |

## Configured coverage

- `unit`: `go test -race -json ./...`; root unit tests only (integration-tagged files excluded by Go build constraints).
- `integration`: `go test -race -tags integration -run '^TestServiceRoundTrip$' -json ./...`; selected because `integration_test.go` uses Testcontainers and `busybox:1.37`. It is marked `testcontainers: true` and requires explicit `--allow-testcontainers`, which was not granted.
- `external`: `go test -race -tags integration -run '^TestExternalService$' -json ./...`; the source skips when `EXTERNAL_SERVICE_URL` is unset and otherwise intentionally fails for this disposable trial. No external URL or secret was supplied.
- `tools`: `go -C tools test -race -json ./...`; nested module test from `tools/go.mod` and `tools/tool_test.go`.

## Actual outcomes and gaps

- Validation accepted schema v2 with four selected jobs and four steps. List confirmed all four selected and eligible.
- The initial doctor/run attempt returned `DOCKER_UNAVAILABLE` and is preserved under `initial-docker-block/`.
- After Docker restarted, doctor passed. The rerun used Docker 29.8.2 on linux/arm64 with the pinned `golang:1.26.5` digest and returned exit 0; report: `/Users/thanyatab/Library/Caches/pitstop/runs/1793193754191626000/report.json`.
- Rerun jobs `unit`, `external`, and `tools` all returned `passed` with engine exit 0. Unit executed `TestUnit` and passed; tools executed `TestTool` and passed. External returned exit 0 but `TestExternalService` was explicitly skipped because `EXTERNAL_SERVICE_URL` was not configured, so external-service behavior is not covered.
- The report selected coverage as `complete`, with no warnings or failure reason. All three evidence directories contain `completed`; paired stderr logs are empty for unit/tools, while external stderr contains dependency-download messages only. No cleanup failure was reported.
- Integration was not run: explicit `--allow-testcontainers` authorization is required by the installed skill and was not present. A Docker/Testcontainers pass is therefore unverified.
- `trial-progress.log` contains the Pitstop step progress; per-job stdout/stderr and step evidence remain at the report path above.
- No host-side project commands or secrets were used; CI files, source files, tests, Makefile, module files, installed skill, and CLI were not modified. Project tests ran only inside Pitstop containers.
