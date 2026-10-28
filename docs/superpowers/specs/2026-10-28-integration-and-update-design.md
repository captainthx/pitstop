# Go integration coverage, Testcontainers, and self-update

Date: 2026-10-28 (Asia/Bangkok). Status: approved by the user for sequential implementation with `luna_worker`; see the implementation plan for progress. Not a release announcement.

## Intent and evidence

The user wants setup to include a project's integration tests, including real Testcontainers execution, and a simple `pitstop update` that updates both the CLI and existing installed skills. Work remains sequential, with a separate branch per deliverable.

Read-only inspection of a user-provided Go service found a separate integration build tag, a Makefile integration target, a separate CI integration job, and Testcontainers dependencies for PostgreSQL and object storage. A plain `go test -race ./...` excludes the tagged files. Some external-service tests can also skip when their configuration is absent. These are observations about the current example, not a reproduction of the user's previous setup: that checkout currently has no `.pitstop.json` to audit.

The example repository is not a fixture to modify or publish. Use newly authored, generic fixtures in this public repository. Do not copy its source, private paths, credentials, or business details into evidence.

## Deliverables and order

1. `feature/self-update`: CLI plus installed-skill updater, reusing the release installer and skill ownership checks.
2. `feature/testcontainers-runner`: an explicitly enabled Docker-in-Docker backend for selected checks, with real execution and cleanup evidence.
3. `feature/go-integration-setup`: evidence-based Go unit/integration discovery, coverage reporting, portable bundle upgrade, and agent trials against the new backend.

Each branch is reviewed and checked before merging into the base for the next. No new release, tag movement, or replacement of published v0.1.0 assets is part of this design. A separately authorized new release will be needed to distribute these capabilities through binary downloads. v0.1.0 has no update command; its first upgrade still uses the existing installer.

## 1. CLI and skill update

### User interface

```sh
pitstop update
pitstop update --version vX.Y.Z
pitstop update --repo /path/to/project --json
```

- Default target: latest published stable `vMAJOR.MINOR.PATCH` on the fixed official GitHub repository.
- `--version` explicitly pins a stable release, including an intentional reinstall or downgrade. Reject malformed versions before downloading.
- `--json` keeps one result envelope on stdout and operational progress on stderr.
- Update the actual running executable, including a custom installation directory; do not guess `$HOME/.local/bin` or search PATH for another installation.
- Update only **existing** Codex/Claude skills at the known user-scope paths and in the selected Git root. Do not recursively scan other projects or create new skill installations.
- Outside a Git repository, an implicit default `--repo .` updates user skills only. An explicitly invalid `--repo` fails before any update.
- An already-current CLI still checks/updates these installed skills. Default latest must not downgrade a newer stable binary. A non-semver development build requires an explicit target if replacement cannot be ordered safely.

### Update protocol

1. Resolve the executable and selected project, inventory existing skill targets, and acquire a nonblocking lock for this executable's update. Reject unsafe/nonregular or changed destinations. No sudo, profile changes, credentials forwarding, or arbitrary target-directory flag is added.
2. Resolve the latest stable tag once, using bounded HTTPS and the exact official repository URL. Pin the selected tag for the rest of the operation.
3. Reuse the **checked-in** `scripts/install.sh`, embedded in the CLI, to install that pinned release into a private staging directory on the executable's filesystem. Do not download and execute a mutable remote installer. Keep the installer as one source of truth; a small repo-root Go asset package is acceptable if required by `go:embed` path rules.
4. Use the verified staged binary's `version --json` and `skill install --force --dry-run --json` to preflight every existing target. Check process exits and structured results; a zero-exit blocked action is still blocked. Unknown, changed, or recovery-required skill directories must remain untouched, and a preflight failure leaves the original CLI in place.
5. Atomically replace the executable only after successful download, checksum/version checks, and preflight. Recheck destination identity under the update lock immediately before replacement. When default latest equals the current stable version, preserve the current executable and use it for skill refresh; an explicit `--version` may reinstall.
6. Invoke the **new executable in a child process** for actual skill updates. Calling `skill.Install` in the old updater process would reuse stale embedded bytes and is incorrect. Pass explicit agent/scope and only a project scope's `--repo`.
7. Report per-target outcomes, previous/target versions, executable path and whether completion is partial. A failure before publication preserves the CLI. A failure or cancellation after CLI replacement reports the actual changed CLI and remaining/blocked skills; do not pretend the whole operation rolled back. Retrying can finish remaining owned targets.

