# Pitstop CLI outputs

The installed binary was inspected before configuration.

Command:

```sh
/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a version
```

Exit code: `0`

Output:

```text
pitstop 0.1.0
```

Command:

```sh
/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a -h
```

Exit code: `0`

Output:

```text
Run configured checks in fresh Docker containers using the current working-tree snapshot.

Features:
  - Run selected checks without installing their toolchain on the host.
  - Keep steps in one job filesystem while isolating jobs from one another.
  - Capture step evidence, logs, reports, timeout, cancellation, and cleanup.

Configure .pitstop.json with schemaVersion 2, a Docker image, and checks.

Usage:
  pitstop [command]

Examples:
  pitstop doctor --repo /path/to/repo
  pitstop list --repo /path/to/repo
  pitstop run --repo /path/to/repo --json
  pitstop skill install --agent codex --scope project --dry-run --json

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  doctor      Check Git, config, Docker, and image readiness
  help        Help about any command
  init        Create a starter .pitstop.json configuration
  list        List configured checks and selection
  run         Run selected checks in isolated Docker containers
  skill       Manage installed agent skills
  validate    Validate the repository configuration without running checks
  version     Show version and build information

Flags:
  -h, --help          help for pitstop
      --repo string   target repository path (default ".")

Use "pitstop [command] --help" for more information about a command.
```

Command:

```sh
/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a validate -h
```

Exit code: `0`

Output:

```text
Validate the repository configuration without running checks

Usage:
  pitstop validate [flags]

Flags:
      --arch string   override configured container architecture (linux/arm64 or linux/amd64)
  -h, --help          help for validate
      --job strings    configured check ID (repeatable or comma-separated)
      --json           JSON output

Global Flags:
      --repo string   target repository path (default ".")
```

Command:

```sh
/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a validate --repo /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/trials/yarn-classic/repo --json
```

Exit code: `0`

Output:

```json
{"schemaVersion":1,"command":"validate","exitCode":0,"result":{"repo":"/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/trials/yarn-classic/repo","configPath":"/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/trials/yarn-classic/repo/.pitstop.json","valid":true,"selectedJobs":["verify"],"architecture":"linux/arm64","checkCount":1,"stepCount":3,"issues":[]}}
```
