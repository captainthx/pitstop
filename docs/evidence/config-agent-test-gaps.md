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

The independent scoped correction re-review approved every finding at `3e77208`; no new finding emerged. The exact path-boundary gap and cancellation-test cleanup gap are closed. Production code is unchanged. The original gate and correction re-review are preserved below.

---

# Scoped test-gap gate — 8dd8dda → b754bb6

Scope: the two user-requested test-only gaps, their evidence/status edits, and whether the cancellation test still guards the production behavior it previously guarded. This is not a new whole-branch review. The diff changes tests and docs only; no production code, dependency, asset, or engine file changed. I used the supplied full diff and current source. I did not rerun the already reported green suites.

## Rulings

| Gap | Ruling | Evidence |
| --- | --- | --- |
| Exact rendered issue path at the byte boundary | **ADDRESSED** | `internal/config/parse_test.go:263-288` checks zero `Config`, typed validation issue, `JSON_UNKNOWN_FIELD`, exact expected nonempty prefixes and valid UTF-8 for six cases: ASCII at 256/257 bytes, a three-byte rune at/across the cap, and JSON Pointer `~`/`/` expansion at/across the cap. Equality to a concrete expected path makes empty, wrong and overlong paths fail. The over-limit multibyte case expects the prefix before the incomplete rune. This is controlled parser evidence, not a wider runtime/security claim. |
| Fake-Git fatal-path release and join | **NOT ADDRESSED in full** | `internal/skill/install_test.go:211-233` defers cancellation, release/abort signaling and a bounded `workerDone` wait, so startup and result timeouts normally run cleanup before `t.TempDir`/`t.Setenv` cleanup. `internal/skill/install_test.go:239-264` exercises success, failure and a simulated release-write failure, and checks the child marker. However, `joined = true` at line 251 occurs upon reading the buffered result, before the actual `workerDone` join at lines 255-260. If that join times out and calls `t.Fatalf`, deferred cleanup skips the join because `joined` is already true. The helper should set `joined` only after `workerDone` closes. A bounded timeout can still report an unresponsive worker; it cannot guarantee no orphan if both release and abort signaling fail or the worker never exits. |

## Findings

- **Important — Cancellation regression loses its no-write guard.** The former `TestInstallCancellationWinsGitResolutionSuccessAndFailure` asserted that `<repo>/.agents` had not been created after cancellation. The refactor removes that assertion (`internal/skill/install_test.go:143-161, 248-265`). `Install` currently checks cancellation immediately after destination resolution (`internal/skill/install.go:94-100`), before it can create a parent directory at line 149. If that check slips behind the directory creation, the successful fake-Git case can still return `CANCELLED` at line 196, pass the new result assertions, and leave `.agents` behind. Restore an assertion that the fresh repo has no `.agents` after the installer has joined, preferably in the helper where `repo` is available. This is test coverage lost in the follow-up, not an observed production regression.
- **Minor — Worker join flag is set early.** `internal/skill/install_test.go:251, 255-260, 222-229` has the precise fatal-path hole described above. Keep `joined` false until `workerDone` is observed; the deferred path can then retry its bounded wait.
- **Minor — Evidence overstates the prior assertion and the new join.** `docs/evidence/config-agent-test-gaps.md:14` says the old parser assertion required a “nonempty” path. It only checked the code, maximum length and valid UTF-8; empty also passed. At line 20, cleanup is said to “join the installer result channel,” while the deferred cleanup waits on `workerDone` and does not drain `results` on timeout. Describe the actual checks. The historical original and scoped reviews correctly retain their old deferred status, and the follow-up section at `docs/evidence/config-agent-review.md:250-256` identifies the supersession rather than rewriting those rulings.
- **Minor — Gate status is premature.** `docs/superpowers/plans/2026-10-08-config-agent.md:13, 243` calls the follow-up gate complete before this independent gate has passed. Update the status after the focused fixes and gate result. `docs/evidence/config-agent-validation.md:96` accurately describes the new parser boundary coverage; remote CI/release/platform/security limits remain explicit.

No Critical finding. The reported controlled Go 1.26.5 race/vet, affected-package, and repeated cancellation runs are plausible evidence for the code paths they executed, but green passing paths do not resolve the source-derived failures above. The test's release-write failure is deliberately simulated before the write; it demonstrates the abort path, not an OS write-error guarantee. The child marker is useful alongside the installer join, not by itself proof that a process has exited.

**Task quality: Needs fixes.** Restore the no-directory assertion, move the join flag to after `workerDone`, correct the two evidence phrases, then mark the follow-up gate complete after a focused passing check. No broad runtime or security rerun is indicated by these test-only corrections.

---

# Scoped correction re-review — b754bb6 → 3e77208

This pass reviews only the correction diff against /tmp/pitstop-test-gaps-scoped-review.md. The parser boundary test was unchanged and remains approved. The correction changes one test helper and evidence/status text; no production, dependency, asset or engine file changed. I relied on the reported focused correction checks rather than repeating green suites.

| Original finding | Verdict | Evidence |
| --- | --- | --- |
| Rendered issue-path boundary gap | **ADDRESSED** | Unchanged from the approved first pass. `internal/config/parse_test.go:263-288` has exact expected paths, code, zero config and UTF-8 checks at/across the limit for ASCII, multibyte and JSON Pointer escaping. |
| Fake-Git fatal-path join gap | **ADDRESSED for bounded cooperative cleanup** | `internal/skill/install_test.go:248-259` now sets `joined` only after `workerDone` closes. If the result or main worker wait times out, `joined` stays false and deferred cleanup at lines 211-233 signals release/abort and retries a bounded worker wait before test fixture/environment cleanup. The helper cannot promise no orphan after both signals fail or an unresponsive worker; `docs/evidence/config-agent-test-gaps.md:32` now says so accurately. |
| Important: lost no-`.agents` cancellation guard | **ADDRESSED** | `internal/skill/install_test.go:260-264` checks that the fresh repository's `.agents` path does not exist and treats other stat errors as failures. It runs after the actual worker join, so the success fixture again detects a cancellation regression that creates directories before returning `CANCELLED`. |
| Minor: early `joined` assignment | **ADDRESSED** | The buffered-result receive at line 250 no longer marks the worker joined; only the `workerDone` receive at line 255 does. |
| Minor: evidence misstated prior path check and join | **ADDRESSED** | `docs/evidence/config-agent-test-gaps.md:14` says the prior assertion allowed an empty path; line 20 names the `workerDone` wait rather than a result-channel join. The correction record at lines 34-49 preserves what changed and reports the bounded checks. |
| Minor: premature gate status | **ADDRESSED** | `docs/superpowers/plans/2026-10-08-config-agent.md:13,243` and the linked evidence now say the coordinator follow-up gate is pending this independent re-review. Historical deferrals remain preserved and identified as superseded for the current checkout. |

No Critical, Important or new Minor finding in this correction diff. Reported correction checks cover the affected 112 package tests, 80 repeated cancellation runs, Go 1.26.5 race/vet and diff hygiene; that is proportionate to these test/evidence changes. The simulated release-write failure exercises the abort branch, and the marker plus worker join gives useful proof for passing controlled fixtures. No native platform, remote CI, release or security guarantee is inferred.

**Task quality: Approved.** Parent bookkeeping may now record the independent follow-up gate as passed.
