# Docker Checks: Design

## เป้าหมาย

Pitstop เตรียม Linux container ที่มี toolchain ตาม image ของโปรเจกต์ นำ snapshot ของ working tree เข้าไป แล้วรันคำสั่งหรือ shared script โดยไม่รัน checks บน host และไม่ต้องติดตั้ง act สำหรับโหมดนี้ เก็บผลราย job/step, logs และ source identity ได้เหมือนการรันเดิม

ใช้ branch `feature/docker-checks` สำหรับแผนและ implementation ในอนาคต เอกสารนี้เป็น design ประกอบแผน ยังไม่ใช่การเปลี่ยนโค้ด

## ขอบเขตรุ่นแรก

- เพิ่มโหมด `docker` แบบเลือกอย่างชัดเจนด้วย config schema version 2
- config version 1 และการรัน workflow ผ่าน act 0.2.89 ใช้ต่อได้ตามเดิม
- ใช้ Docker image ที่มีอยู่ หรือ pull เมื่อยังไม่มีบน daemon; ไม่สร้าง image ทุกครั้งที่แก้ source
- รองรับ custom image ที่ผู้ใช้ build เองแล้ว เช่น `docker build -t my-checks:dev .`; ตั้ง image นั้นใน config
- เลือก jobs และเรียก steps ตามลำดับ ใน container ใหม่ต่อ job; steps ภายใน job แชร์ filesystem
- แต่ละ step ใช้ `/bin/sh -e` และ cwd `/workspace`; shell variables และ `cd` ไม่ส่งต่อไป step ถัดไป
- เลือก script เดิมได้ เช่น `sh scripts/check.sh`, `npm run check`, `make check` หรือระบุคำสั่งโดยตรง
- `--repo`, `--job`, `--timeout`, `--arch`, `--json` ใช้ API เดิม
- Git repository ต้องมี HEAD ตาม snapshot implementation ปัจจุบัน

รุ่นแรกไม่แปล GitHub workflow YAML เป็น script ไม่รองรับ `uses:`, expression `${{ ... }}`, matrix, needs, services, secret injection, arbitrary Docker flags, host mounts, Docker socket หรือการ build image อัตโนมัติ ไม่ทำ self-hosted runner registration ส่วนเหล่านี้ต้องเป็นงานแยกเมื่อมี use case จริง

## Config Contract

```json
{
  "schemaVersion": 2,
  "engine": "docker",
  "runner": {
    "image": "node:22-bookworm",
    "architecture": "linux/arm64"
  },
  "jobs": ["verify"],
  "checks": [
    {
      "id": "verify",
      "steps": [
        {"id": "install", "name": "Install dependencies", "run": "npm ci"},
        {"id": "check", "name": "Run project checks", "run": "npm run check"}
      ]
    }
  ]
}
```

- `jobs` คือ selection; `checks` คือ definitions สามารถกำหนดหลาย job แล้วเลือก subset ด้วย `--job` ได้
- ID ของ job/step ต้องตรง `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`, unique ตาม scope ของตน ห้ามใช้เป็น path ที่ escape run directory ได้
- มีอย่างน้อยหนึ่ง selected job, หนึ่ง check และหนึ่ง step ต่อ check; `run` ห้ามว่างหรือมี NUL
- step name optional; ถ้าไม่กำหนดใช้ ID
- image ต้องไม่ว่าง ไม่เริ่ม `-`, ไม่มี whitespace/control characters; architecture เป็น `linux/arm64` หรือ `linux/amd64`
- version 2 ต้อง engine=`docker` และไม่มี workflow/event/adapter/runner.label
- version 1 ห้ามระบุ engine/checks; Validate เดิมและ strict unknown-field decoding ใช้ต่อ
- ไม่เพิ่ม `env` ในรุ่นแรก: non-secret environment สามารถเขียนในคำสั่ง เช่น `CI=true npm run check` หรือกำหนดใน image/script
- input command เป็น shell code ที่ผู้ใช้ตั้งใจรันใน container; ส่งเป็นไฟล์ ไม่ประกอบเป็น shell command บน host

## Execution

```text
resolve repo → acquire existing repo lock → load/validate config
→ check Docker endpoint → create run directory → capture snapshot
→ reload captured config and compare original normalized config
→ prepare image once and resolve immutable image ID
→ for each selected job:
    create owned container → copy snapshot + generated step scripts
    → start/attach → inspect terminal state → copy step evidence → remove container
→ remove snapshot → persist metadata/report → return human/JSON result
```

