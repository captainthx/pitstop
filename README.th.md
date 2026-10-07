# Pitstop

[English](README.md) | **ไทย**

รัน lint, test และ build ของโปรเจกต์ใน Linux container ก่อน commit หรือ push โดยใช้ไฟล์ที่กำลังแก้บนเครื่อง แล้วเก็บ log กับรายงานผลไว้ให้ตรวจย้อนหลัง

**เริ่มต้น:** build CLI → คัดลอกตัวอย่างเป็น `.pitstop.json` ใน repo เป้าหมาย → แก้ image/คำสั่ง → `pitstop doctor` → `pitstop run`

## Pitstop ทำอะไรให้

คุณกำหนด **environment** ผ่าน Docker image และ **คำสั่งตรวจ** ผ่าน config ส่วน Pitstop จัดการขั้นตอนที่เหลือ:

```text
working tree → snapshot → container ใหม่ต่อ check → รัน steps → เก็บผล → cleanup
```

- ตรวจไฟล์ที่แก้แล้วแต่ยังไม่ commit รวมถึงไฟล์ใหม่ที่ไม่ถูก ignore และไฟล์ที่ลบ
- รันเครื่องมือของโปรเจกต์ใน container; step ที่เขียนไฟล์จะไม่แก้ repo ต้นฉบับ
- รัน steps ตามลำดับ แชร์ไฟล์ภายใน check เดียวกัน และแยก filesystem ของแต่ละ check
- จัดการ timeout, Ctrl+C, log และ JSON report พร้อมหลักฐานแต่ละ step

