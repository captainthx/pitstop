# Requirements and scope

Pitstop requires a Git repository with at least one commit, Git, Docker CLI, and a reachable Docker daemon. The target project supplies a schema version 2 `.pitstop.json`; Pitstop supplies source snapshot isolation, container lifecycle, timeout, cancellation, cleanup, logs, evidence, and reports.

The configuration names a Linux image (`linux/arm64` or `linux/amd64`), selected check IDs, and ordered shell steps. The image must contain the tools used by those steps. Public images or a user-built image are supported; Pitstop does not build images automatically.

The first version supports `run`, `doctor`, `list`, and `version`. It does not interpret `.github/workflows/*.yml`, install or invoke `act`, translate `uses:` actions or expressions, or provide matrix, needs, services, secret injection, cache volumes, or self-hosted runner registration. `workflowCoverage` is always `not_applicable`.

Each selected check runs in a fresh container. Its steps share `/workspace` and generated evidence under `/pitstop`; another check cannot see files created by the previous check. The working tree is copied from a snapshot through `docker cp`, so dirty, new, and deleted files are tested without a host bind mount. Timeout and cancellation stop the active owned container, then cleanup removes only matching run labels.

Schema version 1 is rejected with migration guidance. A migration moves `run` commands into `checks`, replaces `checkout` with the Pitstop snapshot, and replaces setup actions with an image toolchain. Existing GitHub workflows remain the CI project's responsibility.

Reports contain an immutable image ID, step exit records, raw logs, source identity, command argv, and cleanup evidence. Normal check failures are distinct from missing evidence and container runtime failures. Exit codes are 0 for all passed, 1 for proven check failure, 2 for invalid/blocked/unknown/persistence/cleanup failure, and 130 for cancellation.

See the [user guide](user-guide.en.md), [Docker examples](../examples/docker-react.json), and retained historical [validation evidence](evidence/final-validation.md).
