# Pitstop

**English** | [ไทย](README.th.md)

Run your project's lint, test and build commands in Linux containers before commit or push. Test the files currently on your machine and retain logs and a report for review.

**Start here:** install the CLI → use `pitstop init --dry-run --json` or copy an example to the target repo → inspect with `pitstop validate --json` → `pitstop doctor` → `pitstop run`.

**Configuration tooling status (2026-10-09):** the current checkout includes `init`, `validate`, and `skill install`, plus starters for Node, Go, Python, Java, .NET, Rust, PHP, and Ruby. The portable `pitstop-config` skill guides evidence-based AI configuration for Codex and Claude Code. The 23-attempt mapping/prerequisite trial record is in the [config-agent evidence index](docs/evidence/config-agent-trials/README.md); selected Node/Go/Python/Rust runtime and focused pnpm/Yarn Berry checks are recorded in [local validation evidence](docs/evidence/config-agent-validation.md). All identified review findings are closed in the [current review status](docs/evidence/config-agent-review.md), including the [test-gap follow-up](docs/evidence/config-agent-test-gaps.md). Remote CI and workflow parity are not established by these local checks. See the [current guide](docs/user-guide.en.md), [agent guide](docs/ai-agent.en.md), and [development roadmap](docs/roadmap.md).

## What Pitstop handles

You choose the **environment** through a Docker image and the **checks** through configuration. Pitstop handles the rest:

```text
working tree → snapshot → fresh container per check → ordered steps → report → cleanup
```

- Include uncommitted edits, new non-ignored files and deletions.
- Run project tools inside containers; generated files do not change the original repository.
- Share files between steps in one check and isolate each check's filesystem.
- Manage timeouts, Ctrl+C, logs and JSON reports with step evidence.

