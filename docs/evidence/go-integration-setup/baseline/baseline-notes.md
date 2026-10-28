# Pitstop baseline setup

Scope: prepare the repository's Pitstop configuration for the existing test coverage; do not execute project tests, run Docker containers, start a privileged daemon, update Pitstop, or modify source, tests, Makefile, workflows, or module files.

## Evidence reviewed

- `go.mod` declares Go `1.26.5` and the testcontainers dependency.
- `tools/go.mod` declares the nested tools module.
- `Makefile` defines `test`, `test-integration`, and `test-tools` with the same commands as the workflow.
- `.github/workflows/test.yml` defines `unit`, `integration`, and `tools` jobs.
- `unit_test.go` is the default unit test; `integration_test.go` and `external_test.go` use the `integration` build tag. The integration test starts a Testcontainers BusyBox service; `external_test.go` skips without `EXTERNAL_SERVICE_URL` and otherwise intentionally remains outside this disposable trial.

## Configuration

`.pitstop.json` now selects all three evidence-backed checks using `golang:1.26.5` on `linux/arm64`:

- `unit`: `go test -race -json ./...`
- `integration`: `go test -race -tags integration -json ./...`
- `tools`: `go -C tools test -race -json ./...`

No deploy or publish behavior was selected. The one-image v2 limitation means this setup does not claim CI parity for external services, environment-dependent external tests, or any workflow behavior outside these commands.

## Readiness evidence

- `baseline-validation.json`: Pitstop `validate --json` exited `0`; config valid, 3 selected jobs, 3 steps, no issues.
- `baseline-list.json`: Pitstop `list --json` exited `0`; `unit`, `integration`, and `tools` are selected and eligible.
- `baseline-doctor.json`: Pitstop `doctor --json` exited `0`; repository, config, image, and Docker were reported ready.

These are configuration/readiness results only, not test passes. No `pitstop run`, `go test`, Docker run, image pull, or update was performed. The runnable command when execution is authorized is:

`/tmp/pitstop-testcontainers-final run --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-go-setup-oghrgou0/baseline --json`
