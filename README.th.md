# Pitstop

[English](README.md) | **ไทย**

**รันคำสั่งตรวจโค้ดใน Docker environment ที่แยกจากเครื่อง ก่อน commit หรือ push**

Pitstop เก็บ snapshot ของ working tree ปัจจุบัน แล้ว copy เข้า Linux container ใหม่ต่อ check ที่เลือก เก็บ log, หลักฐานแต่ละ step และ JSON report ไว้ให้ตรวจย้อนหลัง ระบบไม่อ่านหรือรัน GitHub Actions workflow และไม่ต้องติดตั้ง `act`

## เริ่มใช้งาน

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
TARGET_REPO="/absolute/path/to/your/repo"
"$PITSTOP_BIN" -h
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --json
```

เครื่องที่รัน Pitstop ต้องมี Git, Docker และ Docker daemon ที่เชื่อมต่อได้ ส่วนเครื่องมือของโปรเจกต์ต้องอยู่ใน image ที่ตั้งค่าไว้ จึงไม่ต้องติดตั้ง Node หรือ Go บน host ใช้ตัวอย่าง [docker-react.json](examples/docker-react.json) หรือ [docker-go.json](examples/docker-go.json) ได้

## ตั้งค่า

`.pitstop.json` ใช้ schema version 2:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "node:22-bookworm", "architecture": "linux/arm64"},
  "jobs": ["verify"],
  "checks": [{"id":"verify","steps":[
    {"id":"install","run":"npm ci"},
    {"id":"check","run":"npm run check"}
  ]}]
}
```

แต่ละ step ใช้ `/bin/sh -e` และทำงานที่ `/workspace` ไฟล์ที่สร้างใน step ก่อนหน้าจะใช้ต่อได้ใน check เดียวกัน แต่ check ถัดไปได้ container ใหม่ สามารถเรียก script เดิม เช่น `sh scripts/check.sh` ได้ แต่รุ่นนี้ไม่แปล `uses:`, expression, services, matrix, cache volume, secret หรือสร้าง image อัตโนมัติ หาก image สาธารณะไม่มีเครื่องมือ ให้ build image เองครั้งเดียว:

```dockerfile
FROM node:22-bookworm
```

```sh
docker build -t my-checks:dev -f Dockerfile.checks .
go run ./cmd/pitstop run --repo /absolute/path/to/repo
```

config schema version 1 จะถูกปฏิเสธพร้อมคำแนะนำย้ายไป v2 ให้ย้ายคำสั่งจาก `run` ใน workflow มาไว้ใน `checks`, ให้ Pitstop จัดการ snapshot แทน checkout และใช้ image แทน setup action หาก GitHub CI ต้องตรวจเหมือนกัน ให้ workflow เรียก project script เดียวกัน

รายงานเก็บ source identity, image ID, arguments, log, exit record และหลักฐาน cleanup ไว้ใน user cache ค่า `workflowCoverage` เป็น `not_applicable` เพราะไม่ได้ตีความ workflow เมื่อ timeout หรือกด Ctrl+C ระบบจะหยุดและลบเฉพาะ container ของ run นั้น และไม่แก้ source repository

ดู [คู่มือ](docs/user-guide.md), [ข้อกำหนด](docs/requirements.md) และ [หลักฐานเดิม](docs/evidence/final-validation.md)

หากต้องการตรึง toolchain ให้ตั้ง `runner.image` เป็น image digest เช่น `node@sha256:<digest>` เพราะ tag อาจเปลี่ยนได้ แต่ละ run จะบันทึก image ID ที่ใช้ ส่วน custom image ต้อง build เองก่อนรัน Pitstop
