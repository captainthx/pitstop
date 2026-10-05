# Pitstop

[English](README.md) | **ไทย**

**รันงานตรวจโค้ดจาก GitHub Actions บนเครื่องตัวเอง ก่อนส่งขึ้น CI**

Pitstop ใช้ workflow เดิมของโปรเจกต์ เลือกเฉพาะ job ที่กำหนด แล้วรันผ่าน `act` และ Docker โดยทดสอบสำเนาของโค้ดที่อยู่ใน working tree รวมถึงงานที่ยังไม่ได้ commit

## เริ่มตรงนี้

| ต้องการอ่านเรื่องอะไร | เอกสาร |
|---|---|
| เริ่มใช้ ตั้งค่า และดูผลการรัน | [คู่มือใช้งาน](docs/user-guide.md) |
| เครื่องต้องมีอะไร ฟีเจอร์ที่รองรับ และข้อกำหนดของ MVP | [ความต้องการและขอบเขต](docs/requirements.md) |
| ผลทดสอบกับ React, Go และ integration จริง | [ผล validation](docs/evidence/final-validation.md) |

## เริ่มใช้แบบสั้น

รันจากโฟลเดอร์ source ของ Pitstop:

```sh
go build -trimpath -o pitstop ./cmd/pitstop
PITSTOP_BIN="$PWD/pitstop"
TARGET_REPO="/absolute/path/to/your/repo"
```

สร้าง `.pitstop.json` ในโปรเจกต์เป้าหมายตาม [ตัวอย่างในคู่มือ](docs/user-guide.md#2-ตั้งค่าโปรเจกต์ที่จะทดสอบ) แล้วรัน:

```sh
"$PITSTOP_BIN" doctor --repo "$TARGET_REPO"
"$PITSTOP_BIN" list --repo "$TARGET_REPO"
"$PITSTOP_BIN" run --repo "$TARGET_REPO"
```

เครื่องต้องมี Git, Docker daemon ที่ใช้งานได้ และ **act 0.2.89** ส่วน Go **1.26.5 ขึ้นไป** จำเป็นเมื่อ build Pitstop จาก source

## ฟีเจอร์หลัก

- เลือก job จาก workflow เดิม เช่น lint, test หรือ verify และรันทีละ job
- ทดสอบโค้ดปัจจุบัน รวมไฟล์ที่แก้ไว้แต่ยังไม่ได้ commit
- ดูผลแบบข้อความ หรือ JSON สำหรับ script และ AI agent
- เก็บ report, log และข้อมูลของ source ที่ใช้รัน เพื่อย้อนตรวจได้
- กำหนด timeout, หยุดด้วย Ctrl+C และป้องกันการรันซ้ำใน repo เดียวกัน
- มีโหมด integration เฉพาะสำหรับ `recruit100-service`

รุ่นนี้มีผลทดสอบบน macOS ARM64 กับ Linux ARM64 runner งานที่มีเงื่อนไขหรือความสามารถที่ยังไม่รองรับจะแสดงว่า `blocked` และการผ่านบนเครื่องยังต้องผ่าน required checks บน GitHub ตามปกติ

## CI ของ Pitstop

[GitHub repository](https://github.com/captainthx/pitstop) · [GitHub Actions](https://github.com/captainthx/pitstop/actions)

CI ทำงานเมื่อเปิดหรืออัปเดต PR ที่เข้า `main`, เมื่อ push/merge เข้า `main` และเมื่อสั่งรันด้วยตนเอง ตรวจ formatting, dependencies, vet, race tests, build และ JSON smoke test ของ CLI โดยใช้เวอร์ชัน Go จาก `go.mod`

ผลที่ตรวจคือโค้ดของ Pitstop ส่วนหลักฐานการรัน workflow ของ React/Go service จริงอยู่ใน [ผล validation](docs/evidence/final-validation.md)
