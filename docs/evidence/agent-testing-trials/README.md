# Agent testing workflow trials

Recorded 2026-10-28 (Asia/Bangkok). These are small, isolated **real Docker** trials, not proof of arbitrary project compatibility or automatic skill discovery.

## Candidate and method

- Source: `9c3b65b` (`feature/agent-testing-workflow`); embedded bundle **2**. Exact binary and manifest hashes are in [candidate-provenance.json](candidate-provenance.json).
- The CLI still reports the source default version `0.1.0`. This candidate is distinguished by its hash and installed `bundleVersion: "2"`; it is **not** the published v0.1.0 binary or a new release.
- Host: macOS ARM64; Docker 29.8.2, `linux/arm64`; cached `alpine:3.22`. Reports retain the actual image ID/digest.
- Evaluators: fresh `luna_worker` subagents, each explicitly given a skill path and the Thai request “ใช้ Pitstop ทดสอบโปรเจกต์นี้”. Their harness supplied absolute binary/repository paths and temporary HOME/cache/Docker endpoint settings. This tests **direct-read use**, not provider discovery or an unaided request without supplied context.
- Every fixture had a Git HEAD. Its harmless shell assertion ran inside Docker, never on the host. No provider keys, real-home skill installations, deployments, or source/test fixes were needed.
- `prompt.txt`, agent-written command journals, CLI captures and report/log copies are retained below. Journals are agent records, not complete provider tool-event transcripts; trailing blank lines were normalized for Markdown review, while captured command streams were preserved. Absolute temporary paths are preserved as raw evidence; use the adjacent saved files after those temporary directories disappear.

## Agent execution results

| Case | Observed behavior | Result and evidence |
| --- | --- | --- |
| Existing valid config | Kept every original file byte-for-byte; selected custom ID `quality_gate`; omitted configured release-only `publish_artifacts` | Exit 0, assertion passed; [prompt](agents/existing-config/prompt.txt), [journal](agents/existing-config/journal.md), [reports/logs](agents/existing-config/report-files/) |
| Absent config | Read workflow image and `check.sh`; created bounded `verify` config; did not execute workflow YAML | Exit 0, assertion passed; [prompt](agents/absent-config/prompt.txt), [journal](agents/absent-config/journal.md), [config](agents/absent-config/repo/.pitstop.json), [reports/logs](agents/absent-config/report-files/) |
| Broken config | Observed `CONFIG_INVALID` / `JSON_INVALID`; removed only the trailing comma; kept source/tests unchanged | Exit 0 after correction; [original](agents/broken-config/original-config.txt), [prompt](agents/broken-config/prompt.txt), [journal](agents/broken-config/journal.md), [reports/logs](agents/broken-config/report-files/) |
| Genuine failing test | Executed unchanged `quality_gate`; read report, paired logs and step exit; reported expected 99 versus actual 42 without editing the test | CLI exit 1 / command exit 7; [prompt](runtime/test-failure/agent-prompt.txt), [journal with path correction audit](runtime/test-failure/agent-journal.md), [run JSON](runtime/test-failure/agent-run.json), [stderr](runtime/test-failure/agent-quality_gate.stderr) |

The three successful agents recorded opening the returned report, stdout/stderr logs and step evidence before concluding. The failing-test agent additionally reviewed the four adverse runtime cases below read-only and kept their outcomes distinct. This latter review is evidence interpretation, not four additional natural-language execution trials.

### Baseline, before bundle 2

The [bundle-1 baseline](baseline/journal.md) **also passed the existing-config runtime check**. It did not establish a failure to understand the test request. A follow-up [audit response](baseline/audit-response.txt) confirmed that the baseline agent had not separately opened the returned report or check log files before summarizing. The new workflow explicitly requires that inspection; the bundle-2 journals record it.

The baseline [prompt](baseline/prompt.txt), [provenance](baseline/provenance.json) and [raw result](baseline/run.json) are retained. These trials do not replace or rename the older [23 mapping/prerequisite attempts](../config-agent-trials/README.md).

## Runtime cases and report interpretation

