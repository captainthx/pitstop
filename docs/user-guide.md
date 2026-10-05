# คู่มือใช้งาน Pitstop

[English](user-guide.en.md) | **ไทย**

**ใช้งานครั้งแรก: build → ตั้งค่า → ตรวจความพร้อม → รัน → อ่านผล**

Pitstop เหมาะกับการตรวจโค้ดก่อน commit หรือ push เช่น lint, unit test และ build ที่เขียนไว้ใน GitHub Actions อยู่แล้ว คำว่า **job** ในคู่มือนี้หมายถึงรายการใต้ `jobs:` ในไฟล์ workflow

[กลับหน้าแรก](../README.th.md) · [ความต้องการและขอบเขต](requirements.md)

## 1. เตรียมเครื่องและ build

เครื่องต้องมี Git, act **0.2.89** และ Docker daemon ที่เปิดใช้งานอยู่ ตรวจได้ด้วย:

```sh
git --version
act --version
docker version
```

หาก build จาก source ให้ใช้ Go **1.26.5 ขึ้นไป** แล้วรันคำสั่งนี้จากโฟลเดอร์ Pitstop:

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
"$PITSTOP_BIN" version
```

`PITSTOP_BIN` เก็บ path ของโปรแกรมไว้ เพื่อให้ใช้กับโปรเจกต์อื่นได้ หากใช้ binary ที่ build ไว้แล้ว ไม่ต้องมี Go เพื่อเปิด Pitstop แต่ job ของโปรเจกต์ยังต้องมีเครื่องมือตามที่ workflow กำหนด

binary ที่ส่งมอบใน workspace นี้อยู่ที่ `.pitstop-work/bin/pitstop` หากต้องการใช้ไฟล์นั้น ให้กำหนด `PITSTOP_BIN="$PWD/.pitstop-work/bin/pitstop"` แทน โดยเริ่มจากโฟลเดอร์ Pitstop ไฟล์นี้ถูก ignore และไม่ติดไปกับ GitHub clone; หลัง clone ให้ build จาก source

## 2. ตั้งค่าโปรเจกต์ที่จะทดสอบ

โปรเจกต์เป้าหมายต้องเป็น Git repository ที่มีอย่างน้อยหนึ่ง commit และมี workflow อยู่แล้ว ตั้ง path เป็น **โฟลเดอร์รากของ repo**:

```sh
TARGET_REPO="/absolute/path/to/your/repo"
```

สร้างไฟล์ `.pitstop.json` ที่รากของโปรเจกต์ หากมีอยู่แล้วให้แก้ไฟล์เดิม ตัวอย่างสำหรับ workflow ที่มี job ชื่อ `verify`:

```json
{
  "schemaVersion": 1,
  "workflow": ".github/workflows/ci.yml",
  "event": "pull_request",
  "jobs": ["verify"],
  "runner": {
    "label": "ubuntu-latest",
    "image": "catthehacker/ubuntu:act-latest",
    "architecture": "linux/arm64"
  }
}
```

ปรับ `workflow`, `jobs` และ `runner` ให้ตรงกับโปรเจกต์จริง โดย `jobs` ใช้ **job ID** ใต้ `jobs:` ไม่ใช่ข้อความใน `name:`

| ค่า | ใส่อะไร |
|---|---|
| `schemaVersion` | `1` |
| `workflow` | path ของไฟล์ `.yml` หรือ `.yaml` ภายใน repo |
| `event` | `pull_request` |
| `jobs` | job ID ที่ต้องการรัน อย่างน้อยหนึ่งรายการ และไม่ซ้ำกัน |
| `runner.label` | ต้องตรงกับ `runs-on` ของ job เช่น `ubuntu-latest` |
| `runner.image` | Docker image ที่ใช้รันงาน |
| `runner.architecture` | `linux/arm64` หรือ `linux/amd64` |
| `adapter` | เว้นได้; ใช้ `recruit100-service-integration-v1` เฉพาะ service profile ที่รองรับ |

ไฟล์ต้องเป็น JSON ที่ถูกต้อง ไม่มี comment หรือ field ที่ไม่รู้จัก และ workflow ต้องประกาศ event `pull_request` ไว้ด้วย

มี profile ตัวอย่างสำหรับ [recruit100-react](../examples/recruit100-react.json) และ [recruit100-service](../examples/recruit100-service.json) ตัวอย่างเหล่านี้ผูกกับ job ของแต่ละโปรเจกต์ จึงควรตรวจชื่อ job ก่อนนำไปใช้กับ repo อื่น

## 3. ตรวจความพร้อม

```sh
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
```

- `doctor` ตรวจ repo, config, workflow, เวอร์ชัน act และการเชื่อมต่อ Docker หากรายการที่เลือกยังรันไม่ได้ จะได้ `ready=false` และ exit code `2`
- `list` แสดง job ทั้งหมดใน workflow พร้อมรายการที่เลือกและเหตุผลของงานที่ถูกบล็อก

`doctor` เป็นการตรวจเบื้องต้น ความพร้อมของ dependency และผล lint/test จะทราบเมื่อรันจริง ใน service profile ที่เลือก integration ไว้ `doctor` อาจรายงานไม่พร้อม เพราะ integration ถูกบล็อกในโหมดปกติ แต่ยังเลือกเฉพาะ lint/test มารันได้

## 4. รันงาน

รันทุก job ที่กำหนดใน `.pitstop.json`:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO"
```

