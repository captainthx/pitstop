# Evidence and scope

## Evidence inspected

- `.github/workflows/complex.yml`: `verify` runs `python -m pytest`; the workflow also has a separate `prepare` install job, Python `3.12`/`3.13` matrix, PostgreSQL 16 service, `TOKEN` from `secrets.CI_TOKEN`, `MODE` from the matrix, and `needs: [prepare]`.
- `pyproject.toml`: declares `pytest>=8,<9`.
- `requirements-dev.txt`: declares `pytest>=8,<9`.
- No existing `.pitstop.json` was present.
- The installed skill's `partial-features.json` mapping uses the same one-step `verify` check and Python 3.13 image.

## Represented by one schema-v2 check

The `verify` job's reviewed `python -m pytest` command is represented as check `verify`, step `test`. The runner is explicitly `python:3.13-slim-bookworm` on `linux/arm64`.

## Omitted or unsupported

- `prepare` dependency installation: omitted from the selected check per the bounded partial-features mapping.
- Python 3.12 matrix leg: schema-v2 selects one runner architecture/image and does not expand matrix legs here.
- PostgreSQL 16 service: services are outside the config schema and were kept out as requested.
- `CI_TOKEN` and `MODE`: secrets and dynamic matrix environment are omitted; no credential values were copied.
- `needs: [prepare]`: job dependency is not represented in this one-check selection.

## Assumptions and gaps

- `pytest` and any runtime dependencies needed by the test are assumed available when the check runs. Validation does not install dependencies or run the check, and the Python base image's runtime readiness was not tested.
- The generated check represents the reviewed test command only; it does not claim parity with the workflow's matrix, service, secret, environment, or job dependency behavior.
- Image availability, dependency installation, shell success, and test success remain unverified.

## CLI and inspection commands

All shell invocations began with `rtk` as required.

```sh
rtk cat request.md
rtk cat .agents/skills/pitstop-config/SKILL.md
rtk cat .agents/skills/pitstop-config/references/config-reference.md
rtk cat .agents/skills/pitstop-config/references/examples.md
rtk cat .agents/skills/pitstop-config/references/workflow-mapping.md
rtk cat pyproject.toml
rtk cat requirements-dev.txt
rtk find .github -type f
rtk cat .github/workflows/complex.yml
rtk cat .agents/skills/pitstop-config/assets/mappings/partial-features.json
rtk cat .agents/skills/pitstop-config/assets/workflows/partial-features.yml
rtk /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a version
rtk /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a -h
rtk /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a validate -h
rtk /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-a validate --repo /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/trials/unsupported-ci/repo --json
```

## Project execution

None. No project test, install, Docker, network, deployment, or publish command was run. The only execution beyond read-only inspection was the Pitstop CLI's config-only `validate` command.
