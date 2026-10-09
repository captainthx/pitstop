# Evidence

## Selected scope

- Job: `verify`
- Check: `verify`
- Step: `cargo test --locked`
- Runner image: `rust:1.99.0-bookworm`
- Runner architecture: `linux/arm64` (observed host architecture)
- Test-only scope: no build step was selected.

## Repository evidence

- `Cargo.toml` declares package `fixture-rust`, version `0.1.0`, edition `2024`.
- `Cargo.lock` is present and uses lockfile version `4`.
- `rust-toolchain.toml` pins channel `1.99.0`, profile `minimal`, and lists `rustfmt` and `clippy` components.
- No workflow, package script, or other test runner was present in the fixture.
- The installed Rust example uses `rust:1.99.0-bookworm` and locked Cargo commands; only its test command was retained to honor the requested test-only scope.

## Assumptions and gaps

- `linux/arm64` was selected from `uname -m` returning `arm64`; the CLI validator accepts the explicit architecture but does not prove image availability.
- Validation proves config shape and bounds only. It does not prove image pulls, dependency installation, Cargo success, runtime isolation, or CI parity.
- No workflow parity claim is made because no workflow was present.
- The Rust toolchain and lockfile are prerequisites for a future run.
- Actual Codex subagent: `luna_worker`, model `gpt-5.6-luna`, reasoning `high`.
- Client version: unavailable.
- Skill handling: read the explicitly installed project skill at `.agents/skills/pitstop-config/SKILL.md` and its config, examples, and workflow references only.

## Command list

Every shell command was invoked through `rtk proxy`.

- `pitstop-phase-a --version` (exit `2`; unsupported flag)
- `pitstop-phase-a version` (exit `0`; `pitstop 0.1.0`)
- `pitstop-phase-a -h` (exit `0`)
- `pitstop-phase-a validate -h` (exit `0`)
- `pitstop-phase-a validate --repo <fixture> --json` (exit `0`; valid, one job, one step)
- Read-only inspection: `uname -m`, `git status --short --branch`, file listing, and reads of request, manifests, lockfile, toolchain, and installed skill references.

## Project commands

No project commands ran. In particular, `cargo` was not executed while gathering evidence or validating the configuration.