เลือกบาง job จากรายการที่กำหนดไว้ เช่น service profile:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --job lint --job test
```

กำหนดเวลาสูงสุดต่อ job:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --timeout 10m
```

ค่าเริ่มต้นคือ **20 นาทีต่อ job** ไม่ใช่ทั้ง run งานรันตามลำดับที่เลือก หาก job หนึ่ง fail หรือถูกบล็อก Pitstop ยังรัน job อิสระรายการถัดไป กด **Ctrl+C** เพื่อยกเลิกการรัน

`--job` เลือกได้เฉพาะ ID ที่มีใน `.pitstop.json` หากจะเพิ่ม job ใหม่ ให้แก้ config ก่อน

เปลี่ยน architecture สำหรับการรันครั้งนั้นได้ เช่น `--arch linux/amd64` แต่ต้องใช้ image และ Docker environment ที่รองรับด้วย รุ่นนี้มีหลักฐาน runtime สำหรับ ARM64 ส่วน AMD64 ยังไม่ได้ทดสอบจริง

## 5. อ่านผล

| สถานะ job | ความหมาย |
|---|---|
| `passed` | act จบสำเร็จและมีหลักฐานของ job ที่เลือก |
| `failed` | engine/job จบด้วย failure หรือหมดเวลา |
| `blocked` | ตรวจพบก่อนรันว่างานใช้ความสามารถที่ยังไม่รองรับ |
| `skipped` | งานไม่ได้รันจนยืนยันผลได้ เช่น ยกเลิก, launch ไม่ได้ หรือ outcome ไม่ชัดเจน |

| Exit code | ทำอะไรต่อ |
|---|---|
| `0` | job ที่เลือกผ่านครบภายใต้เงื่อนไขที่รองรับ |
| `1` | เปิด log เพื่อดู failure หรือ timeout |
| `2` | อ่านเหตุผลของ config, งานที่ถูกบล็อก หรือผลที่ยืนยันไม่ได้ |
| `130` | การรันถูกยกเลิก |

หากมีทั้ง failed และ blocked จะได้ exit `1` หากยกเลิกจะได้ `130` แม้มี job fail ไปแล้ว

**Coverage บอกว่าตรวจครบแค่ไหน ไม่ได้แปลว่าผ่านทุกอย่าง:**

- `selectedCoverage`: รัน job ที่เลือกได้ครบหรือไม่ (`complete` / `partial`) งานที่จบด้วย failure ก็อาจนับว่ารันครบได้
- `workflowCoverage`: ครอบคลุม workflow ต้นฉบับครบหรือไม่ การเลือกบาง job หรือใช้ forced integration จะเป็น `partial`