For a default same-version refresh, use the current executable for both skill preflight and installation so a source-built newer bundle is not silently replaced by the older published bundle with the same version string.

Output statuses distinguish `updated`, `up_to_date`, `blocked`, and `not_attempted`. Exit 0 means every selected operation completed or was already current; exit 2 means blocked, failed, or partial completion; exit 130 takes precedence for cancellation. Progress and errors must not corrupt JSON. Existing checksum trust limitations remain unchanged.

### Acceptance

- Isolated temporary HOME/bin/project fixtures prove replacement at a custom executable path and both user/project skill scopes for Codex and Claude.
- The new executable supplies the installed bundle; an old-process bundle cannot make the test pass.
- Missing skill directories stay absent. Modified/unknown skill files survive unchanged, and preflight prevents CLI replacement in those cases.
- Wrong checksum/version, bad release URL/version, unsupported platform, unwritable/unsafe destination, concurrent update and pre-publication cancellation preserve the original executable.
- Late skill-write failure and cancellation report partial completion accurately. Default same-version refresh and newer-version protection are exercised.
- Download tests use controlled fixtures, not replacement of the real user's binary or skills.

## 2. Real Testcontainers checks

### Explicit capability

Add an optional boolean `testcontainers` to an individual schema-v2 check, default false. Accept existing configs unchanged; preserve strict unknown-field, null, type, size and ID validation.

```json
{
  "id": "integration",
  "testcontainers": true,
  "steps": [
    {"id": "test", "run": "go test -race -count=1 -json -tags integration ./..."}
  ]
}
```

This is a config fragment; the ID and command come from project evidence, not a universal naming convention. Integration tests that do not need Docker do not require this capability.

```sh
pitstop validate --repo /path/to/project --json
pitstop run --repo /path/to/project --job integration --allow-testcontainers --json
```

`--allow-testcontainers` is an explicit per-invocation authorization for a privileged DinD service. Without it, selecting any opted-in check returns a clear blocked result before allocating containers/networks or starting any project checks. A repository config alone cannot enable privileged execution. `doctor` may describe prerequisites but does not start a privileged probe or grant this authorization.

### Chosen backend

Use a dedicated `docker:29.8.2-dind` daemon container per opted-in check. The tag's manifest was checked during design and advertises Linux amd64/arm64. Implementation must also resolve and record the actual platform image ID/digest, as with the runner image.

- Create a uniquely named, run-labelled user-defined Docker network and DinD container. Only the DinD service is privileged; the project runner remains unprivileged.
- Use fresh anonymous DinD data storage, not a host bind mount or a persistent shared cache. Do not mount the host Docker socket or repository into either container.
- Attach only that check's runner and daemon to the network, with a fixed private daemon alias. Publish no daemon or child-container port on the outer host.
- Configure the runner for the inner daemon using `DOCKER_HOST=tcp://<daemon-alias>:2375`, `TESTCONTAINERS_HOST_OVERRIDE=<daemon-alias>` and the inner daemon's `/var/run/docker.sock` for the reaper socket override. Disable TLS only on this private, unpublished connection. Do not forward host Docker contexts, credentials, `.env`, or arbitrary host variables.
- Wait for the inner daemon with a bounded readiness probe using the daemon container's own Docker client before starting project steps. Capture daemon diagnostics on failure.
- Nested Testcontainers ports must be reachable through the daemon alias. The initial supported path is public images and a Docker daemon that permits privileged Linux containers; unsupported/rootless/restricted environments report blocked, never silently fall back to host execution or host socket sharing.

DinD still requires privileged execution and is not a hostile-code sandbox. That is why it is opt-in even though it avoids directly granting the project the outer Docker API socket. This design does not add a generic service/compose engine.

### Lifecycle and reporting

Preserve the existing working-tree snapshot, per-check filesystem, steps, timeout, cancellation and report semantics. Include provisioning/readiness within the check deadline; image preparation retains its separately bounded budget.

