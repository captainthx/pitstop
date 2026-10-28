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
| `rtk /tmp/pitstop-go-setup-final doctor --repo . --json` | 2 | [trial-doctor.json](trial-doctor.json) |
| `rtk /tmp/pitstop-go-setup-final run --repo . --job unit,external,tools --json > trial-run.json 2> trial-progress.log` | 2 | [trial-run.json](trial-run.json), [trial-progress.log](trial-progress.log) |

## Configured coverage

- `unit`: `go test -race -json ./...`; root unit tests only (integration-tagged files excluded by Go build constraints).
- `integration`: `go test -race -tags integration -run '^TestServiceRoundTrip$' -json ./...`; selected because `integration_test.go` uses Testcontainers and `busybox:1.37`. It is marked `testcontainers: true` and requires explicit `--allow-testcontainers`, which was not granted.
- `external`: `go test -race -tags integration -run '^TestExternalService$' -json ./...`; the source skips when `EXTERNAL_SERVICE_URL` is unset and otherwise intentionally fails for this disposable trial. No external URL or secret was supplied.
- `tools`: `go -C tools test -race -json ./...`; nested module test from `tools/go.mod` and `tools/tool_test.go`.

## Actual outcomes and gaps

- Validation accepted schema v2 with four selected jobs and four steps. List confirmed all four selected and eligible.
- Doctor returned `DOCKER_UNAVAILABLE`: Docker daemon unavailable at the resolved endpoint (`exit 1` from the daemon probe).
- The authorized non-Testcontainers run returned `DOCKER_UNAVAILABLE` before starting containers. No project test command executed, no report path or per-job log paths were produced, and `trial-progress.log` is empty. Cleanup was not attempted because no container was created.
- Integration was not run: explicit `--allow-testcontainers` authorization is required by the installed skill and was not present. A Docker/Testcontainers pass is therefore unverified.
- External-service coverage is configured but unverified; its environment-gated behavior was inspected from source only.
- No host-side project commands, Docker socket, secrets, CI files, source files, tests, Makefile, module files, installed skill, or CLI were modified.
