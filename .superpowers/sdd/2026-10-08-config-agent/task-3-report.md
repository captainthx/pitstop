# Task 3 report

## RED

Added `TestValidateOfflineAndNoWrites` before implementation. The required focused run failed because `validate` was an unknown command; the existing fallback emitted a `pitstop` error envelope (`rtk go test ./internal/cli -run '^TestValidateOfflineAndNoWrites$' -count=1`, 0 passed, 1 failed).

## GREEN

Implemented the offline `validate` command, `ValidateResult`, shared command result writing/output error sentinel, command-aware JSON fallback, and human validation summaries. Validation uses `ResolveRepo`, `Load`, and `ApplyOverrides`; it reports one bounded issue with no partial config and does not execute configured commands or Docker. Added JSON failure, cancellation, flag-value, and one-envelope tests.

`rtk go test ./internal/config ./internal/model ./internal/cli ./internal/run -count=1`: PASS (58 tests).

The output-failure test uses a writer that always returns an error and confirms exit 2 with the failure reported only on stderr.

## Fuzz

- `rtk go test ./internal/config -run '^$' -fuzz '^FuzzParse$' -fuzztime=30s`: PASS.
- `rtk go test ./internal/config -run '^$' -fuzz '^FuzzConfigRoundTrip$' -fuzztime=30s`: PASS.

## Limitations

Fuzzing was run locally with the required 30-second bounds; it is not a security proof. No Docker, network, shell syntax, configured command, or project script was executed by validation tests.
