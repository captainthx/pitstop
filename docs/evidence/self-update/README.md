# Self-update verification

Recorded 2026-10-28 (Asia/Bangkok). Source feature: `feature/self-update`. This verifies the updater implementation, not publication of a new release.

## Real CLI and installer acceptance

The [replay](replay.py) compiles the actual CLI twice in temporary directories. The second executable has a deliberately distinct embedded skill marker and a hypothetical version `1.2.3`. Controlled curl fixtures feed that executable and its checksum to the real embedded installer. No public v1.2.3 release was created or downloaded.

The [summary](summary.json) records executable/manifest hashes and these assertions:

- The updater atomically replaced the executable in a custom directory containing spaces.
- The new executable installed its distinct embedded skill bytes into all four existing Codex/Claude user/project targets. The old process's bundle cannot satisfy these assertions.
- [Structured update output](update.json) remained separate from [installer progress](progress.log).
- A subsequent [automatic latest check](automatic.json), with no `--version`, used real public GitHub release metadata. It preserved the hypothetical newer binary and still refreshed existing skills. This was a live metadata check, **not** a live release binary download.
- [Modified skill](modified.json) and [unknown user file](unknown.json) cases were rejected before publication. File hashes and the executable's inode remained unchanged; partial was false.

Replay from a fresh output directory using a compatible Go toolchain, Git, `rtk` and the source checkout:

```sh
PITSTOP_UPDATE_TRIAL="$(mktemp -d)"
rtk proxy python3 docs/evidence/self-update/replay.py \
  --source "$PWD" --output "$PITSTOP_UPDATE_TRIAL/results"
```

The runner creates no real-home skills or installed executables and refuses an existing output directory. It reads public release metadata; an unavailable metadata endpoint is recorded as blocked rather than fabricated as a pass. It changes only a copied fixture's assets to represent a hypothetical new release; it never changes the source checkout's bundle.

## Tests and review corrections

[Raw focused output](raw-focused.log) covers version parsing, all-four-scope compiled child sourcing, missing targets, fail-closed JSON, changed/unsafe destinations, bounded process-group cancellation, locks and latest-release boundary cases. The initial [RED log](red.log) only records missing parser symbols; it is not claimed as a complete behavioral regression baseline.

A later controlled [negative replay](contradictory-exit-negative-replay.log) runs the contradictory child-exit test against captured pre-fix code: JSON claims a completed install but the process exits nonzero. It fails as intended. The [same regression on corrected code](contradictory-exit-green.log) passes. This replay is separate from the original worker RED run.

Review also corrected stale lock behavior, FIFO lock blocking, executable identity rechecks, malformed child result validation, cancellation of installer descendants, late cancellation precedence and partial/unknown outcome reporting. An installation child that may have published before failing is not reported as an untouched target.

## Full checks

- [Race suite](verification/race.log): `rtk proxy env GOTOOLCHAIN=go1.26.5 go test -race -count=1 ./...` — passed.
- [Vet](verification/vet.log) and [fresh build](verification/build.log) — exit 0.
- [Offline config-agent smoke](verification/offline-smoke.log) — passed. It intentionally does not supply Docker runtime evidence.
- Bilingual command tables/examples, local links/anchors, formatting and diff whitespace checks passed.

The first parent [race attempt](verification/race-first-attempt.log) exposed a pre-existing timing race in `TestExecuteRejectsInvalidCapturedConfigBeforePrepare`: a background shell writer did not always replace the captured file before validation. Separate branch `feature/captured-config-test`, commit `1678567`, replaces that test injection with a synchronous post-capture write. Snapshot behavior is unchanged. The [targeted race test](snapshot-targeted.log) passed 20 repetitions, and the [full suite](snapshot-full-race.log) passed after the correction.

## Limits

Live public binary downloads/replacements, a published update-capable release, native execution on all supported architectures and recovery from power loss are not established by these fixtures. Published v0.1.0 still needs an initial installer/source-build upgrade because it has no `update` command. CLI publication and multiple skill directories are not one atomic transaction; late failure/cancellation reports actual partial or unknown outcomes.
