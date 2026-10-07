# Pitstop

**English** | [ไทย](README.th.md)

Run your project's lint, test and build commands in Linux containers before commit or push. Test the files currently on your machine and retain logs and a report for review.

**Start here:** build the CLI → copy an example to the target repo's `.pitstop.json` → edit the image/commands → `pitstop doctor` → `pitstop run`.

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

## 1. Install the CLI from source

From this Pitstop source checkout, use Git and Go **1.26.5** as specified in `go.mod`. Docker CLI and a reachable daemon are needed to run checks:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop -h
```

Add `export PATH="$HOME/.local/bin:$PATH"` to your shell configuration to retain it in new terminals. Building from source is the documented installation method today; no release installer link is provided yet.

Host Go builds Pitstop. The project's Node, Java, Go or other check tools belong in the Docker image. See [prerequisites](docs/user-guide.en.md#prerequisites).

## 2. Configure each repository once

**There is no `pitstop init` command yet.** Create `.pitstop.json` at the target repository root by copying an example, then edit it.

Run from the Pitstop checkout and replace the target path:

```sh
PITSTOP_SOURCE="$PWD"
TARGET_REPO="/absolute/path/to/your/repo"
cp "$PITSTOP_SOURCE/examples/docker-react.json" "$TARGET_REPO/.pitstop.json"
```

If a config already exists, edit it instead of overwriting it. For Go, copy `examples/docker-go.json` instead. The Node example is:

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

## 3. Check readiness and run

```sh
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO"
pitstop run --repo "$TARGET_REPO"
```

`doctor` probes config, Docker and image readiness without pulling or creating containers. `run` pulls an image when needed, then runs checks. A missing image may be reported as ready with a message that it will be pulled during run; doctor does not verify that every project command exists in the image.

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
- [Documentation index](docs/README.md) and [current Docker validation](docs/evidence/docker-checks-validation.md)