Label and record ownership before subsequent operations can fail. Clean up runner, DinD service, its anonymous data volumes and the private network on success, test failure, timeout, cancellation, source-copy failure and startup failure. Removing the dedicated daemon removes its nested workload environment; do not enumerate or delete unrelated outer-daemon Testcontainers resources. Final label-based recovery handles this run's containers/networks only and verifies their ownership before removal.

Retain ordinary config/run results for non-opted-in checks. Report daemon image identity and diagnostics for opted-in checks. Distinguish daemon readiness failure (no test executed), real test failure, timeout/cancellation, and incomplete cleanup. Preserve the current cancellation/failure/integrity exit-code precedence. Expose incomplete resource cleanup in warnings and persistent ownership evidence rather than declaring a trustworthy pass.

### Acceptance

- A newly authored Go fixture uses Testcontainers for a real database query or equivalent service round-trip, including mapped-port connectivity through the daemon alias. Pin the fixture dependency; do not add Testcontainers to Pitstop's production dependencies merely for the fixture.
- Unit-only config still runs with no privileged service and no new flags.
- Opted-in config without the CLI authorization allocates no runtime resources.
- Real success/failing assertion, daemon startup failure, timeout and SIGINT trials demonstrate outcomes and removal of this run's containers, anonymous volumes and network. Unrelated labelled/unlabelled resources remain untouched.
- Linux amd64 CI and local macOS ARM64/Linux-daemon evidence are distinct; untested platforms are not advertised as verified.

## 3. Go setup and coverage

Extend the existing portable skill rather than inventing a universal integration command or workflow executor.

- Inspect Go test files/build constraints, `go.mod`/`go.work`, nested modules, Makefile/task/shared-script targets, CI jobs, and relevant test helpers before choosing test scope.
- Identify unit, tagged integration, external-service and environment-gated/skipped suites. Trace Testcontainers imports/helpers and required images. Do not assume `go test ./...` covers tagged files or every nested module.
- Prefer evidence-backed commands. The generic Go preset stays a starter, not a claim of complete project coverage. Do not add `-tags integration` to every Go project.
- Preserve existing valid checks and user edits. When the requested full test scope has missing coverage, show the gap and add only evidence-backed missing checks if authorized; do not regenerate the whole config or silently report the smaller scope as complete.
- Enable a check's Testcontainers capability only when its helpers require it. Before executing that privileged scope, obtain the actual user authorization for `--allow-testcontainers`; an ordinary test request alone is not that capability grant. Configuration-only inspection does not execute tests or privileged services.
- Read Go JSON/log output and report skipped tests, empty targets and environment-gated suites explicitly, even if the shell command returned 0. Do not manufacture credentials or claim external-service coverage from skipped tests.
- Keep unsupported features and dependencies visible, including private registries and external services not provided by the selected test helpers.

Update the existing examples/references, AI guides, user guides, command tables, installation guide and roadmap where their behavior changes. Add generic tagged-test fixtures and real isolated agent trials. Bump the embedded bundle from 2 to 3, regenerate its manifest/allowlist, archive exact bundle-2 history if not already present, and retain the released-v0.1.0 upgrade protections.

## Boundaries

No edits or execution in the user's example repository, secrets import, arbitrary host project execution, general workflow/compose/services scheduler, host Docker socket mode, Windows support, automatic image builds, resource/cache redesign, recursive machine-wide skill search, automatic privileged execution, or new public release are included.

Implementation details such as helper names and internal file splits are left to the builder, provided the existing parser/runner/report/installer responsibilities remain clear. Tests must exercise behavior at the actual trust and process boundaries rather than only matching documentation strings.

## Primary references

- [Go build constraints and `-tags`](https://pkg.go.dev/cmd/go): tagged files require a matching build context.
- [Testcontainers Go configuration](https://golang.testcontainers.org/features/configuration/) and [networking](https://golang.testcontainers.org/features/networking/): Docker endpoint and host override affect daemon access and mapped-port connections.
- [Running Testcontainers inside containers](https://golang.testcontainers.org/system_requirements/ci/dind_patterns/): the execution environment needs an appropriate Docker access pattern.
- [Official Docker image documentation](https://github.com/docker-library/docs/blob/master/docker/README.md): DinD requires privileged mode and has explicit TLS configuration.