The CLI executes commands defined in `.pitstop.json`. See the [supported scope](docs/user-guide.en.md#supported-scope).

## 1. Install the CLI

Install a published [GitHub Release](https://github.com/captainthx/pitstop/releases) on macOS/Linux arm64/amd64 without Go. The installer selects the latest stable release; source builds remain available below.

Installation options:

- [Install with curl](#install-with-curl) — copy the commands below; no Go or GitHub CLI required.
- [Download the latest installer (install.sh)](https://github.com/captainthx/pitstop/releases/latest/download/install.sh) — no Go or GitHub CLI required; review the script and run it with `sh`.
- [Install with GitHub CLI](docs/install.md#authenticated-github-cli-route).
- [Build from source](#build-from-source).

### Install with curl

Copy these commands into your terminal to download the installer with curl. Review the script before running it; stop if downloading fails:

```sh
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
cat "$PITSTOP_INSTALLER"
```

```sh
sh "$PITSTOP_INSTALLER"
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

GitHub CLI remains an optional alternative; see the [installation guide](docs/install.md#authenticated-github-cli-route).

The installer checks the binary checksum and version before replacement. Existing files require `--force`; no sudo or shell profile edits. See [installation, upgrades, removal and release trust limits](docs/install.md). Git and Docker are still required for checks. Checksums do not protect against a compromised release publisher; macOS binaries are not notarized.

### Build from source

From this Pitstop source checkout, use Git and Go **1.26.5** as specified in `go.mod`. Docker CLI and a reachable daemon are needed to run checks:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop -h
```

Add `export PATH="$HOME/.local/bin:$PATH"` to your shell configuration to retain it in new terminals. Source builds remain available before release publication and as an alternative to prebuilt binaries.

Host Go builds Pitstop. The project's Node, Java, Go or other check tools belong in the Docker image. See [prerequisites](docs/user-guide.en.md#prerequisites).

## 2. Configure each repository once

Use `pitstop init` for a starter, or let the installed skill draft a config from repository evidence. `init` never overwrites an existing `.pitstop.json`; use `--dry-run --json` to inspect a starter before writing it.

Run from the Pitstop checkout and replace the target path:

```sh
PITSTOP_SOURCE="$PWD"
TARGET_REPO="/absolute/path/to/your/repo"
if [ -e "$TARGET_REPO/.pitstop.json" ] || [ -L "$TARGET_REPO/.pitstop.json" ]; then
  echo "preserving existing .pitstop.json; edit it deliberately"
else
  cp "$PITSTOP_SOURCE/examples/docker-react.json" "$TARGET_REPO/.pitstop.json"
fi
```

If a config already exists, preserve it and edit it deliberately. A base preview is:

```sh
pitstop init --repo "$TARGET_REPO" --preset base --dry-run --json
```

Choose a named preset with `--preset`, or use `custom` with an explicit `--image` and one or more repeated `--command` flags. The available presets are listed by `pitstop init -h`. For a manual path, copy the [Node example](examples/docker-react.json) or [Go example](examples/docker-go.json) and edit it.

The Node example is:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "node:22-bookworm", "architecture": "linux/arm64"},
  "jobs": ["verify"],
  "checks": [{"id": "verify", "steps": [
    {"id": "install", "name": "Install dependencies", "run": "npm ci"},
    {"id": "check", "name": "Run project checks", "run": "npm run check"}
  ]}]
}
```

| Field | What to choose |
| --- | --- |
| `runner.image` | An image containing the required tools, such as Node for `npm` |
| `runner.architecture` | `linux/arm64` or `linux/amd64` for the intended environment |
| `checks` | Groups of commands; each group gets a fresh container |
| `checks[].steps` | Commands executed in order within a group |
| `jobs` | Default selected check IDs and their execution order |

The Node example requires `package-lock.json` and a `check` script in `package.json`. Change `run` to your project's command, such as `npm test` or `sh scripts/check.sh`. Examples do not create project scripts.

Check daemon architecture with `docker version --format '{{.Server.Arch}}'`: `arm64`/`aarch64` maps to `linux/arm64`; `amd64`/`x86_64` maps to `linux/amd64`. The target repo needs at least one commit before `run`. Commit configuration and shared scripts so the team uses the same definitions.

## 3. Validate, check readiness and run

`validate` checks the config file and selected jobs without Docker, image pulls, cache writes, or project commands. It returns one JSON envelope with `valid`, selected jobs, architecture, counts, and bounded issues.

```sh
pitstop validate --repo "$TARGET_REPO" --json
```

`doctor` checks Git, Docker and image readiness; `run` executes selected checks in containers. Validation does not certify that commands, images, dependencies, or CI workflows will succeed.

```sh
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO"
pitstop run --repo "$TARGET_REPO"
```

`doctor` probes config, Docker and image readiness without pulling or creating containers. `run` pulls an image when needed, then runs checks. A missing image may be reported as ready with a message that it will be pulled during run; doctor does not verify that every project command exists in the image.

Install the portable skill into an isolated user or project location:

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
```

User scope uses `~/.agents/skills/pitstop-config` for Codex or `~/.claude/skills/pitstop-config` for Claude. Project scope uses the matching hidden directory under the Git root. `--force` replaces only an owned Pitstop bundle authenticated by a trusted manifest.

Common options:

```sh
pitstop run --repo "$TARGET_REPO" --job verify
pitstop run --repo "$TARGET_REPO" --timeout 10m
pitstop run --repo "$TARGET_REPO" --json > result.json 2> progress.log
pitstop run -h
```

`--job` can select IDs already included in `jobs`. The default timeout is **20 minutes per check**. Ctrl+C cancels the run. Pitstop attempts to stop and remove its containers; inspect report warnings if cleanup fails.

## 4. Read the result

Once checks run, the result includes a `reportPath` and per-check log paths. Reports remain in the user cache; temporary snapshots and scripts are removed. A config or environment error before execution may return an error without a run report.

| Exit code | Meaning |
| --- | --- |
| `0` | All selected checks passed without a result-integrity problem |
| `1` | At least one failed check, including timeout or container runtime failure |
| `2` | Invalid config, unavailable environment, unknown outcome, or log/report/cleanup problem without a failed check |
| `130` | Cancellation; takes precedence over other outcomes |

See [reports and troubleshooting](docs/user-guide.en.md#reports-and-logs) to interpret coverage and warnings.

## Further reading

- [Full guide: config, shared scripts, custom images, commands, reports and troubleshooting](docs/user-guide.en.md)
- [Requirements and scope](docs/requirements.en.md)
- [AI-agent configuration](docs/ai-agent.en.md)
- [Config and agent design](docs/superpowers/specs/2026-10-08-config-agent-design.md), [implementation plan](docs/superpowers/plans/2026-10-08-config-agent.md) and [deferred development scope](docs/roadmap.md)
- [Documentation index](docs/README.md) and [current Docker validation](docs/evidence/docker-checks-validation.md)
