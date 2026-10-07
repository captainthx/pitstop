# Pitstop

**English** | [ไทย](README.th.md)

**Run repository checks in an isolated Docker environment before commit or push.**

Pitstop captures the current Git working tree, copies it into a fresh Linux container for each selected check, and retains logs, step evidence, and a JSON report. It does not read or execute GitHub Actions workflow YAML and does not require `act`.

## Quick start

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
TARGET_REPO="/absolute/path/to/your/repo"
"$PITSTOP_BIN" -h
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --json
```

The host needs Git, Docker, and a reachable Docker daemon. Checks need their own toolchain in the configured image; Node or Go on the host is unnecessary. Start from [examples/docker-react.json](examples/docker-react.json) or [examples/docker-go.json](examples/docker-go.json).

## Configuration

`.pitstop.json` uses schema version 2:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "node:22-bookworm", "architecture": "linux/arm64"},
  "jobs": ["verify"],
  "checks": [{"id":"verify","steps":[
    {"id":"install","run":"npm ci"},
    {"id":"check","run":"npm run check"}
  ]}]
}
```

Steps use `/bin/sh -e` with `/workspace` as the working directory. Steps in one check share files; each check gets a new container. A step can call an existing project script such as `sh scripts/check.sh`, but GitHub `uses:` actions, expressions, services, matrix, cache volumes, secrets, and automatic image builds are not translated. Build a custom image once when the public image lacks a tool:

```dockerfile
FROM node:22-bookworm
```

```sh
docker build -t my-checks:dev -f Dockerfile.checks .
go run ./cmd/pitstop run --repo /absolute/path/to/repo
```

Schema version 1 configurations are rejected with migration guidance. Move commands from workflow `run` steps into `checks`, replace checkout with Pitstop's snapshot, and replace setup actions with a toolchain image. GitHub CI can keep using its workflow and call the same project script.

Reports keep the source identity, resolved image ID, command arguments, logs, step exit records, and cleanup evidence under the user cache. `workflowCoverage` is `not_applicable` because no workflow is interpreted. Timeout and Ctrl+C stop and remove the owned container; the source repository remains unchanged.

See the [user guide](docs/user-guide.en.md), [requirements](docs/requirements.en.md), and historical [validation evidence](docs/evidence/final-validation.md).

For reproducible toolchains, set `runner.image` to an image digest, for example `node@sha256:<digest>`. Tags can move; each run records the resolved image ID. Build custom images manually before running Pitstop.
