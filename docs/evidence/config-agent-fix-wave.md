# Config-agent final fix wave

Scope: five Important and two Minor findings from `final-branch-review.md` at `bdeb767`. This report records only the bounded parser, CLI, installer cancellation, human diagnostics, canonical asset/history, regression-test and evidence updates.

## Resolved

- Wrong nested containers return typed `JSON_TYPE` issues at trusted JSON Pointer paths without leaking values.
- pnpm, Yarn Classic and Yarn Berry starters enable Corepack shims before pinned manager installation. Current manifest hashes were refreshed; exact `bdeb767` bundle assets and manifest are retained as trusted force-upgrade history.
- CLI syntax fallback follows Cobra's bool grammar and all known string value flags, while using parsed `--json` state when available.
- Project skill installation checks cancellation immediately after Git destination resolution; deterministic success/failure fake-Git tests cover release and cleanup.
- Human summaries and CLI human errors escape C0/Cf/DEL and bound displayed paths/messages while preserving machine fields.
- Evidence wording distinguishes representative command assertions from execution of every generated step. Direct ID/count and rendered-path boundary tests assert exact codes, zero values and retained paths.

## Evidence

Tests-only RED was observed before production edits for parser path leakage, CLI JSON mode, and both cancellation races. Focused tests are GREEN after implementation; the combined package set passed 145 tests. Fresh Go 1.26.5 unit, race, vet and both 30-second config fuzz targets passed. A fresh exact binary `/tmp/pitstop-final-fix-wave-bin` was built with SHA-256 `34bbdf22d426ede385c603e9d0470f3c3d8d34913db631bb38d8e60e5b3453c3`; offline config-agent smoke and existing Docker smoke passed against it. Focused Docker checks passed the asset-pinned pnpm `9.15.9` frozen install/check and Yarn Berry `4.6.0` immutable install/check in disposable `node:22-bookworm` containers with tiny fixtures.

The pre-change bundle was captured with `git archive bdeb767 -- internal/agentassets/bundle` before edits. The archived and committed predecessor manifest hash is `15df7dbb90c001b3234c490b60929a09bbede4b10e0f791d700657d64b3933b3`; tests verify every predecessor asset hash independently.

## Remaining gate

The fresh Go 1.26.5 floor race/vet/fuzz checks, exact fresh binary/offline/Docker smoke and focused manager checks are complete. Only the parent scoped review remains before Task 8 item 5 can be marked complete. Existing broad evidence is not relabeled as a post-fix rerun. No dependency, engine, remote CI, global install, push or release action was taken.

The ledger's initial omissions remain explicit: the earlier missing-script check had no behavioral RED, and historical broad acceptance evidence was not rerun or relabeled for this fix wave.

Commit: the final fix-wave changeset recorded in the handoff; no push or release.

## User-requested test-gap follow-up

The scoped review's two remaining test-only gaps have passing correction-round checks after the final fix wave. The parser test now covers exact 256/257-byte rendered-path prefixes for ASCII, a three-byte UTF-8 rune and JSON Pointer escaping, with `JSON_UNKNOWN_FIELD`, zero `Config` and valid UTF-8 assertions. The cancellation fixture now releases or aborts the blocked fake Git, waits for `workerDone` before temporary/environment cleanup on every bounded path, restores the no-`.agents` guard, and checks a child-completion marker; successful and failing resolution plus a controlled release-write failure are covered. See [config-agent test-gap evidence](config-agent-test-gaps.md) for commands, results and limits. The coordinator follow-up gate remains pending independent scoped re-review.
