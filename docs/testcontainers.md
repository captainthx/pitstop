# Go Testcontainers

For a Go suite that actually uses Testcontainers, set `"testcontainers": true` on that schema-v2 check. Leave ordinary checks unchanged. For example, after confirming the project's real integration command:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "golang:1.26.5", "architecture": "linux/amd64"},
  "jobs": ["unit", "integration"],
  "checks": [
    {"id": "unit", "steps": [{"id": "test", "run": "go test -race ./..."}]},
    {"id": "integration", "testcontainers": true, "steps": [{"id": "test", "run": "go test -race -tags integration -v ./..."}]}
  ]
}
```

Use `linux/arm64` for an ARM64 runner. The integration tag is project-specific; inspect build tags, Makefile and CI before choosing it. A passing unit command does not cover tagged tests, nested modules, or tests skipped for missing external configuration.

```sh
pitstop validate --repo /path/to/project --json
pitstop run --repo /path/to/project --job integration --allow-testcontainers --json
```

`--allow-testcontainers` defaults to false and authorizes a privileged Linux Docker-in-Docker daemon for this invocation. Selecting any marked check without it blocks the entire run before Docker allocation or project checks, including a mixed unit/integration selection. Config alone cannot authorize privileged execution. `doctor` checks ordinary Docker/image readiness; it does not probe privileged support.

Each marked check receives its own pinned `docker:29.8.2-dind` daemon, private labelled network and ephemeral labelled data volume. The runner stays unprivileged. Pitstop passes only the nested Docker endpoint, nested socket override and mapped-port host alias; it does not mount the host socket, source directory, credentials or Docker context into either container. No nested daemon port is published on the outer host. The daemon alias makes mapped service ports reachable from the runner. This requires a trusted project/image and an outer Linux daemon that permits privileged containers; it is not a sandbox for hostile code.

Public service images must be reachable from the nested daemon, and dependency registries from the runner. Existing host image caches and private registry credentials are not forwarded. Each check starts fresh. Provisioning counts toward `--timeout`; cleanup has a separate bounded budget. Success, test failure, timeout and cancellation all remove owned runner/daemon containers, data and network. Inspect `CLEANUP_INCOMPLETE` warnings if Docker stops responding. Ordinary checks keep their existing behavior.

Reports add `jobs[].testcontainers` with daemon image identity, network alias, readiness and available diagnostic-log path. Failed assertions remain failures; unsupported daemon startup is an environment error. Inspect Go output for skipped tests or empty packages before claiming complete integration coverage.

The generic acceptance test is `PITSTOP_BIN=/absolute/path/to/pitstop sh scripts/testcontainers-smoke.sh`. It runs the pinned Go SDK against a real HTTP service and exercises failure, timeout, cancellation and startup cleanup. Set `PITSTOP_SMOKE_KEEP=1` to retain its disposable fixture and reports. No private example project is used.

# ภาษาไทย

เพิ่ม `"testcontainers": true` เฉพาะ check ที่ใช้ Testcontainers จริง และใช้ `pitstop run --job integration --allow-testcontainers` เพื่ออนุญาต daemon แบบ privileged ในครั้งนั้น ถ้าไม่ได้ให้ flag จะบล็อกทั้ง run ก่อนสร้าง Docker resources หรือรัน checks แม้เลือก unit มาด้วย `doctor` ไม่ได้ทดลองสิทธิ์ privileged ให้

ตรวจ build tags, Makefile, CI, nested modules และเงื่อนไข skip ก่อนเลือกคำสั่ง integration ตัวอย่างใช้ `-tags integration` แต่ไม่ใช่ค่าที่ใช้ได้กับทุกโปรเจกต์ ผล unit ผ่านไม่ได้แปลว่า integration หรือ external tests ผ่านด้วย

แต่ละ check ใช้ DinD, network และ data volume ชั่วคราวของตัวเอง runner ไม่เป็น privileged ไม่มีการ mount host Docker socket/source/credentials และไม่เปิด port ของ daemon บน host ต้องใช้โปรเจกต์/image ที่เชื่อถือได้กับ Docker ที่อนุญาต privileged Linux containers และเข้าถึง public images/dependencies ได้ ไม่ใช้ image cache หรือ credentials จาก host

เวลาเตรียม daemon รวมใน `--timeout`; cleanup มี budget แยก ตรวจ warnings ถ้าเก็บกวาดไม่สำเร็จ รายงานมี `jobs[].testcontainers` สำหรับ image identity, readiness และ diagnostics ตรวจ skipped tests ใน Go logs ด้วยก่อนสรุป coverage ใช้ generic smoke ข้างบนทดสอบจริงโดยไม่แตะโปรเจกต์ตัวอย่างส่วนตัว
