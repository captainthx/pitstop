# Pitstop Go setup trial

## Scope and assumptions

- The repository's checked-in workflow and Makefile define three test commands: root unit tests, root `integration`-tag tests, and the nested `tools` module tests.
- `integration_test.go` imports Testcontainers and starts `busybox:1.37`; the user's explicit `--allow-testcontainers` authorization was applied to the integration check only.
- `external_test.go` is part of the tagged root integration command and skips when `EXTERNAL_SERVICE_URL` is unset. No external service URL or secret was supplied.
- No source, tests, Makefile, workflow, installed skill, main checkout, or CLI binary was modified.

## Configuration

Updated `.pitstop.json` to select these evidence-backed checks:

- `unit`: `go test -race -json ./...`
- `integration` (`testcontainers: true`): `go test -race -tags integration -json ./...`
- `tools`: `go -C tools test -race -json ./...`

## Commands and outcomes

| Command | Exit | Outcome |
|---|---:|---|
| `/tmp/pitstop-go-setup-final version` | 0 | `pitstop 0.1.0` |
| `/tmp/pitstop-go-setup-final validate --repo <fixture> --json` | 0 | Valid schema; selected `unit`, `integration`, `tools`; 3 checks and 3 steps |
| `/tmp/pitstop-go-setup-final list --repo <fixture> --json` | 0 | All three configured jobs selected and eligible |
| `/tmp/pitstop-go-setup-final doctor --repo <fixture> --json` | 2 | `DOCKER_UNAVAILABLE`; Docker daemon unavailable at resolved endpoint |
| `/tmp/pitstop-go-setup-final run --repo <fixture> --job unit --job integration --job tools --allow-testcontainers --json` | 2 | Blocked before check execution by `DOCKER_UNAVAILABLE` |

## Coverage result

No tests ran because Pitstop could not reach Docker. Therefore unit, tagged integration/Testcontainers, nested-module, and external-service coverage are all **not run**, not passed. The external-service test would remain environment-gated without `EXTERNAL_SERVICE_URL` even after Docker is available.

The run emitted no report path, job logs, or container cleanup work; `trial-progress.log` is empty. This is an engine-readiness block, not a test failure and not a successful coverage claim.

Evidence: `trial-validate.json`, `trial-list.json`, `trial-doctor.json`, `trial-run.json`, and `trial-progress.log`.
