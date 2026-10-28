# Pitstop Go setup trial

## Scope and assumptions

- The checked-in workflow and Makefile define three evidence-backed test commands: root unit tests, root `integration`-tag tests, and the nested `tools` module tests.
- `integration_test.go` imports Testcontainers and starts public image `busybox:1.37`; the user's explicit `--allow-testcontainers` authorization was applied only to this check.
- `external_test.go` is included by the tagged root integration command and skips when `EXTERNAL_SERVICE_URL` is unset. No external URL or secret was supplied.
- No source, tests, Makefile, workflow, installed skill, main checkout, or CLI binary was modified. Initial blocked artifacts remain under `initial-docker-block/`.

## Configuration

`.pitstop.json` selects these checks:

- `unit`: `go test -race -json ./...`
- `integration` (`testcontainers: true`): `go test -race -tags integration -json ./...`
- `tools`: `go -C tools test -race -json ./...`

## Commands and outcomes

| Command | Exit | Outcome |
|---|---:|---|
| `/tmp/pitstop-go-setup-final version` | 0 | `pitstop 0.1.0` |
| `validate --repo <fixture> --json` | 0 | Valid schema; selected `unit`, `integration`, `tools`; 3 checks and 3 steps |
| `list --repo <fixture> --json` | 0 | All three jobs selected and eligible |
| `doctor --repo <fixture> --json` (rerun) | 0 | Repository, config, image, and Docker ready |
| `run --repo <fixture> --job unit --job integration --job tools --allow-testcontainers --json` | 0 | All selected jobs passed; run ID `1793193745404086000` |

Run report: `/Users/thanyatab/Library/Caches/pitstop/runs/1793193745404086000/report.json`.

## Coverage and test-level findings

- `unit`: passed, engine exit 0; `TestUnit` passed.
- `integration`: passed, engine exit 0; `TestServiceRoundTrip` passed against dedicated DinD (`docker:29.8.2-dind`) and `busybox:1.37`; the log records successful HTTP round-trip and test-container termination. `TestUnit` also passed in the tagged command.
- `external-service`: skipped, not passed; `TestExternalService` reported `external service URL is not configured` because `EXTERNAL_SERVICE_URL` was absent.
- `tools`: passed, engine exit 0; nested `TestTool` passed.
- Report selected coverage is `complete` for configured checks, with no workflow-parity claim (`workflowCoverage: not_applicable`).

## Cleanup and evidence

The report and paired logs were read for every selected job: `unit.stdout`/`.stderr`, `integration.stdout`/`.stderr`, and `tools.stdout`/`.stderr`. All step evidence files contain exit `0` and are marked `complete`; report warnings are empty. Docker checks after completion found no containers, networks, or volumes with label `com.pitstop.run=1793193745404086000`, indicating run resources were cleaned up. `trial-progress.log` is empty because no JSON progress lines were emitted.

Evidence files: `trial-validate.json`, `trial-list.json`, updated `trial-doctor.json`, `trial-run.json`, `trial-progress.log`, and this note. The prior Docker-blocked evidence is preserved under `initial-docker-block/`.
