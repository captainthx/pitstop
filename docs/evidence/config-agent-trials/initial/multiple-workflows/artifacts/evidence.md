# Evidence

## Scope

Selected the ordinary test workflow job test from .github/workflows/test.yml.
The selected check is npm test, represented as job/check/step ID test.
The runner is node:22-bookworm on explicit architecture linux/arm64.

The release workflow .github/workflows/release.yml job publish and its
npm publish step are omitted because the request is limited to ordinary tests.

## Evidence

- .github/workflows/test.yml: test job, ubuntu-24.04, npm test.
- .github/workflows/release.yml: publish job, tag-triggered, npm publish.
- No manifest, lockfile, package metadata, or existing .pitstop.json is present
  in the fixture.
- Installed CLI reports pitstop 0.1.0.
- validate --repo <fixture> --json returned valid: true, selected job test,
  architecture linux/arm64, one check, one step, and no issues.

## Assumptions and gaps

- node:22-bookworm is selected from the installed skill's multi-workflow
  mapping and the npm test workflow command; dependency installation cannot be
  specified from fixture evidence because no package manifest or lockfile exists.
- linux/arm64 is an explicit Pitstop runner choice. The workflow's
  ubuntu-24.04 label does not establish CI architecture parity.
- Validation proves config shape and selection only. It does not pull the image,
  install dependencies, run npm test, or prove CI parity.
- Client version metadata is unavailable; only the installed skill was read.

## Command list

See commands.txt. Every shell command was prefixed with rtk.

## Project execution

No project test or Docker command was run. The only project-targeted operation
was the requested configuration validation.