The [assert-based replay script](runtime-trials.py) ran all six scenarios against the candidate, with a fresh Git fixture and HOME per scenario. These are actual CLI/Docker results, with no mocked daemon response. The unavailable-Docker case selected a nonexistent Unix socket. Cancellation sent SIGINT after the fixture started sleeping.

| Case | CLI exit | Actual result |
| --- | --- | --- |
| [Pass](runtime/pass/run.json) | 0 | `quality_gate` passed, command exit 0 |
| [Test failure](runtime/test-failure/run.json) | 1 | Failed assertion, command exit 7; complete coverage does not mean passed |
| [Missing prerequisite](runtime/missing-tool/run.json) | 1 | Container ran; missing fixture tool, command exit 127; [stderr](runtime/missing-tool/report-files/quality_gate.stderr) identifies the cause |
| [Docker unavailable](runtime/docker-unavailable/run.json) | 2 | `DOCKER_UNAVAILABLE`; no project step or run report produced |
| [Timeout](runtime/timeout/run.json) | 1 | Job failed with `TIMEOUT`, partial coverage; incomplete step skipped |
| [Cancellation](runtime/cancellation/run.json) | 130 | `CANCELLED`, job skipped, partial coverage |

The [suite summary](runtime/summary.json) records expected/actual exits, warnings, run IDs and the report files inspected. No warnings were returned. A label check found no containers remaining for these six run IDs. A second run of the saved script also passed; its [summary](verification/runtime-replay-summary.json) is kept separately.

To replay, build a fresh candidate, choose a **new** output directory and use a reachable Docker daemon. Requires Python 3, `rtk`, Git, Docker and Linux Alpine image availability:

```sh
PITSTOP_TRIAL="$(mktemp -d)"
rtk proxy env GOTOOLCHAIN=go1.26.5 go build -o "$PITSTOP_TRIAL/pitstop" ./cmd/pitstop
rtk proxy python3 docs/evidence/agent-testing-trials/runtime-trials.py \
  --binary "$PITSTOP_TRIAL/pitstop" --output "$PITSTOP_TRIAL/results"
```

The runner refuses an existing output directory. It creates only fixture data there and leaves evidence available for inspection; it does not run checks against this source repository.

## Bundle and Go verification

- [Race suite](verification/race.log): `rtk proxy env GOTOOLCHAIN=go1.26.5 go test -race -count=1 ./...` — passed.
- [Vet](verification/vet.log): `rtk proxy env GOTOOLCHAIN=go1.26.5 go vet ./...` — exit 0 (empty log).
- [Build](verification/build.log): fresh candidate build — exit 0 (empty log).
- [Offline smoke](verification/offline-smoke-retry.log): init/validate/custom/skill and all bundled examples/mappings passed. Its toolchain runtime section was deliberately skipped; the separate Docker trials above supply runtime evidence only for these Alpine fixtures.
- [Isolated negative replay](verification/released-v1-regression-replay.log): the new upgrade regression test against copied released-v1 assets fails at `bundle version="1", want 2`. This is a later controlled regression replay, not the worker's original RED terminal transcript.
- [Current upgrade test](verification/released-v2-upgrade.log): the same test passes against bundle 2. The full suite also checks refusal of modified/unknown destinations, current manifest hashes/inventory, and exact released manifest SHA-256 `8a903c994e2a571818dba7cb47d0ca7790a8efa3f3b2425e173a3e984e28a8a2`.

The first parent smoke invocation completed its assertions but failed during cleanup because `GOTOOLCHAIN=go1.26.5` downloaded a read-only Go toolchain into the isolated HOME. The [first-attempt excerpt](verification/offline-smoke-first-attempt.txt) retains that failure. The successful retry put the already installed Go 1.26.5 `bin` directory on the process PATH and used `GOTOOLCHAIN=local`; no product code was changed to hide the cleanup failure. The worker's original RED/GREEN output was terminal-only; it was not reconstructed as an original log.

## Limits

These fixtures do not verify every language/preset, dependency installation/download, private registry, image-pull failure, forced cleanup failure, hostile-code isolation, CI parity, native Windows, CLI-missing agent installation, or provider skill discovery. Public v0.1.0 remains unchanged and embeds bundle 1. A separately authorized new binary release is required to distribute bundle 2 through the release installer.
