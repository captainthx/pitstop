# ข้อกำหนดและขอบเขต

Pitstop ต้องใช้ Git repository ที่มี commit อย่างน้อยหนึ่งรายการ, Git, Docker CLI และ Docker daemon ที่เชื่อมต่อได้ โปรเจกต์เป้าหมายต้องมี `.pitstop.json` schema version 2 ส่วน Pitstop จัดการ snapshot, container lifecycle, timeout, cancellation, cleanup, log, evidence และ report

config ระบุ Linux image (`linux/arm64` หรือ `linux/amd64`), check ที่เลือก และ shell step ตามลำดับ image ต้องมีเครื่องมือที่ step เรียก ใช้ image สาธารณะหรือ image ที่ผู้ใช้ build เองได้ แต่ Pitstop ไม่ build image อัตโนมัติ

รุ่นแรกมี `run`, `doctor`, `list` และ `version` ไม่อ่าน `.github/workflows/*.yml`, ไม่ติดตั้งหรือเรียก `act`, ไม่แปล `uses:` หรือ expression และยังไม่มี matrix, needs, services, secret injection, cache volume หรือ self-hosted runner `workflowCoverage` เป็น `not_applicable` เสมอ

แต่ละ check ได้ container ใหม่ Steps ใน check เดียวกันใช้ `/workspace` และ evidence ใต้ `/pitstop` ร่วมกัน แต่ check อื่นมองไม่เห็นไฟล์ที่สร้างไว้ source ถูก copy จาก snapshot ผ่าน `docker cp` จึงตรวจ dirty, new และ deleted files ได้โดยไม่ bind mount working tree Timeout และ cancellation จะหยุด container ที่มี label ของ run แล้ว cleanup เฉพาะทรัพยากรนั้น

schema version 1 จะถูกปฏิเสธพร้อมคำแนะนำ migration ให้ย้ายคำสั่ง `run` มาไว้ใน `checks`, แทน `checkout` ด้วย snapshot ของ Pitstop และแทน setup action ด้วย toolchain image Workflow เดิมยังเป็นหน้าที่ของ CI ในโปรเจกต์

รายงานเก็บ image ID, exit record ของ step, log ดิบ, source identity, command argv และหลักฐาน cleanup แยก failure ของ check, evidence ที่หาย และ runtime failure ค่า exit code คือ 0 เมื่อผ่านทั้งหมด, 1 เมื่อพบ check failure, 2 เมื่อ config/blocked/unknown/persistence/cleanup failure และ 130 เมื่อยกเลิก

ดู [คู่มือ](user-guide.md), [ตัวอย่าง Docker](../examples/docker-react.json) และ [หลักฐานเดิม](evidence/final-validation.md)