## 6. ใช้ JSON กับ script หรือ AI agent

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --json > result.json 2> progress.log
```

`result.json` มี JSON หนึ่งเอกสาร ส่วนความคืบหน้าอยู่ใน `progress.log` คำสั่ง `doctor`, `list` และ `version` ก็ใช้ `--json` ได้

อ่าน field เหล่านี้ก่อน:

| Field | ใช้ดูอะไร |
|---|---|
| `exitCode` | ผลรวมของคำสั่ง |
| `error` | ข้อผิดพลาดระดับคำสั่ง เมื่อมี |
| `result.jobs` | ผลและเหตุผลของแต่ละ job |
| `result.selectedCoverage` / `result.workflowCoverage` | ความครบของการตรวจ |
| `result.adjustments` | สิ่งที่ adapter ปรับจาก workflow ต้นฉบับ |
| `result.warnings` | ข้อจำกัดหรือปัญหาเพิ่มเติมของ run |
| `result.reportPath` | path ของ report ที่บันทึกไว้ |

กรณีผิดพลาดก่อนสร้าง run อาจไม่มี `result` ให้ดู `error` แทน ค่า `null` หมายถึงไม่มีข้อมูล ส่วน `steps: []` ไม่ได้ยืนยันว่าไม่มี test แต่หมายถึงไม่มีรายละเอียด step ที่รายงานได้อย่างน่าเชื่อถือ

## 7. รัน integration ของ recruit100-service

ใช้ profile ที่มี `adapter: "recruit100-service-integration-v1"` และเลือก `integration` ไว้ใน config:

```sh
"$PITSTOP_BIN" run --repo "$TARGET_REPO" --job integration --force-integration
```

โหมดนี้ตรวจรูปแบบ job ที่กำหนดไว้ แล้วนำ `needs` และ `if` ของ integration ออกจาก **สำเนา workflow** เพื่อรัน integration โดยตรง ตัว workflow ใน repo ต้นฉบับไม่ได้ถูกแก้

ต้องใช้ Docker endpoint แบบ local Unix socket เช่น environment ที่ทดสอบกับ OrbStack โหมดนี้ส่ง Docker socket ให้ integration runner เพื่อให้ Testcontainers สร้าง container ของฐานข้อมูลได้

ผลสำเร็จยังมี `workflowCoverage=partial` เพราะข้ามเงื่อนไขเดิม พร้อมบันทึกการปรับใน `adjustments` หากรูปแบบ workflow เปลี่ยนจนไม่ตรงกับ adapter จะหยุดด้วย `ADAPTER_SHAPE_CHANGED` โหมดนี้ไม่ใช่ adapter ทั่วไปสำหรับ integration ทุกโปรเจกต์

## 8. โค้ดและไฟล์ใดถูกนำไปทดสอบ

Pitstop สร้างสำเนาแยกจาก repo ต้นฉบับ โดยใช้ไฟล์ใน working tree ขณะรัน:

- ไฟล์ที่แก้แต่ยังไม่ได้ commit ถูกนำไปด้วย; หาก staged กับ working tree ต่างกัน จะใช้ working tree
- ไฟล์ที่ลบแล้วจะไม่อยู่ในสำเนา
- ไฟล์ใหม่ที่ไม่ถูก ignore ถูกนำไปด้วย
- ไฟล์ ignored ไม่ถูกนำไป; `.env*`, `.actrc`, `.secrets`, `.vars`, `.input` ที่ยังไม่ tracked ถูกกันออก
- หากไฟล์เหล่านี้ tracked อยู่แล้ว จะยังเป็นส่วนของ source เช่น tracked `.env`

ไม่ต้อง commit ทุกครั้งก่อนใช้ Pitstop แต่ repo ต้องมี commit เริ่มต้นแล้ว และหาก source เปลี่ยนระหว่าง capture จะปฏิเสธ snapshot แทนการใช้สำเนาที่ไม่ตรงกัน

## 9. Report และ log อยู่ที่ไหน

ดู path จริงจาก `result.reportPath` หรือข้อความสรุปท้าย run โดยเก็บใต้ user cache directory:

```text
<user-cache>/pitstop/runs/<run-id>/
```

ในแต่ละ run มี `report.json`, log ของ job, `manifest.txt`, สำเนา workflow, hash และ argv ที่ใช้รัน บน macOS โดยทั่วไปอยู่ใต้ `~/Library/Caches/pitstop/runs/`

เมื่อจบงานจะลบ source snapshot และ HOME ชั่วคราว ส่วน report/log เก็บไว้จนกว่าจะลบเอง สามารถลบโฟลเดอร์ของ run ที่จบแล้วและไม่ต้องการเก็บหลักฐานได้

## 10. เมื่อรันไม่ได้

| ปัญหา | ตรวจอะไร |
|---|---|
| `TOOL_MISSING` | ติดตั้ง Git, act หรือ Docker ที่ขาดให้เรียบร้อย แล้วตรวจ PATH |
| `ACT_VERSION_UNSUPPORTED` | ใช้ act 0.2.89 |
| `DOCKER_UNAVAILABLE` | เปิด Docker daemon และตรวจ Docker context/endpoint ที่ใช้งาน |
| `CONFIG_INVALID` | ตรวจ JSON, path workflow, job ID และ runner |
| `WORKFLOW_UNSUPPORTED` | ดู `list` และเหตุผลของ job; เลือกเฉพาะงานที่รองรับ |
| `SNAPSHOT_CHANGED` | หยุดการแก้ไฟล์ระหว่าง capture, แก้ merge ค้าง และตรวจ submodule/symlink |
| `RUN_ALREADY_ACTIVE` | รอหรือยกเลิก run เดิมของ repo นี้ก่อน |
| `TIMEOUT` | เปิด log; เพิ่ม `--timeout` หากงานต้องใช้เวลานานตามปกติ |
| `ENGINE_OUTCOME_UNKNOWN` | เปิด raw log; outcome ที่ไม่ชัดเจนยังไม่นับว่าผ่าน |
| `INTEGRATION_RUNTIME_UNSUPPORTED` | ตรวจเงื่อนไข local Unix Docker endpoint ของ service adapter |
| `ADAPTER_SHAPE_CHANGED` | workflow ไม่ตรงกับรูปแบบ integration ที่ adapter รองรับ |
| `REPORT_WRITE_FAILED` / `CLEANUP_INCOMPLETE` | ตรวจพื้นที่และสิทธิ์ใน cache หรือ Docker; ผลที่เกิดขึ้นแล้วอาจยังอยู่ใน JSON ให้ตรวจด้วย |

ดูรายการความสามารถที่รองรับและข้อจำกัดทั้งหมดใน [ความต้องการและขอบเขต](requirements.md)
