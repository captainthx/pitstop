# Evidence ledger

## Scope

Test-only Rust Pitstop configuration for the bounded `rust-manifest` fixture. Selected job `cargo`; selected command `cargo test --locked`. No build or unrelated checks.

## Evidence

| File | Observed evidence | Configuration consequence |
| --- | --- | --- |
| `Cargo.toml` | Package `fixture-rust` at version `0.1.0`, edition `2024` | Rust project identity and edition recorded |
| `Cargo.lock` | Lockfile format `4`, package entry present | `--locked` is required for reproducibility |
| `rust-toolchain.toml` | Channel `1.99.0`, minimal profile, `rustfmt` and `clippy` | Runner image `rust:1.99.0-bookworm` selected |
| File inventory | No `src/`, tests, target declaration, workflow, or existing `.pitstop.json` | Preserve the evidence-backed test command; report runtime gap |
| Installed Rust mapping | One `cargo test --locked` step on `rust:1.99.0-bookworm` | Used as the starter shape for this test-only case |

## Prerequisites and gaps

- A Rust source or declared test target must be present before the selected test can be environment-ready.
- Docker/image availability was not tested.
- No Cargo command, Docker command, network access, deploy, or source/workflow/manifest change was performed.
- Validator success proves config syntax/shape and selection only; it does not prove test success or CI parity.

## Command list

- `pitstop-phase-b version`
- `pitstop-phase-b -h`
- `pitstop-phase-b init --help`
- `pitstop-phase-b validate --help`
- `pitstop-phase-b validate --repo <repo> --json`

## Execution

The final validation returned:

`valid: true`, `selectedJobs: ["cargo"]`, `architecture: "linux/arm64"`, `checkCount: 1`, `stepCount: 1`, `issues: []`, exit `0`.

Raw CLI output and exit evidence are in `cli-version.*`, `cli-help.*`, `cli-init-help.*`, `cli-validate-help.*`, and `cli-validate.*`. The final config copy is `config.json`.