ใช้ Docker CLI ผ่าน Go `exec.CommandContext` และ argv แยก argument ไม่ใช้ host shell ทุก Docker invocation ใช้ endpoint เดียวที่ resolve แล้ว และลบ `DOCKER_CONTEXT` ออกจาก command environment เพื่อไม่ให้ override endpoint เช่นเดียวกับ implementation เดิม ไม่ต้องใช้ Docker SDK ใหม่

นำ source เข้า filesystem ของ container ด้วย `docker cp` ไม่ bind mount working tree หรือ snapshot จึงไม่ต้องให้ daemon เห็น host path และ container เขียนแก้ source บนเครื่องไม่ได้ ไม่ใช้ cache volume รุ่นแรก container ใหม่ต่อ job จึงไม่มี dependency state จาก job ก่อน

container มี run ownership label และชื่อที่ derive จาก validated IDs; `--init`, `--user 0:0`, `--workdir /workspace`, `--entrypoint /bin/sh`, `--no-healthcheck`, `--pull never`, `--platform` ที่ระบุ ไม่เปิด privileged/host network/ports/socket/mounts CLI จะ resolve image ก่อนแล้วสร้าง container ด้วย immutable image ID (ไม่ใช้ tag ที่อาจเปลี่ยนระหว่างรัน)

root user อยู่ใน container เพื่อให้ copied source และผลตรวจเขียนได้; ไม่ให้สิทธิ์ host เพิ่ม image ต้องมี `/bin/sh` และเครื่องมือที่ script ต้องใช้ Docker เป็นการแยก environment ไม่ใช่ sandbox สำหรับโค้ดที่ไม่ไว้ใจ

Image preparation: inspect local image ด้วย platform เป้าหมาย; ถ้ายังไม่มี/ผิด architecture ให้ pull ภายใน 2 นาที แล้ว inspect อีกครั้ง ต้องตรง architecture จึงเริ่ม checks; docker image inspect ใช้ `--format '{{json .}}'` และ parse JSON กำหนด engine.version เป็น daemon version, engine.image เป็น config ref, engine.imageId เป็น resolved image ID และ imageDigest เป็น RepoDigest ถ้ามี (custom image อาจมีแต่ ID)

`doctor` ตรวจ Git/config/Docker daemon และ local image status โดยไม่ pull/build หรือเริ่ม container image ที่ยังไม่มีแสดง `ready` พร้อมข้อความว่าจะ pull ตอน run; shell/toolchain readiness พิสูจน์ตอนรันจริง ถ้า local image platform ไม่ตรง doctor ไม่บอกว่าพร้อมใช้งานโดยไม่อธิบายว่าจะต้อง resolve ใหม่

## Step Evidence

สร้างไฟล์ `/pitstop/step-N.sh` จาก run text และ `/pitstop/run.sh` ที่เรียกแต่ละ step พร้อมบันทึก `/pitstop/results/step-N.exit` หลัง step จบ บันทึก `completed` เฉพาะเมื่อผ่านทุก step ล้ม step ใดหยุด steps หลังจากนั้น; jobs ที่เลือกไว้ยังรันต่อแบบ independent

หลัง container หยุด อ่าน Docker state และ copy results ออกมา ไม่ตีความข้อความที่โปรแกรมพิมพ์บน stdout ว่าเป็น pass/fail หากไม่มี terminal state/evidence ที่ตรงกัน ให้รายงาน outcome unknown แทน passed

- passed: terminal container exit=0, no State.Error/OOM, ทุก selected step มี exit record=0, completed marker และ logs ครบ
- failed: terminal exit ตรงกับ first failed step ที่มี nonzero record; steps ที่ตามหลังเป็น skipped
- timeout: job failed, `TIMEOUT`; cancel: job skipped, `CANCELLED`; ขั้นถัดไป skipped
- daemon/launch/copy/inspect failure หรือ evidence ขาด: skipped/unknown, `ENGINE_OUTCOME_UNKNOWN` (ห้ามตีความ Docker CLI exit 125/126/127 เป็น exit ของ checks โดยลำพัง)
- setup/runtime infrastructure error ที่มี container state Error/OOM: failed category engine พร้อม `CONTAINER_RUNTIME_FAILED` ไม่มี selectedCoverage complete
- failure ที่พิสูจน์แล้วต้องไม่ถูกลบโดย log/report/cleanup error ที่ตามมา; cancellation exit 130 มี precedence สูงสุดตามเดิม
- durationMs ของ step ใช้ null รุ่นแรก; job/run มี duration จริง ไม่มีการใช้เวลารวมเป็นเวลาแต่ละ step

