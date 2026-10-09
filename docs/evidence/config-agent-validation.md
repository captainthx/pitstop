# Config-agent validation evidence

Updated 2026-10-09 (Asia/Bangkok). This is local evidence for the Task 8 acceptance work on `feature/config-agent-plan`. No merge, push, release, real user-home install, or remote CI result is claimed.

## Environment and commands

The final smoke used a fresh absolute binary and isolated temporary resources:

```text
GOTOOLCHAIN=go1.26.5
go version go1.26.5 darwin/arm64
Docker Server 29.4.0, linux/arm64 (aarch64)
PITSTOP_BIN=/tmp/pitstop-task8-bin
```

The binary was built with:

```sh
rtk proxy env GOTOOLCHAIN=go1.26.5 go build -trimpath -o /tmp/pitstop-task8-bin ./cmd/pitstop
```

The offline acceptance command was:

```sh
rtk proxy env PITSTOP_BIN=/tmp/pitstop-task8-bin PITSTOP_CONFIG_AGENT_RUNTIME=0 sh scripts/config-agent-smoke.sh
```

It passed init dry-run and write, existing-file preservation, duplicate rejection, custom init, project skill dry-run/install/no-force collision/owned force replacement, and canonical validation of every embedded example and mapping. Every CLI response was parsed by the repository's `scripts/validate-json.go`, then checked for command, exit code, and relevant result fields.

The runtime command was:

```sh
rtk proxy env PITSTOP_BIN=/tmp/pitstop-task8-bin sh scripts/config-agent-smoke.sh
```

It selected and ran these temporary Git fixtures with actual manifests and lockfiles:

| Toolchain | Image | Result |
| --- | --- | --- |
| Node | `node:22-bookworm` | passed `npm ci` and `npm run check`; image ID `sha256:80503c2cadca910dfe16f1516025a2133cdfb0e839f02e79191f83548f2ec729` |
| Go | `golang:1.26.5` | passed vet, test, and build; image ID `sha256:04b188c2db8449ea15c09355dfab9ce9ed4ecf9414e94674104e316bda7d1996` |
| Python | `python:3.13-slim-bookworm` | passed pinned `pytest==8.3.5` install and pytest; image ID `sha256:55432fc4ef6d07e33c10449312f0ec00071bbb310bb9bd7ba626a31fbbc2cb0d` |
| Rust | `rust:1.99.0-bookworm` | passed locked Cargo test and build; image ID `sha256:e393e07b3c3c39f238de676b0eb60c03dc7aefe5d6e37be6d29c686bf4d7e256` |

The fixtures, temporary HOME/cache, reports, and validator binary were removed by the smoke trap. Docker cleanup remained ownership-scoped to Pitstop runs; no image or unrelated container cleanup was performed.

## Coverage and limits

- The first new-smoke check was intentionally run before implementation and failed because `scripts/config-agent-smoke.sh` was missing. The implemented smoke then passed; this is the genuine RED/GREEN record.
- A first runtime attempt exposed Go VCS stamping in the source snapshot. The fixture now uses `go build -buildvcs=false ./...`, which reflects the snapshot execution boundary; the failure is retained here as a fixture correction rather than a passing claim.
- The CI workflow runs the offline smoke and all embedded parser checks using its freshly built binary. Local evidence does not establish remote CI success.
- The runtime sample covers Node, Go, Python, and Rust only. It does not claim that every embedded language, package manager, image, workflow mapping, or host OS was executed.
- `quickvalidate` was unavailable in the default and bundled Python environments because PyYAML was unavailable; front matter, links, and referenced hashes were checked with available local tooling and the limitation is retained honestly.
- The [config-agent trial index](config-agent-trials/README.md) records 14 independent cases and 23 evaluator attempts. Those trials establish mapping and prerequisite reporting, not runtime or Docker coverage.

The acceptance boundary remains the canonical parser and `config.Load` consumers; no smoke path bypasses them. Parser byte/depth, name, command, job/check/step, and issue limits are enforced by the existing unit and fuzz suites, including boundary and over-limit cases. Diagnostics expose structured issue codes and JSON paths rather than raw config values. Dry-run paths assert no destination is created. Skill replacement accepts only trusted current or bundled historical manifests; arbitrary destination-declared ownership cannot authorize deletion. Init and installer failures preserve recoverable destinations, and post-publish cancellation reports a completed installation with a warning. Runtime reports use `selectedCoverage=complete` and `workflowCoverage=not_applicable` only for the selected fixture checks.

## Acceptance status

- [x] Offline config-agent smoke and embedded example/mapping parsing.
- [x] Representative Node and Go runtime fixtures.
- [x] Explicit Python and Rust runtime fixtures with actual manifests/locks.
- [x] CI integration beside existing formatting, dependency, race, vet, Docker, and JSON checks.
- [ ] Remote CI execution, release publication, and whole-branch review: intentionally unverified here and owned by the parent review.
