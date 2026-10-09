# Scope

Mapped the workflow's normal verification into one schema-v2 `verify` check. The selected scope is the dependency bootstrap followed by the Python test command.

Trial metadata: Codex `luna_worker` (`gpt-5.6-luna`, high); client unavailable. Used the explicitly installed project skill only.

# Evidence

- `.github/workflows/complex.yml:7` runs `python -m pip install -r requirements-dev.txt` in `prepare`.
- `.github/workflows/complex.yml:21` runs `python -m pytest` in `verify`.
- `requirements-dev.txt:1` declares `pytest>=8,<9`.
- `pyproject.toml:4` also declares `pytest>=8,<9`.
- `.pitstop.json` records the reviewed commands in order using `python:3.13-slim-bookworm` and `linux/arm64`.

# Prerequisites and gaps

- The selected runner image must be available for `linux/arm64` when the check is later run.
- The dependency bootstrap must have network or package-cache access at runtime; this was not exercised.
- Python 3.13 is the selected matrix leg. The Python 3.12 leg is omitted because one v2 check has one runner image and no matrix expansion.
- The `prepare` job relationship and `needs: [prepare]` are flattened into ordered steps in one check.
- The PostgreSQL 16 service is omitted because v2 has no service definition.
- `TOKEN: ${{ secrets.CI_TOKEN }}` is omitted to keep credentials out of the generated config.
- `MODE: ${{ matrix.python }}` is omitted because v2 does not represent matrix-derived environment values.
- This config cannot claim CI parity or prove that tests require no database, secret, or mode value.

# Command list

- `/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-b version`
- `/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-b -h`
- `/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-b validate -h`
- `/Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/bin/pitstop-phase-b validate --repo /Users/thanyatab/Desktop/project/my-lab/pitstop/.superpowers/sdd/2026-10-08-config-agent/task-7-scratch/phase-b-retries/unsupported-ci --json`

# Execution

- CLI version: `pitstop 0.1.0`, exit `0`.
- Root help and `validate -h`: exit `0`; raw output and exit files are under `raw-cli/`.
- Validation: exit `0`, `valid: true`, selected job `verify`, `checkCount: 1`, `stepCount: 2`, and no issues.
- No project commands, Docker commands, workflow commands, or checks were executed.
