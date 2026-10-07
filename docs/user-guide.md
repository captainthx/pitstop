# คู่มือใช้งาน

## สิ่งที่ต้องเตรียม

ติดตั้ง Git และ Docker แล้วเปิด daemon ตรวจด้วย `git --version` และ `docker version` Build Pitstop ด้วย Go 1.26.5 หรือใช้ binary ที่ build แล้ว เครื่อง host ไม่ต้องมี Node, Go หรือ toolchain ของโปรเจกต์

## ตั้งค่าครั้งเดียวต่อ repo

คัดลอก [docker-react.json](../examples/docker-react.json) หรือ [docker-go.json](../examples/docker-go.json) เป็น `.pitstop.json` แล้วตั้ง image, architecture, jobs และ steps ตามลำดับ image ต้องมีคำสั่งทั้งหมดที่ step เรียก สามารถเรียก shared script ของโปรเจกต์ เช่น `sh scripts/check.sh` และให้ GitHub CI เรียก script เดียวกันหลัง checkout/setup ตามปกติ

config schema version 1 จะไม่ถูกแปลงอัตโนมัติ ให้ลบ field workflow/event/adapter/runner label, เปลี่ยนเป็น schema 2, เลือก image และย้ายคำสั่ง `run` ที่ต้องการจาก workflow มาเป็น step `checks` `actions/checkout` ถูกแทนด้วย source snapshot และ setup action ถูกแทนด้วยเครื่องมือใน image รุ่นแรกยังไม่มีการแปล `uses:`, expression, services, matrix, needs, secrets หรือ cache volume

## คำสั่ง

```sh
pitstop doctor --repo /absolute/path/to/repo
pitstop list --repo /absolute/path/to/repo
pitstop run --repo /absolute/path/to/repo --job verify --timeout 20m
pitstop run --repo /absolute/path/to/repo --json > result.json 2> progress.log
```

`doctor` ตรวจ Git, config และ Docker โดยไม่ pull image หรือเปิด container `list` อ่าน `.pitstop.json` เท่านั้น `run` เตรียม image, copy snapshot เข้า container ใหม่ต่อ check และลบ container ที่มี label ของ run หลังเก็บ evidence แล้ว `--arch` เปลี่ยน Linux architecture สำหรับ run เดียว ส่วน `--force-integration` ถูกถอดออกและเป็น unknown flag

Step ใช้ `/bin/sh -e` ที่ `/workspace` ไฟล์อยู่ต่อได้ใน check เดียวกัน source repository ไม่ถูก bind mount และไม่ถูกแก้เมื่อ step เขียนไฟล์ image จะถูก inspect และ pull เมื่อไม่มีหรือ platform ไม่ตรง โดยมีเวลาเตรียมสองนาที ค่า timeout ต่อ check เริ่มต้นยี่สิบนาที เมื่อ timeout หรือกด Ctrl+C ระบบจะหยุด container รอ Docker client/stream และ cleanup ตามเวลาที่กำหนด

JSON envelope ยังเป็น schema version 1 `result.engine.name` เป็น `docker`, `result.engine.imageId` คือ image ID และ `result.workflowCoverage` เป็น `not_applicable` ต้องมี exit record และ terminal state จึงจะรายงาน passed ข้อความที่ดูเหมือน success อย่างเดียวไม่ใช่หลักฐาน รายงานและ log ถูกเก็บใน user cache ส่วน snapshot และ script ชั่วคราวถูกลบหลังจบ

ถ้าต้องใช้ image เฉพาะ ให้ build ครั้งเดียวแล้วตั้งชื่อใน config:

```dockerfile
FROM node:22-bookworm
```

```sh
docker build -t my-checks:dev -f Dockerfile.checks .
pitstop doctor --repo /absolute/path/to/repo
pitstop run --repo /absolute/path/to/repo
```

รุ่นแรกยังไม่มี services, cache volume, auto-build image หรือแปลง workflow เป็น checks ดู [ข้อกำหนด](requirements.md) และ [หลักฐานเดิม](evidence/final-validation.md)

หากต้องการตรึง toolchain ให้ตั้ง `runner.image` เป็น image digest เช่น `node@sha256:<digest>` เพราะ tag อาจเปลี่ยนได้ แต่ละ run จะบันทึก image ID ที่ใช้ ส่วน custom image ต้อง build เองก่อนรัน Pitstop
