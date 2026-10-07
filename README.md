# Pitstop

**English** | [ไทย](README.th.md)

**Run your GitHub Actions checks locally before sending changes to CI.**

Pitstop selects jobs from an existing workflow and runs them through `act` and Docker. It tests an isolated copy of the current working tree, including changes you have not committed yet.

## Start here

| What you need | Documentation |
|---|---|
| Set up, run checks, and understand results | [User guide](docs/user-guide.en.md) |
| Tools, features, MVP requirements, and limitations | [Requirements and scope](docs/requirements.en.md) |
| Actual React, Go, and integration validation | [Validation evidence](docs/evidence/final-validation.md) |

## Quick start

From the Pitstop source directory:

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
TARGET_REPO="/absolute/path/to/your/repo"
```

Explore features, commands, flags, and examples from the CLI:

```sh
"$PITSTOP_BIN" -h
"$PITSTOP_BIN" run -h
```

Help works without configuring a target repository or starting Docker.

Create `.pitstop.json` in the target repository using the [guide's example](docs/user-guide.en.md#2-configure-the-target-project), then run:

```sh
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
"$PITSTOP_BIN" run --repo "$TARGET_REPO"
```

You need Git, a reachable Docker daemon, and **act 0.2.89**. Go **1.26.5 or newer** is required when building Pitstop from source.

## Main features

- Select jobs such as lint, test, or verify from the original workflow and run them sequentially.
- Test current source files, including uncommitted changes.
- Read human output or JSON for scripts and AI agents.
- Retain reports, logs, and source identity for inspection.
- Set a timeout, cancel with Ctrl+C, and prevent overlapping runs for the same repository.
- Opt into the dedicated `recruit100-service` integration adapter.

Runtime validation covers macOS ARM64 with a Linux ARM64 runner. Unsupported conditions or workflow capabilities are reported as `blocked`. Local success still needs the required GitHub checks to pass.

## Pitstop CI

[GitHub repository](https://github.com/captainthx/pitstop) · [GitHub Actions](https://github.com/captainthx/pitstop/actions)

CI runs on pull requests targeting `main`, pushes and merges into `main`, and manual dispatch. It checks formatting, dependencies, vet, race tests, build, and the CLI's JSON smoke test using the Go version in `go.mod`.

These checks validate Pitstop's code. Actual React/Go application workflow runs are recorded separately in the [validation evidence](docs/evidence/final-validation.md).
