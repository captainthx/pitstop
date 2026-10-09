# Config-agent test-gap follow-up

Updated 2026-10-09 (Asia/Bangkok). This records the user-requested follow-up for the two test-only gaps left by the scoped review. It changes tests and evidence only; parser, installer, dependencies, assets and engine code are unchanged.

## Before the follow-up

The existing focused tests were green:

```text
rtk go test ./internal/config -run TestParseIssuePathStopsAtUTF8Boundary -count=1  PASS
rtk go test ./internal/skill -run TestInstallCancellationWinsGitResolutionSuccessAndFailure -count=1  PASS
```

Those runs did not provide a behavioral RED because the production behavior already passed. They exposed coverage gaps: the parser assertion allowed any bounded valid-UTF-8 path, including an empty path, and the cancellation test guarded only its ordinary result path.

## Changes and results

`TestParseIssuePathStopsAtUTF8Boundary` now asserts zero `Config`, `JSON_UNKNOWN_FIELD`, the exact retained path and valid UTF-8 at the 256-byte limit and one byte over it for ASCII, a three-byte UTF-8 rune, and JSON Pointer `~`/`/` escaping. The over-limit cases assert the exact retained prefix, including the rune-boundary backoff.

The fake-Git cancellation fixture now has a bounded helper. Cleanup cancels the context, signals release or an abort gate, waits for the installer worker's `workerDone` channel before temporary-directory/environment cleanup, and checks a child-completion marker. The helper covers successful and failing repository resolution plus a controlled release-write failure; startup and result timeouts use the same deferred bounded cleanup path. A result received from the buffered channel is not treated as a worker join until `workerDone` closes, and the no-`.agents` guard runs after that join.

```text
rtk go test ./internal/config ./internal/skill -count=1  PASS (112 tests)
rtk go test ./internal/skill -run 'TestInstallCancellationWinsGitResolutionSuccessAndFailure|TestInstallCancellationCleanupOnReleaseWriteFailure' -count=20  PASS (80 runs)
rtk go test -race ./internal/skill -run 'TestInstallCancellationWinsGitResolutionSuccessAndFailure|TestInstallCancellationCleanupOnReleaseWriteFailure' -count=5  PASS (20 runs)
rtk env GOTOOLCHAIN=go1.26.5 go test -race ./internal/config ./internal/skill -count=1  PASS
rtk env GOTOOLCHAIN=go1.26.5 go vet ./internal/config ./internal/skill  PASS
rtk gofmt -w internal/config/parse_test.go internal/skill/install_test.go
rtk git diff --check  PASS
```

No full repository, Docker, remote CI, release, platform matrix or fuzz rerun was needed for this tests-only follow-up. Cleanup is bounded and cooperative: if both release/abort signaling fail or a worker remains unresponsive, the test reports the timeout and cannot provide an unconditional no-orphan guarantee. The fake-Git child marker proves the controlled fixture exited before cleanup on passing paths; it does not expand the repository's security or platform support claims.

## Correction round after scoped review

The independent scoped review found that the first helper revision marked `joined` when it received the buffered result and had removed the original no-`.agents` assertion. This correction keeps `joined` false until `workerDone` closes, restores the no-write guard after the actual worker join, and leaves the approved parser cases unchanged. The review's release-write branch remains a controlled pre-write error with an abort fallback, not an operating-system write-error guarantee.

Current correction-round checks:

```text
rtk go test ./internal/config -run TestParseIssuePathStopsAtUTF8Boundary -count=1  PASS (7 cases)
rtk go test ./internal/config ./internal/skill -count=1  PASS (112 tests)
rtk go test ./internal/skill -run 'TestInstallCancellationWinsGitResolutionSuccessAndFailure|TestInstallCancellationCleanupOnReleaseWriteFailure' -count=20  PASS (80 runs)
rtk env GOTOOLCHAIN=go1.26.5 go test -race ./internal/config ./internal/skill -count=1  PASS
rtk env GOTOOLCHAIN=go1.26.5 go vet ./internal/config ./internal/skill  PASS
rtk git diff --check  PASS
```

The implementation checks are green; the coordinator follow-up gate remains pending the parent agent's independent scoped re-review.
