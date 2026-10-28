# Config reference

Pitstop configs use `schemaVersion: 2`, one Linux runner image, selected `jobs`, and ordered shell `steps`. The image must be an explicit supported tag or digest; parsing does not prove registry availability. Commands are executable shell text: show the selected commands and run them only when the user's request authorizes execution.

Use the starter matching project evidence, then replace its commands with the project's real lockfile and test scripts. `base` is an intentional starter: its configure step exits 2 until a project check is defined. `custom` is supplied through init flags and has no placeholder template.

## Shared prerequisites

Inspect these prerequisites without executing project commands while configuring.

| Preset | Evidence and prerequisite |
| --- | --- |
| npm | A lockfile and a real `check` script; use `npm ci` before `npm run check`. |
| pnpm | A pinned `packageManager`, Corepack bootstrap, and lockfile; use `pnpm install --frozen-lockfile`. |
| Yarn Classic | A pinned Yarn 1 `packageManager`, Corepack bootstrap, and lockfile; use `--frozen-lockfile`. |
| Yarn Berry | A pinned Yarn 4 `packageManager`, Corepack bootstrap, and lockfile; use `--immutable`. |
| Go | The module's selected Go toolchain and real vet, test, and build targets. |
| Python pip | A requirements or project dependency file declaring `pytest` and any lint dependencies; pin versions or hashes when reproducibility is required. |
| Python uv | A locked `uv.lock`, project test dependencies, and `uv sync --locked` before `uv run`. |
| Maven / Gradle | A checked-in `mvnw` or `gradlew` wrapper and project test task. |
| .NET | A reviewed solution or project file; restore before `dotnet test --no-restore`. |
| Rust | A lockfile and Cargo test/build targets; use `--locked`. |
| Composer | Composer scripts plus required PHP extensions; use the lockfile and `composer run-script test`. |
| Bundler | A lockfile and reviewed Rake task; use `bundle exec rake`. |

The runner architecture is explicit: `linux/arm64` or `linux/amd64`. A starter is parser-valid; it is not evidence that its image, dependencies, or checks were executed.

Trace selected commands one level past their wrapper. For npm, pnpm, and Yarn, inspect the referenced `test` or check script and report a missing script or test target as runtime-blocked. For Rust, inspect `Cargo.toml`, source targets, and the lockfile; `cargo test --locked` is not environment-ready when no source or declared target exists. Read a delegated shared script before selecting it. Never invent a project script or target to make a config appear ready.

## CLI paths

Inspect help from the installed binary before relying on a flag. The current commands are:

```sh
pitstop init --repo <path> [--preset <name>] [--image <ref>] [--command <shell-code> ...] [--arch linux/arm64|linux/amd64] [--dry-run] [--json]
pitstop validate --repo <path> [--job <id> ...] [--arch linux/arm64|linux/amd64] [--json]
pitstop list --repo <path> [--json]
pitstop doctor --repo <path> [--json]
pitstop run --repo <path> [--job <id> ...] [--timeout <duration>] [--arch linux/arm64|linux/amd64] [--json]
pitstop skill install --agent codex|claude --scope user|project [--repo <path>] [--dry-run] [--force] [--json]
```

If `pitstop` is missing, use the public installer only for the current local request: download it to a temporary file, inspect it, and install into a fresh temporary directory so an existing binary and profile remain untouched. Use the resulting absolute path for every command; do not pass `--force` or install into a real home directory.

```sh
set -eu
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 \
  --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
cat "$PITSTOP_INSTALLER"  # inspect before executing
```

After inspecting the downloaded script, run it in a separate shell block:

```sh
set -eu
PITSTOP_BIN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pitstop-bin.XXXXXXXX")"
sh "$PITSTOP_INSTALLER" --bin-dir "$PITSTOP_BIN_DIR"
PITSTOP_BIN="$PITSTOP_BIN_DIR/pitstop"
"$PITSTOP_BIN" version
```

