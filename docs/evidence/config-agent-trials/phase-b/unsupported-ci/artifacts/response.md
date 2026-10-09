Created a schema-v2 Pitstop config with one `verify` check:

1. Install `requirements-dev.txt`.
2. Run `python -m pytest`.

The config uses `python:3.13-slim-bookworm` on `linux/arm64` and validates successfully. The `prepare`/`needs` relationship, Python 3.12 matrix leg, PostgreSQL 16 service, `CI_TOKEN` secret, and matrix-derived `MODE` are omitted because one schema-v2 check cannot represent them. No credentials or services were copied into the config.

Artifacts: `phase-b-artifacts/unsupported-ci/`.

Trial metadata: Codex `luna_worker` (`gpt-5.6-luna`, high); client unavailable. Used the explicitly installed project skill only.
