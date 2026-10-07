> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../README.th.md) สำหรับการใช้งาน Docker checks.

# Pitstop — ชุดสเปกและแผน MVP 1

ชุดนี้ใช้ส่งต่อให้คนหรือ AI agent เริ่ม implement โดยไม่ต้องอ่านประวัติแชต ไม่มี product code และยังไม่มี CLI pitstop ที่ติดตั้งได้

## อ่านตามลำดับ

1. [Context / handoff](pitstop-context.md): ที่มา สิ่งที่ผู้ใช้เลือก สถานะเครื่องและ repos ผลทดลอง และ prompt เริ่มงานต่อ
2. [Specification](pitstop-spec.md): ขอบเขต CLI/config/report, snapshot, act isolation, integration adapter, ข้อจำกัดและ acceptance criteria
3. [Implementation plan](pitstop-implementation-plan.md): 8 tasks พร้อมไฟล์ interfaces tests และ verification
4. [ผลทดลอง act](../evidence/act-react-ci-report.md): คำสั่งจริงและผล React CI; log/JSON results อยู่ในชุดเดียวกัน

## สิ่งที่พิสูจน์แล้ว

React CI job verify ผ่านครบโดยใช้ workflow เดิม ไม่ส่ง secret ไม่แก้ต้นฉบับ บน act 0.2.89 / Linux ARM64 ผ่าน Docker ของ Mac ARM64 รอบแรก 111.08 วินาที รอบมี cache 45.86 วินาที ทั้งสองรอบ exit 0 และ tests 15/15 ผ่าน

## สิ่งที่ยังต้องทดลอง

Go lint/test/integration, Testcontainers networking, dirty/untracked source snapshot, act JSON success/failure contract, actrc isolation และ cancellation cleanup มี task แรกในแผนสำหรับพิสูจน์ประเด็นเหล่านี้

## Config examples

- `pitstop-react.example.json`: schema ที่เสนอสำหรับ React job verify
- `pitstop-service.example.json`: schema ที่เสนอสำหรับ service lint/test/integration

ยังไม่ต้องนำ config ไปใส่ repo จนกว่าจะมี implementation การเลือก integration ไม่รับประกันว่ารันได้: เครื่องมือต้องรายงาน blocked เมื่อ runtime/เงื่อนไขไม่รองรับ

## ใช้ส่งต่อ

แตก `pitstop-planning-kit.zip` ให้ agent อ่านไฟล์ตามลำดับด้านบน เมื่อเริ่ม implementation ให้เลือกโฟลเดอร์ repo ใหม่ของ Pitstop และนำเอกสารไปเก็บใน docs ของ repo นั้น โดยไม่เขียน product code ลงในสอง recruit100 repos

เอกสาร technical ใช้ภาษาอังกฤษเพื่อให้คนหรือ agent อ่านต่อได้ตรงตาม interfaces ส่วน context และ README มีสรุปภาษาไทย ไม่มีการเลือก license, สร้าง GitHub repo หรือกำหนดเจ้าของ module path แทนผู้ใช้