## Timeout / Cleanup / Persistence

- `--timeout` default 20m ต่อ job ครอบ create/copy/start/execute; image preparation มี budget 2m ก่อน job loop
- เมื่อ deadline/cancel อย่าหยุดแค่ docker client: stop owned container ด้วย fresh context, force-remove หากจำเป็น และรอ client process/stream goroutines จบ
- cleanup budget 10s ต่อรอบ; ownership metadata เขียนก่อน create เพื่อเก็บกวาดได้แม้ create ถูก interrupt หลัง daemon สร้าง container
- ลบเฉพาะ containers ที่ label ตรง run ID ตรวจ ownership ซ้ำก่อน rm; `rm --force --volumes` ลบ anonymous volumes ที่ container ของรอบนี้สร้าง ไม่ลบ image/user cache/containers ของคนอื่น ไม่ใช้ prune
- temporary scripts และ snapshot ลบหลัง cleanup; logs, captured config, manifest, command argv, container/image identity, step evidence, metadata และ report เก็บไว้ใต้ user cache
- report persistence ใช้ `writeAtomic` เดิม; ไม่มีการส่ง snapshot/private source ขึ้น GitHub

## CLI / Report Compatibility

- run config v2 ไม่เรียก `act --version`, workflow.Inspect หรือ PrepareIntegration
- `--force-integration` กับ docker mode เป็น CONFIG_INVALID ก่อนเตรียม image
- list แสดง jobs ที่กำหนดใน checks และ selection โดยไม่เรียก Docker/act
- doctor ไม่เรียก act สำหรับ config v2; กรณี config invalid ยังคงแสดง actionable config error
- JSON envelope schemaVersion ยังคง 1; config schemaVersion=2 เป็นคนละ version space
- RunReport ใช้ jobs/steps/engine เดิม เพิ่ม engine.imageId แบบ optional
- docker mode: selectedCoverage complete เมื่อ selected jobs ได้ผลตรวจ terminal ที่เชื่อถือได้ (รวม failure ปกติ), partial เมื่อ blocked/unknown/cancel/timeout; workflowCoverage=`not_applicable`
- เพิ่ม ListResult.engine แบบ optional เพื่อให้ human output ใช้ `checks: .pitstop.json` แทน `workflow:` ใน docker mode; v1 JSON/output คง shape เดิมด้วย omitempty
- code 0=passed, 1=failed/timeout, 2=invalid/blocked/unknown/persistence/cleanup error เมื่อไม่มี failure, 130=cancel
- Human output ต้องบอก engine=docker และไม่กล่าวว่าได้รัน GitHub workflow; JSON mode progress เป็น stderr และ stdout มี envelope เดียว

## Acceptance

1. Git + Docker เพียงพอสำหรับ Docker checks; host ไม่ต้องมี act, Node หรือ Go ที่ checks ใช้
2. working tree เดิมไม่ถูกแก้ แม้ script เขียนไฟล์; dirty/new/deleted inputs ตรง snapshot เดิม
3. dependencies ที่ step แรกสร้างใช้ได้ใน step ถัดไปของ job เดียวกัน แต่ไม่รั่วไป job ถัดไป
4. ผล success/failure/unknown/cancel/timeout และ step evidence ตรวจสอบย้อนกลับได้
5. ทุก outcome ล้างเฉพาะทรัพยากรที่ Pitstop รอบนี้สร้าง; cleanup/report failure ไม่ถูกซ่อน
6. v1 act tests, CLI help และ JSON smoke เดิมผ่าน
7. ทดสอบ Docker จริงบน macOS ARM64 + Linux ARM64 runner; AMD64 เป็น configurable แต่ไม่อ้าง runtime validation จนกว่าจะทดสอบ

## Sources for Docker mechanics

- [create: workdir, entrypoint, platform, ownership label](https://docs.docker.com/reference/cli/docker/container/create/)
- [cp: copy to/from stopped containers and directory contents](https://docs.docker.com/reference/cli/docker/container/cp/)
- [image inspect](https://docs.docker.com/reference/cli/docker/image/inspect/)
