# User guide

## Prerequisites

Install Git and Docker, start a daemon, and verify `git --version` and `docker version`. Build Pitstop with Go 1.26.5 or use a released binary. The host does not need the project's Node, Go, or other check toolchain.

## Configure once per repository

Copy [docker-react.json](../examples/docker-react.json) or [docker-go.json](../examples/docker-go.json) to `.pitstop.json`, then set the image, architecture, selected jobs, and ordered steps. The image should contain every command used by the steps. A step can call a project-owned shared script such as `sh scripts/check.sh`; the same script can be called by GitHub CI after its normal checkout/setup.

Schema version 1 configurations are not converted. Remove workflow/event/adapter/runner label fields, set `schemaVersion` to 2, choose an image, and move each intended workflow `run` command into a check step. `actions/checkout` becomes the Pitstop source snapshot and setup actions become the image contents. `uses:`, expressions, services, matrix, needs, secrets, and cache volumes need an explicit future design.

## Commands

```sh
pitstop doctor --repo /absolute/path/to/repo
pitstop list --repo /absolute/path/to/repo
pitstop run --repo /absolute/path/to/repo --job verify --timeout 20m
pitstop run --repo /absolute/path/to/repo --json > result.json 2> progress.log
```

`doctor` checks Git, config, and Docker without pulling or starting a container. `list` reads `.pitstop.json` only. `run` prepares the configured image, copies the working-tree snapshot into a fresh container per selected check, and removes owned containers after evidence is collected. `--arch` overrides the configured Linux architecture for one run; `--force-integration` is removed and is an unknown flag.

Steps run with `/bin/sh -e` in `/workspace`. Their files persist within one check. The original repository is never used as a bind mount and remains unchanged when a step writes files. The image is inspected and pulled when missing or on the wrong platform; preparation has a two-minute budget. Each check has a default twenty-minute timeout. A timeout or Ctrl+C stops the active container, waits for Docker clients and streams, and performs bounded cleanup.

The JSON envelope remains schema version 1. `result.engine.name` is `docker`, `result.engine.imageId` records the immutable image, and `result.workflowCoverage` is `not_applicable`. Step exit records and terminal Docker state are required for `passed`; success-looking output alone is never evidence. Reports and logs are retained under the user cache; temporary snapshots and scripts are removed after the run.

To use a custom image, build it once and reference its tag:

```dockerfile
FROM node:22-bookworm
```

```sh
docker build -t my-checks:dev -f Dockerfile.checks .
pitstop doctor --repo /absolute/path/to/repo
pitstop run --repo /absolute/path/to/repo
```

The first release does not provide services, cache volumes, automatic image builds, or workflow-to-check conversion. See [requirements](requirements.en.md) for the complete boundary and [historical evidence](evidence/final-validation.md) for the former workflow backend.

For reproducible toolchains, set `runner.image` to an image digest, for example `node@sha256:<digest>`. Tags can move; each run records the resolved image ID. Build custom images manually before running Pitstop.
