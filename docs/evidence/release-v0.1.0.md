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

Final remote CI, release workflow, publication and live installation results are recorded below after completion.
