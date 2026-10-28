# Pitstop baseline test journal

## Scope and assumptions

- Target: `/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-agent-testing-k6gyr9xk/baseline/repo` only.
- Used the supplied portable `pitstop-config` skill at `baseline/skill/SKILL.md`.
- The repository's hidden `.pitstop.json` was pre-existing and selected `quality_gate`; no source, test, or config files were edited.
- The fixture check is evidence-backed by `check.sh`: `test "$(cat answer.txt)" = 42`.
- Docker endpoint came from `/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-agent-testing-k6gyr9xk/docker-host.txt` (`unix:///Users/thanyatab/.orbstack/run/docker.sock`).

## Commands and results

All Pitstop invocations used this exact isolated prefix:

`rtk proxy env HOME=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-agent-testing-k6gyr9xk/baseline/home XDG_CACHE_HOME=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-agent-testing-k6gyr9xk/baseline/cache DOCKER_HOST=unix:///Users/thanyatab/.orbstack/run/docker.sock /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-agent-testing-k6gyr9xk/bin/pitstop`

1. `rtk cat .../baseline/skill/SKILL.md` — exit 0; read the supplied skill.
2. `rtk find .../baseline -maxdepth 2 -type f -print` — exit 0; found `bin/pitstop`, root `docker-host.txt`, `baseline/SKILL.md`, `context.json`, and repository files.
3. `rtk cat .../baseline/repo/check.sh` — exit 0; command is `sh` with `set -eu`, checks `answer.txt` equals `42`, then prints `fixture assertion passed`.
4. `rtk cat .../baseline/repo/answer.txt` — exit 0; content `42`.
5. `${prefix} version` — exit 0; `pitstop 0.1.0`.
6. `${prefix} -h` — exit 0; commands include `doctor`, `list`, `run`, `validate`.
7. `${prefix} run --help` — exit 0; saved to [run-help.txt](run-help.txt), stderr [run-help.stderr](run-help.stderr).
8. `${prefix} validate --help` — exit 0; saved to [validate-help.txt](validate-help.txt), stderr [validate-help.stderr](validate-help.stderr).
9. `${prefix} doctor --help` — exit 0; saved to [doctor-help.txt](doctor-help.txt), stderr [doctor-help.stderr](doctor-help.stderr).
10. `${prefix} doctor --repo .../baseline/repo --json` — exit 0; `ready=true`, repository/config/image/docker all `ready`, image available. Captured in [doctor.json](doctor.json) and [doctor.stderr](doctor.stderr).
11. `rtk cat .../baseline/repo/.pitstop.json` — exit 0; pre-existing schema v2 config uses `alpine:3.22`, `linux/arm64`, job `quality_gate`, step `sh check.sh`.
12. `${prefix} list --repo .../baseline/repo --json` — exit 0; `quality_gate` selected and eligible. Captured in [list.json](list.json) and [list.stderr](list.stderr).
13. `${prefix} validate --repo .../baseline/repo --json` — exit 0; valid, selected jobs `[quality_gate]`, 1 check, 1 step, no issues. Captured in [validate.json](validate.json) and [validate.stderr](validate.stderr).
14. `${prefix} run --repo .../baseline/repo --job quality_gate --timeout 2m --json` — exit 0; job and step both `passed`, engine exit code 0, selected coverage `complete`, duration 1091 ms. Captured in [run.json](run.json) and [run.stderr](run.stderr).

One initial probe used the incorrect path `.../baseline/docker-host.txt`; that file did not exist. I corrected it to the harness-specified root `.../docker-host.txt` before running Pitstop. The final commands above used the correct endpoint.

## User-facing response

ทดสอบโปรเจกต์ด้วย Pitstop สำเร็จครับ

- `doctor`: พร้อมใช้งานทั้ง Git/config/Docker/image
- `validate`: config ถูกต้อง (`quality_gate`, 1 check, 1 step)
- `run --job quality_gate`: ผ่าน (`assert_answer`, exit code 0) และได้ผล `fixture assertion passed`
- ผล JSON: [run.json](run.json)

ไม่แก้ไข source, tests หรือ `.pitstop.json` ของโปรเจกต์
