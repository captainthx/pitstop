# Release installation design

Approved scope: GitHub Releases binaries for macOS/Linux, arm64/amd64; a user-local installer with checksums, safe replacement, release workflow, tests and bilingual documentation. No Homebrew, Windows or self-update.

## Decisions

- Publish four standalone binaries (`pitstop_darwin_arm64`, `pitstop_darwin_amd64`, `pitstop_linux_arm64`, `pitstop_linux_amd64`), `checksums.txt`, and `install.sh`. Raw binaries avoid archive extraction hazards.
- Stable tags are `vMAJOR.MINOR.PATCH`; inject version without `v` and source commit through existing CLI linker variables. Keep the current Go module path and source build intact.
- POSIX `install.sh` accepts `--version TAG` (default latest stable), `--bin-dir DIRECTORY` (default `$HOME/.local/bin`), `--force`, and `--help`. Resolve latest once to a tag before fetching assets. HTTPS only, bounded curl timeouts, exact checksum entry, no eval, no sudo, no profile edits.
- Download into private temporary storage, verify SHA-256 and executable version, then stage within the destination directory and rename. Existing files require `--force`; symlinks and non-regular destinations are rejected even with force. Failure before rename preserves the prior executable. Concurrent modification of a user-controlled install directory is outside the threat boundary; do not install into shared/untrusted directories.
- Checksums detect corruption but do not authenticate against a compromised GitHub account/release. HTTPS and the repository are trust roots. macOS signing/notarization is deferred and must be disclosed.
- The tag workflow verifies tests, builds all binaries and uploads a draft release, never automatically publishes. Installer links work only after a release is published. No tag push or publication is performed as part of implementation.
- CI exercises installer fixtures on macOS/Linux and verifies release build artifacts. Native Linux arm64 runtime remains unverified unless an actual matching runner is used.

## Verification

Hermetic installer tests cover platform mapping, latest pinning, checksum mismatch/missing/duplicate entries, failed/partial download, unsupported host, invalid arguments/tag, version mismatch, spaces, overwrite opt-in, symlink/non-file refusal and preserving old bytes on failures. A local release build verifies four executable formats, injected metadata and checksum inventory. Existing Go tests/vet and config-agent smoke remain required.
