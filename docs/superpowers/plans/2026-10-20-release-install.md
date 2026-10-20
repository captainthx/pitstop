# Release installation implementation plan

> **For agentic workers:** Use superpowers:executing-plans task-by-task. User approved the complete scope in chat; continue without another approval round.

**Goal:** Install Pitstop from versioned GitHub Release assets without a host Go compiler.

**Architecture:** POSIX installer downloads standalone binaries from one pinned release and atomically replaces an explicitly approved destination. A shell build script packages existing Go CLI metadata; GitHub Actions creates draft releases.

**Tech Stack:** Go 1.26.5, POSIX shell, curl, SHA-256 utilities, GitHub Actions/gh; no new runtime dependencies.

**Spec:** `docs/superpowers/specs/2026-10-20-release-install-design.md`

## Global constraints

- Four targets only: darwin/linux × arm64/amd64; stable vMAJOR.MINOR.PATCH tags.
- Preserve source builds and existing CLI interfaces; never publish or push in this implementation session.
- Downloads use HTTPS; verify before replacement; no sudo/profile edits.

## Review focus

- Partial/mismatched downloads must preserve old bytes.
- Latest changing between requests must not mix releases.
- Symlinks/directories must not redirect or absorb installation.
- Shell metacharacters and spaces in arguments must remain literal.
- Checksums must match exactly one expected filename, never arbitrary checksum paths.

### Task 1: Installer

Files: `scripts/install.sh`, `scripts/install_test.go`.
Interface: flags and asset names specified in the design; tests intercept only network/host probes, with real hashing/filesystem/shell behavior.

- [x] Add tests and observe failure before implementation.
- [x] Implement bounded download, platform/tag/argument validation, checksum and version checks, destination staging and cleanup.
- [x] Run hermetic tests including all review-focus inputs.

### Task 2: Release build and automation

Files: `scripts/release.sh`, `scripts/release_test.go`, `.github/workflows/release.yml`, `.github/workflows/ci.yml`.
Interface: `sh scripts/release.sh vX.Y.Z OUTPUT_DIR`; output four binaries, installer and checksum file. Consume Task 1's exact names.

- [x] Add packaging assertions and observe missing-script failure.
- [x] Implement cross-build with injected CLI version/commit, checksums; refuse an existing output directory.
- [x] Add tag-triggered draft release and installer test matrix, pin existing actions to repository-used SHAs.
- [x] Build and inspect artifacts; run native binaries where supported.

### Task 3: Documentation and final verification

Files: README pair, user-guide pair, `docs/install.md`, docs index, roadmap, `docs/evidence/release-install.md`.

- [x] Document download-review-execute commands, exact-version install, upgrade, removal, PATH, prerequisites and trust limits; mark links pending publication.
- [x] Run Go tests/race/vet, shell syntax, packaging and existing config-agent smoke.
- [x] Fresh independent review, fix actionable findings, preserve evidence and commit the completed implementation locally.
