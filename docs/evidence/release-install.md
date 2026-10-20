# Release installation verification

Date: 2026-10-20, Asia/Bangkok. Scope: [approved design](../superpowers/specs/2026-10-20-release-install-design.md), [plan](../superpowers/plans/2026-10-20-release-install.md).

## Decisions and execution

- Ruling: retain the existing feature checkout and Go module path. Release binaries use the existing linker variables; no caller imports or engine/snapshot behavior need changes. Cost: `go install github.com/captainthx/pitstop/cmd/pitstop@...` is not a supported installation route.
- Ruling: use standalone binaries instead of archives. Cost: larger downloads; no archive traversal or extraction surface is introduced.
- Ruling: require `--force` before replacing any existing regular file and refuse symlink/non-file targets even with force. Cost: upgrades need explicit opt-in. A user-controlled directory is the trust boundary; hostile concurrent directory changes are not defended against.
- Ruling: generate draft releases only. No tag, push, release creation or publication is performed in this implementation session. Cost: the installation link remains unavailable until a maintainer publishes the first reviewed release.
- Ruling: implement inline with independent review at the end; the user approved the concrete scope. Preserve design, plan and this evidence in docs.

## Test-first record

- `go test ./scripts -run TestReleaseInstaller -count=1` failed before `scripts/install.sh` existed (missing installer); then passed after implementation.
- `go test ./scripts -run TestReleaseRejectsInvalidInputs -count=1` failed before `scripts/release.sh` existed; then passed after implementation.
- Installer tests use real shell, SHA-256 and filesystem operations, substituting platform/network probes plus injected final-rename failure and chmod/noexec modeling. Test payloads are shell fixtures; a separate package test installs the real native Go binary.
- Cases cover four platform mappings, latest tag pinning, initial install/upgrade, explicit overwrite opt-in, checksum mismatch/missing/duplicate entry, partial/failed downloads, version mismatch, unsupported OS/CPU, malformed/multiline/metacharacter tags, invalid flags, symlink/directory refusal, destination paths with spaces and shell metacharacters, preservation on rename failure, temporary/staging cleanup, and non-executable download storage.
- Packaging verifies the exact six-file inventory, five SHA-256 entries, all four Mach-O/ELF CPU formats, version `1.2.3` plus current HEAD commit in native JSON output, then installs the native release binary into an isolated temporary HOME and compares metadata.

## Completed local checks

- Host: macOS arm64, default Go 1.27.2.
- `go test -race -count=1 ./...`: passed for every package.
- `go vet ./...`: passed.
- `PITSTOP_TEST_RELEASE=1 go test ./scripts -count=1`: passed, including real native binary installation.
- Go 1.26.5 floor: `GOTOOLCHAIN=go1.26.5 PITSTOP_TEST_RELEASE=1 go test -race -count=1 ./...` and `GOTOOLCHAIN=go1.26.5 go vet ./...` passed before review and passed again after the noexec review fix.
- Go 1.26.5 CLI build plus `PITSTOP_CONFIG_AGENT_RUNTIME=0 sh scripts/config-agent-smoke.sh`: passed for init/validate/custom/skill and every embedded example/mapping; runtime toolchain smoke was explicitly skipped.
- `sh -n scripts/install.sh scripts/release.sh`: passed.
- Both workflow YAML files parsed with Ruby YAML; this confirms syntax only, not GitHub Actions execution.
- `git diff --check`: passed.
- SHA pins for artifact upload/download actions resolved through the official actions repositories with read-only GitHub API calls.
- Read-only `gh api repos/captainthx/pitstop/releases` returned `[]`; documentation marks publication pending.

## Independent review

The read-only QA reviewer returned **PASS WITH NOTES**, with one Minor finding: the installer executed its version check from TMPDIR, which may be mounted noexec on Linux. No replacement/checksum/workflow data-loss blocker was found.

A regression test modeled a non-executable download file and failed with Permission denied before the fix. The installer now checks the destination staging executable before the atomic rename. This preserves the existing executable on failure and avoids executing from the download directory. The final independent re-review returned **PASS**, confirmed the original finding closed, and re-ran `go test ./scripts -count=1` successfully. No further actionable correctness, security or data-loss finding remained.

The evidence document was created while the reviewer was reading earlier files; the reviewer also requested completion of evidence and plan checkboxes. These bookkeeping items are now complete.

## Native Linux arm64 noexec smoke

Built the real four-target release assets with Go 1.26.5 using `sh scripts/release.sh v1.2.3 TEMP_RELEASE_DIR`. This is a local test version, not a published release. Installed its actual Linux arm64 binary inside cached Alpine 3.22 (image `sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8`) using:

- Docker arm64 daemon, read-only root filesystem, `--network none`.
- `/download` tmpfs with `noexec`, `/install` tmpfs with execution permitted.
- Real release installer, manifest, SHA-256 tool and binary; a curl fixture copies the local release assets instead of accessing the network.
- Isolated `HOME=/install`, `TMPDIR=/download`; no host install directories mounted writable.

Actual result:

```text
Installed Pitstop v1.2.3 to /install/.local/bin/pitstop
Add the install directory to PATH in your shell configuration: /install/.local/bin
{"schemaVersion":1,"command":"version","exitCode":0,"result":{"version":"1.2.3","commit":"cde83809e7b08bc069b9fb1a59af8af5e2dec955","buildTime":null}}
```

That commit is the HEAD before these implementation changes were committed; the local build included the working-tree changes. Production workflow builds from a checked-out release tag. The smoke exited 0; its container was removed. This verifies Linux arm64 installation/version in Docker, not full Pitstop check execution on a separate native Linux machine.

## Limits

No remote GitHub Actions run, live-release installation, separate-host Linux execution, Linux amd64 native execution, macOS amd64 native execution, Developer ID signing/notarization, signed checksums/provenance or hostile shared-directory race resistance is claimed. The hermetic tests model host selection; the separate container smoke supplies actual Linux arm64 installation/version evidence. CI defines native macOS/Linux checks that must run remotely after integration. SHA-256 checks file consistency, not publisher authenticity. Shellcheck and actionlint were not installed, so neither tool was run. No global CLI/skill or user profile was changed.
