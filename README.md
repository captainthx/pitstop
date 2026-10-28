# Pitstop

**English** | [ไทย](README.th.md)

Run a project's lint, test and build commands in fresh Linux containers against the files currently on your machine. Pitstop captures logs and a report so you can review what actually ran before commit or push.

**Quick path:** install the CLI → prepare `.pitstop.json` → validate → check readiness → run.

## What Pitstop handles

You choose the Docker image and the checks. Pitstop snapshots the working tree, creates a fresh container per selected check, runs ordered steps, records evidence, and cleans up. Uncommitted edits, new non-ignored files and deletions are included; project commands run inside the container and do not write back to the original repository.

## 1. Install the CLI

Published binaries support macOS/Linux arm64/amd64. The primary route uses the public release installer; it needs curl and a SHA-256 utility, not Go or GitHub CLI. Review the installer and stop if the download fails:

```sh
set -eu
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
cat "$PITSTOP_INSTALLER"
```

After reviewing the downloaded script, run it:

```sh
sh "$PITSTOP_INSTALLER"
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

The [direct `install.sh` download](https://github.com/captainthx/pitstop/releases/latest/download/install.sh) is the same script. [GitHub CLI installation](docs/install.md#authenticated-github-cli-route) and [source builds](#build-from-source) are optional alternatives. The installer verifies checksum/version before replacement, never uses sudo or edits shell profiles, and requires `--force` to replace an existing binary. Full install, upgrade and trust guidance is in [docs/install.md](docs/install.md).

Use `pitstop update` for subsequent upgrades: it checks the latest stable release and refreshes existing user/current-project skills too. This command is in current source builds, **not** published v0.1.0; that version needs its first installer upgrade. See [update behavior and scope](docs/install.md#อัปเกรด--upgrade).

### Build from source

Use Git and Go **1.26.5** from `go.mod`:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop --help
```

Host Go builds Pitstop. Tools used by the checks belong in the configured Docker image; see the [prerequisites](docs/user-guide.en.md#prerequisites).

## 2. Configure the target repository

Set the target once and preview before writing:

```sh
TARGET_REPO="/absolute/path/to/your/repo"
pitstop init --repo "$TARGET_REPO" --preset base --dry-run --json
```

Review the preview, then remove `--dry-run` to write the starter:

```sh
pitstop init --repo "$TARGET_REPO" --preset base --json
```

`base` is an explicit starter with a failure placeholder. Edit `.pitstop.json` to replace its verify command before validating or running. Named presets cover Node, Go, Python, Java, .NET, Rust, PHP and Ruby; `custom` requires `--image` plus one or more repeated `--command` flags. Existing `.pitstop.json` files are preserved by `init`; use the [Node example](examples/docker-react.json) or [Go example](examples/docker-go.json) when editing manually.

Preset examples still need project prerequisites. For example, the Node example requires `package-lock.json`, a matching `runner.image`, and every referenced `package.json` script. Running checks also requires Git, Docker CLI and a reachable daemon; the target repository needs a Git `HEAD` before `run`. Check the Docker daemon architecture with `docker version --format '{{.Server.Arch}}'` and choose `linux/arm64` or `linux/amd64` accordingly.

## 3. Validate, check readiness and run

`validate` is offline config validation: it does not use Docker, pull images, write cache data or execute project commands. Then use `doctor` for host/Docker/image readiness and `run` for actual checks:

```sh
pitstop validate --repo "$TARGET_REPO" --json
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO" --json
pitstop run --repo "$TARGET_REPO" --json > result.json 2> progress.log
```

`run` uses the configured jobs unless `--job` selects configured IDs; the default timeout is **20 minutes per check**. `--arch` overrides architecture for that invocation. Ctrl+C or SIGTERM cancels the run. With `--json`, the result stays on stdout and progress stays on stderr.

### Command reference

| Command | Purpose |
| --- | --- |
| `pitstop --help`, `pitstop -h` | Show root help |
| `pitstop help [command]`, `<command> --help` | Show command help |
| `pitstop init` | Create a starter config; supports `--preset`, `--image`, repeated `--command`, `--arch`, `--dry-run`, `--json` |
| `pitstop validate` | Validate config; supports `--job`, `--arch`, `--json` |
| `pitstop doctor` | Check repository, config, Git/Docker and image readiness; supports `--json` |
| `pitstop list` | List configured checks and selection; supports `--json` |
| `pitstop run` | Execute selected checks; supports `--job`, `--timeout`, `--arch`, `--allow-testcontainers`, `--json` |
| `pitstop update` | Check latest stable, then update the CLI and existing skills; supports `--version`, `--json` |
| `pitstop version` | Show version/build metadata; supports `--json` |
| `pitstop skill` | Manage installed agent skills |
| `pitstop skill install` | Install the optional config skill; supports `--agent`, `--scope`, `--dry-run`, `--force`, `--json` |
| `pitstop completion bash\|zsh\|fish\|powershell` | Generate shell completion; each supports `--no-descriptions` |

Common flags are `--repo string` (global, default `.`), `--json`, `--job`, `--arch`, `--timeout` (default `20m0s`) and `--help`. See the [full command and flag reference](docs/user-guide.en.md#commands-and-flags) for exact types, defaults and error behavior.

## 4. Optional agent entry point

The portable `pitstop-config` skill is separate from CLI installation. Preview a project install before writing:

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --json
```

The first command previews; repeat without `--dry-run` to install. The current skill reads repository evidence, preserves or drafts `.pitstop.json`, validates and inspects it with `list`/`doctor`, and runs only the requested checks; it does not embed an AI provider, execute workflow YAML, or claim runtime success without evidence. For a request such as “Use Pitstop to test this project”, the agent should inspect the CLI/config first, show the selected commands, run the requested scope without redundant confirmation, then read the final report and per-check logs before summarizing. User scope installs to the agent's user directory; project scope installs under the Git root. `--force` replaces only an owned Pitstop bundle. Published v0.1.0 binaries embed skill bundle v1; this checkout's bundle v3 requires a fresh source build until a future release. See the [AI-agent guide](docs/ai-agent.en.md).

## 5. Read the result

Use `reportPath`, per-check log paths and warnings together. Exit codes are:

| Code | Meaning |
| --- | --- |
| `0` | All selected checks passed with complete result integrity |
| `1` | At least one check failed, including timeout or runtime failure |
| `2` | Invalid config/environment, unknown outcome, or incomplete log/report/cleanup without a failed check |
| `130` | Cancellation; takes precedence |

Supported scope includes prebuilt Docker images, snapshots, ordered shell steps, selection, timeout/cancellation and reports. Workflow YAML execution, matrices, job dependencies, generic workflow service containers, secret injection, cache volumes, automatic image builds, artifact export and native Windows support are outside the current scope. See the [full guide](docs/user-guide.en.md) for reports, troubleshooting and the exact limitations.

## Further reading

- [Full user guide and command reference](docs/user-guide.en.md)
- [AI-agent guide](docs/ai-agent.en.md)
- [Documentation index with requirements, engineering records and evidence](docs/README.md)

Testcontainers suites can opt into a dedicated DinD daemon: see [configuration and requirements](docs/testcontainers.md) for `checks[].testcontainers` and `run --allow-testcontainers`. Privileged execution requires authorization for each invocation.