`init` defaults to `base`; supported presets are `base`, `node-npm`, `node-pnpm`, `node-yarn-classic`, `node-yarn-berry`, `go`, `python-pip`, `python-uv`, `java-maven`, `java-gradle`, `dotnet`, `rust`, `php-composer`, `ruby-bundler`, and `custom`. Repeated `--command` values replace all `verify` steps in order. `custom` requires `--image` and at least one `--command`. `--dry-run` writes nothing, including when `.pitstop.json` already exists. Init never overwrites an existing destination; a failure after exclusive creation can leave a partial file for inspection, and a retry reports `CONFIG_EXISTS` until it is handled manually. An architecture omitted from `--arch` defaults from the host runtime; it does not detect a remote Docker daemon.

Successful init JSON is one envelope with `command: "init"`, `exitCode: 0`, and a result containing `repo`, absolute `configPath`, `preset`, `architecture`, `config`, `dryRun`, `written`, `destinationExists`, and `warnings`:

```sh
pitstop init --repo /work/app --preset base --dry-run --json
```

The base result includes `written: false`, `destinationExists: false`, and warnings for the host-architecture default and the explicit exit-2 placeholder. Init uses exit `0` for success and `2` for invalid input, an existing destination, or a write failure; cancellation before persistence is `130`. A post-publish install cancellation is reported as a completed install with a warning by the installer contract.

`validate` reads and strictly validates `.pitstop.json` without Docker, image pulls, cache writes, shell parsing, or project commands. A valid result has `valid: true`, selected jobs, architecture, counts for all definitions, and `issues: []`. An invalid config returns exit `2`, `valid: false`, one bounded issue with `code`, `path`, and `message`, and error code `CONFIG_INVALID`. Use the result to repair config syntax/shape and selection; do not call it a runtime or CI result.

```sh
pitstop validate --repo /work/app --json
```

`list` reads configured check IDs and marks which are selected; use its JSON output instead of inventing a job ID. `doctor` checks repository/config/Docker/image readiness and returns `ready: false` with reason codes when a prerequisite is blocked. Neither command executes project steps. `run` selects configured IDs, defaults to all configured jobs, applies a 20-minute timeout per check, and writes progress to stderr when `--json` is used. Its stdout is one final JSON envelope containing the report, job statuses, reason codes, log paths, report path, coverage, and warnings. Read that report and the referenced logs before summarizing. A nonzero test command is a failed job; timeout, cancellation, unknown engine evidence, or incomplete cleanup is not a pass. Use exit `1` for a failed selected check, `2` for an untrustworthy/incomplete run or setup error, and `130` for cancellation.

```sh
pitstop list --repo /work/app --json
pitstop doctor --repo /work/app --json
pitstop run --repo /work/app --job verify --json >run.json 2>run.progress.log
```

`skill install` copies the complete bundle from embedded assets. Codex user scope is `~/.agents/skills/pitstop-config`; Claude user scope is `~/.claude/skills/pitstop-config`; project scope uses the matching hidden directory under the Git root. User scope does not accept `--repo`. `--dry-run` reports the planned files and action without creating directories or locks. Existing installs require `--force`, and force replaces only a bundle authenticated by the current or explicitly trusted historical Pitstop manifest. Unknown historical content requires manual migration; a destination-provided owner or hash declaration is not proof of ownership.

```sh
pitstop skill install --agent codex --scope project --repo /work/app --dry-run --json
```

The result contains `agent`, `scope`, `targetPath`, `bundleVersion`, sorted bundle-relative `files`, `dryRun`, `action`, `installed`, and `warnings`. Use exit `0` for a planned or completed install and exit `2` for invalid flags, `SKILL_EXISTS`, unsafe destinations, or installation/recovery failures. No install path edits `AGENTS.md`, `CLAUDE.md`, workflows, or project scripts.

## Limits and trust boundary

The parser enforces bounded files and structure: config bytes 1 MiB, JSON nesting depth 16, at most 128 jobs and 128 checks, 128 steps per check and 2,048 total steps, 64-byte ASCII IDs, 65,536-byte step commands, 256-byte step names, and 1,024-byte images. It rejects duplicate or unknown fields, null defined fields, malformed Unicode, unsupported schema versions, special files, symlinks, and unsafe controls. Existing configs that relied on previously tolerated input may need a deliberate manual correction; validation does not rewrite them.

The validator accepts configuration data only. It does not certify shell code, image contents, registry/tool availability, container isolation against hostile programs, or parity with a workflow containing expressions, matrices, services, secrets, caches, artifacts, permissions, or deployment behavior. Record each omission or assumption in the agent report.