CLI นี้รันคำสั่งใน `.pitstop.json` โดยตรง ดูขอบเขตการรองรับใน [คู่มือ](docs/user-guide.md#ขอบเขตที่รองรับ)

## 1. ติดตั้ง CLI จาก source

ใน checkout ของ Pitstop ใช้ Git, Go **1.26.5** ตาม `go.mod` และ Docker CLI/daemon สำหรับรัน checks:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop -h
```

เพิ่มบรรทัด `export PATH="$HOME/.local/bin:$PATH"` ในไฟล์ตั้งค่า shell หากต้องการให้ใช้ได้ใน terminal ครั้งถัดไป ปัจจุบันขั้นตอนที่รองรับคือ build จาก source; ยังไม่มี installer link หรือคำสั่งติดตั้ง release ที่จัดทำไว้

Go บน host ใช้ build ตัว Pitstop เท่านั้น เครื่องมือที่ check ใช้ เช่น Node, Java หรือ Go ต้องอยู่ใน Docker image [รายละเอียดสิ่งที่ต้องเตรียม](docs/user-guide.md#สิ่งที่ต้องเตรียม)

## 2. สร้าง config ครั้งเดียวต่อ repo

**ตอนนี้ยังไม่มี `pitstop init`** ต้องสร้าง `.pitstop.json` ที่ root ของ repo แต่เริ่มจากตัวอย่างได้ ไม่ต้องเขียนใหม่ทั้งหมด

รันจาก checkout ของ Pitstop โดยเปลี่ยน path เป็น repo ที่ต้องการตรวจ:

```sh
PITSTOP_SOURCE="$PWD"
TARGET_REPO="/absolute/path/to/your/repo"
cp "$PITSTOP_SOURCE/examples/docker-react.json" "$TARGET_REPO/.pitstop.json"
```

ถ้ามี config อยู่แล้ว ให้แก้ไฟล์เดิมแทนการคัดลอกทับ สำหรับ Go เปลี่ยนชื่อไฟล์เป็น `examples/docker-go.json` จากนั้นแก้ `.pitstop.json`:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "node:22-bookworm", "architecture": "linux/arm64"},
  "jobs": ["verify"],
  "checks": [{"id": "verify", "steps": [
    {"id": "install", "name": "Install dependencies", "run": "npm ci"},
    {"id": "check", "name": "Run project checks", "run": "npm run check"}
  ]}]
}
```

| ส่วน | ต้องตั้งอะไร |
| --- | --- |
| `runner.image` | image ที่มีเครื่องมือครบ เช่น Node สำหรับ `npm` |
| `runner.architecture` | `linux/arm64` หรือ `linux/amd64` ให้ตรงกับ environment ที่จะใช้ |
| `checks` | กลุ่มคำสั่งตรวจ แต่ละกลุ่มได้ container ใหม่ |
| `checks[].steps` | คำสั่งในกลุ่ม รันตามลำดับ |
| `jobs` | IDs ของ checks ที่จะรันโดยปริยาย และลำดับการรัน |

ตัวอย่าง Node ต้องมี `package-lock.json` และ script `check` ใน `package.json` ถ้าโปรเจกต์ใช้คำสั่งอื่น ให้แก้ `run` เช่น `npm test` หรือ `sh scripts/check.sh` ตัวอย่างไม่สร้าง project scripts ให้

ตรวจ architecture ของ Docker daemon ด้วย `docker version --format '{{.Server.Arch}}'`: `arm64`/`aarch64` ใช้ `linux/arm64`; `amd64`/`x86_64` ใช้ `linux/amd64` repo ต้องมี commit อย่างน้อยหนึ่งรายการก่อน `run` แนะนำ commit config และ shared scripts เพื่อให้ทีมใช้เหมือนกัน

## 3. ตรวจความพร้อม แล้วรัน

```sh
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO"
pitstop run --repo "$TARGET_REPO"
```

`doctor` ตรวจ config, Docker และ image โดยไม่ pull หรือเปิด container ส่วน `run` จะ pull image เมื่อจำเป็น แล้วรัน checks หาก image ยังไม่มี `doctor` อาจรายงานพร้อมพร้อมข้อความว่าจะ pull ตอน run; ไม่ได้ยืนยันว่า image มีคำสั่งของโปรเจกต์ครบ

ตัวเลือกที่ใช้บ่อย:

```sh
pitstop run --repo "$TARGET_REPO" --job verify
pitstop run --repo "$TARGET_REPO" --timeout 10m
pitstop run --repo "$TARGET_REPO" --json > result.json 2> progress.log
pitstop run -h
```

`--job` เลือกจาก IDs ใน `jobs` เท่านั้น ค่า timeout เริ่มต้น **20 นาทีต่อ check** กด Ctrl+C เพื่อยกเลิก ระบบพยายามหยุดและลบเฉพาะ container ของ run นั้น หาก cleanup ไม่สำเร็จให้ดู warning ใน report

## 4. อ่านผล

เมื่อเริ่มรัน checks แล้ว ผลจะมี `reportPath` และ log ของแต่ละ check รายงานเก็บใน user cache ส่วน snapshot และ scripts ชั่วคราวถูกลบ หาก config หรือ environment ผิดตั้งแต่ก่อนเริ่ม อาจได้ error โดยไม่มี run report

| Exit code | ความหมาย |
| --- | --- |
| `0` | ทุก check ที่เลือกผ่าน และไม่มีปัญหาที่ทำให้ผลไม่สมบูรณ์ |
| `1` | มี check ล้มเหลว รวม timeout หรือ container runtime failure |
| `2` | config/environment ไม่พร้อม ผลไม่ทราบแน่ชัด หรือมีปัญหา log/report/cleanup โดยไม่มี failed check |
| `130` | ยกเลิกการทำงาน มีลำดับความสำคัญสูงสุด |

ดู [วิธีอ่าน report และแก้ปัญหา](docs/user-guide.md#อ่าน-report-และ-log) ก่อนสรุปว่าทุก check ผ่าน

## อ่านต่อ

- [คู่มือครบ: config, shared scripts, custom image, commands, reports และ troubleshooting](docs/user-guide.md)
- [ขอบเขตและข้อกำหนด](docs/requirements.md)
- [สารบัญเอกสาร](docs/README.md) และ [หลักฐานทดสอบ Docker ปัจจุบัน](docs/evidence/docker-checks-validation.md)
