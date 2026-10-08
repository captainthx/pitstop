# Task 1 report

## Changes

- Added `config.Parse` with a 1 MiB bound, UTF-8/BOM checks, escaped-surrogate validation, strict JSON token preflight, duplicate decoded-key rejection, exact field casing, depth bound, v1 migration handling, strict typed decode, and zero-value failure returns.
- Changed `config.Load` to use `Parse` after a confined `os.OpenRoot` descriptor read with `O_RDONLY|O_NOFOLLOW|O_NONBLOCK`, regular-file verification, and a limit-plus-one read.
- Added `model.MaxConfigBytes` (1048576) and `model.MaxJSONDepth` (16).
- Added parser, Unicode, size/depth, run-preservation, and special-file regression tests.

## Tests

- RED: the first `rtk go test ./internal/config -count=1` run failed to build because `Parse` was undefined in the new tests.
- Behavioral RED after declaring a `Parse` stub was not run; this is a TDD evidence gap.
- GREEN: `rtk go test ./internal/config ./internal/model -count=1` passed (16 tests).
- GREEN: `rtk go test ./... -count=1` passed (39 tests across 9 packages).
- `rtk git diff --check` passed.

## Concerns/deviations

- The parser reports fixed `CONFIG_INVALID` messages without structured issue paths; structured diagnostics are assigned to Task 2.
- Unix special-file coverage is skipped on Windows, consistent with the design's existing Unix support promise.

## Commit

Task commit: `feat(config): add strict confined parser` (hash reported by the agent after final amend).
