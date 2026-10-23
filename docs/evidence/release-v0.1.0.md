# v0.1.0 publication verification

Date: 2026-10-23, Asia/Bangkok. User explicitly authorized merging main and publishing v0.1.0.

## Integration and private access

- Fast-forwarded main to the reviewed config-agent/release work and pushed without force. Main had no branch protection and no competing remote changes after fetch.
- Repository visibility was discovered to be private. Visibility was retained. No credential/account permissions were changed.
- Added explicit `--github-auth` using existing GitHub CLI authentication. It pins github.com/captainthx/pitstop, disables interactive prompts, resolves latest once, and reuses the existing checksum/staging verification. Public HTTPS behavior is unchanged.
- Three new regression cases failed with Unknown argument before implementation, then passed: authenticated pinned release, authenticated latest pinning, authenticated download failure preserving previous bytes. Authenticated tests refuse all curl usage.
- Independent auth review returned PASS with no actionable finding; reviewer reran installer tests. Authenticated network timeouts follow gh defaults; public curl has explicit timeouts.
- `GOTOOLCHAIN=go1.26.5 PITSTOP_TEST_RELEASE=1 go test -race -count=1 ./...`, floor vet and shell syntax checks passed after the auth addition.

## First remote CI and runner prerequisite

CI run https://github.com/captainthx/pitstop/actions/runs/39941961358 passed both native installer/package jobs and the Go race/vet/config-agent checks, but Docker smoke failed with DOCKER_UNAVAILABLE at image inspect (exit 125). The runner image ubuntu24/20261012.334 ships Docker 28.0.4; its official image-inspect command source has no --platform flag. Pitstop explicitly requires that capability, already documented in both user guides. CI now provisions pinned Docker 29.8.2 using SHA-pinned docker/setup-docker-action v5 with the containerd image store. Production engine platform checks remain unchanged; smoke is not skipped.

Sources: [runner image software](https://github.com/actions/runner-images/blob/ubuntu24/20261012.334/images/ubuntu/Ubuntu2404-Readme.md), [Docker 28.0.4 image inspect source](https://github.com/docker/cli/blob/v28.0.4/cli/command/image/inspect.go), [Docker image inspect requirements](https://docs.docker.com/reference/cli/docker/image/inspect/).

## Final CI and tag

- Final source commit: `d868e15f1168abaff0265c5326c2412432f7678b`, pushed to main without force.
- [CI run 39942468242](https://github.com/captainthx/pitstop/actions/runs/39942468242): **success**. Both installer/package jobs passed on macOS and Ubuntu; Verify Pitstop passed formatting, dependency verification, vet, race tests, offline config-agent smoke, actual Docker smoke and JSON output validation.
- Created and pushed annotated tag `v0.1.0` only after that CI passed. Remote peeled tag and main both resolved to the exact source commit above at tagging time.
- [Release workflow 39942670949](https://github.com/captainthx/pitstop/actions/runs/39942670949) builds the draft and assets from that tag.

## Published release

- [Release v0.1.0](https://github.com/captainthx/pitstop/releases/tag/v0.1.0) was published at `2026-10-23T11:00:05Z` (18:00:05 Asia/Bangkok).
- Final API verification: `isDraft=false`, `isPrerelease=false`, tag `v0.1.0`; explicitly marked latest. Repository remains private.
- [Release workflow 39942670949](https://github.com/captainthx/pitstop/actions/runs/39942670949): **success** for native installer tests, source verification, packaging, artifact transfer and draft creation. The draft was inspected and its native pinned installer tested before publication. Release notes document private access, installation commands and limitations.
- All six uploaded assets were downloaded through authenticated GitHub CLI. The exact inventory and all five manifest SHA-256 entries passed verification; the downloaded installer matched the committed script byte-for-byte.

| Asset | SHA-256 |
| --- | --- |
| checksums.txt | 93d072bbdd8f671d78e958e693b15010fdad0d7c58575cabfa9163942eeae645 |
| install.sh | c802059300fe7bb54bc205bea54110b66fab90a5e913b6d230a11e751b253979 |
| pitstop_darwin_amd64 | 2a698851b89de56bee2e1299aa340553a7ac28356695af497e8eeaffbfd26ee6 |
| pitstop_darwin_arm64 | 1a806ce2863b9ac67c9c72e807071e3a50178930d88336ed2671c21580127290 |
| pitstop_linux_amd64 | b977eb7c722b4ecbeb8dca28a56db286fc8c4a5a474ce8c75a30a3fd135361a1 |
| pitstop_linux_arm64 | cc0a1777c7742a4105cb7ea401ca625ddecd43e3ce8ee43d5cf529bc288ac527 |

## Live release verification

- Authenticated pinned installation from the draft using `--github-auth --version v0.1.0 --bin-dir TEMP/pinned-bin`: passed, before publication.
- After publication, authenticated latest installation using `--github-auth --bin-dir TEMP/latest-bin`: passed, including fresh live tag resolution and both asset downloads.
- The documented bootstrap `gh release download --repo captainthx/pitstop --pattern install.sh --output TEMP/latest-install.sh --clobber` passed against the published latest release; its bytes matched source.
- Both installed native macOS arm64 executables returned the following JSON. Version and commit match the actual peeled release tag:

```json
{"schemaVersion":1,"command":"version","exitCode":0,"result":{"version":"0.1.0","commit":"d868e15f1168abaff0265c5326c2412432f7678b","buildTime":null}}
```

- Ran `scripts/docker-smoke.sh` using the downloaded/installed latest binary: **passed** on Linux arm64 Docker (`alpine:3.22`). This covers dirty/new/deleted source files, ignored-file exclusion, filesystem isolation/reuse, failed-step skipping, timeout, SIGINT, cleanup and unrelated-container preservation.
- Executed the downloaded Linux arm64 release binary inside Alpine with a read-only root, no network, a read-only asset mount and an executable /tmp tmpfs: version JSON matched the same release commit.
- Test installations used temporary custom directories. No global CLI/skill or shell profile was modified. Reports produced by smoke are retained under the ordinary Pitstop cache.

## Remaining limits

The release remains private: anonymous HTTPS download is unavailable; users need repository access and an authenticated GitHub CLI session. No visibility changes were authorized or made. Public curl behavior remains covered by hermetic tests, not a live public release. Developer ID signing/notarization and publisher-signed checksums/provenance remain absent. Native macOS amd64 was not exercised. GitHub's artifact v4 actions emitted Node 20 metadata deprecation warnings while successfully running under Node 24; future action-version maintenance can remove those warnings. Historical local-only limits remain in the original implementation evidence; this document supersedes them only for the checks listed above.
