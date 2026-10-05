# ความต้องการและขอบเขตของ Pitstop MVP

**ก่อนใช้ต้องมี Git repo, workflow, `.pitstop.json`, act 0.2.89 และ Docker ที่พร้อมรัน**

เอกสารนี้แยกสิ่งที่ผู้ใช้ต้องเตรียม ออกจากข้อกำหนดที่ Pitstop ต้องทำตาม [ดูขั้นตอนใช้งาน](user-guide.md) · [กลับหน้าแรก](../README.md)

## 1. เครื่องต้องมีอะไร

| สิ่งที่ต้องมี | ข้อกำหนด |
|---|---|
| Git | เรียกใช้งานจาก PATH ได้ |
| act | **0.2.89**; เวอร์ชันอื่นถูกปฏิเสธใน MVP |
| Docker | daemon เชื่อมต่อได้ และรัน runner image ที่กำหนดได้ |
| Go | **1.26.5 ขึ้นไป** เมื่อ build Pitstop จาก source; ไม่จำเป็นสำหรับเปิด binary ที่ build แล้ว |
| พื้นที่จัดเก็บ | เพียงพอสำหรับสำเนา repo, Docker image, cache, report และ log; ยังไม่มีตัวเลขขั้นต่ำที่วัดยืนยัน |
| เครือข่าย | ต้องเข้าถึง image, action และ dependency ที่ workflow ต้องดาวน์โหลด หากยังไม่มีใน cache |

Pitstop ตรวจความพร้อม แต่ไม่ได้ติดตั้งเครื่องมือให้เอง ส่วนเครื่องมือของโปรเจกต์ เช่น Node.js หรือ Go ภายใน runner ให้ workflow และ runner image เป็นผู้เตรียม

**สภาพแวดล้อมที่ทดสอบจริง:** macOS ARM64, act 0.2.89, Docker 29.4.0/OrbStack และ Linux ARM64 runner มีผลทดสอบกับ React verify, Go lint/unit tests และ PostgreSQL/Testcontainers integration

config ยอมรับ `linux/amd64` ด้วย แต่ยังไม่มีผลทดสอบ runtime บน AMD64 และไม่มีการยืนยันรองรับ host ทุกระบบปฏิบัติการ

## 2. โปรเจกต์ต้องมีอะไร

| สิ่งที่ต้องเตรียม | ข้อกำหนด |
|---|---|
| Repository | เป็น Git repo และมีอย่างน้อยหนึ่ง commit สำหรับ `run` |
| Workflow | หนึ่งไฟล์ต่อ config, อยู่ภายใน repo, นามสกุล `.yml` / `.yaml` และมี trigger `pull_request` |
| Config | `.pitstop.json` ที่ราก repo; schema version `1`, JSON ถูกต้อง และไม่มี field ที่ไม่รู้จัก |
| Job selection | อย่างน้อยหนึ่ง ID, ไม่ซ้ำ และมีอยู่จริงใน workflow |
| Runner | ระบุ label, image และ architecture; label ตรงกับ `runs-on` ของ job |
| Source | ไม่มี unresolved merge, submodule หรือ symlink ที่ MVP ปฏิเสธ |

config ใช้เลือกงานและ runner ส่วนคำสั่งตรวจโค้ดยังคงอยู่ใน workflow

## 3. ฟีเจอร์ที่มีแล้ว

| ฟีเจอร์ | ช่วยผู้ใช้อย่างไร |
|---|---|
| `doctor` | ตรวจ repo, config, workflow และเครื่องมือก่อนรัน |
| `list` | ดู job ที่เลือก งานที่ถูกบล็อก และเหตุผล |
| `run` | รัน job ที่เลือกผ่าน act และ Docker ทีละรายการ |
| `version` | ดูเวอร์ชันโปรแกรม |
| Working-tree snapshot | ทดสอบโค้ดที่แก้ไว้ รวมไฟล์ใหม่ที่ยังไม่ commit |
| เลือกบาง job | ใช้ `--job` ซ้ำเพื่อเลือกจาก config |
| JSON output | ส่งผลให้ script หรือ AI agent อ่าน โดยแยก progress ไป stderr |
| Report และ raw log | ย้อนดูผล, source hash, workflow และคำสั่งที่ใช้จริง |
| Timeout และ cancellation | จำกัดเวลาต่อ job และหยุดด้วย Ctrl+C |
| Repo lock | ป้องกัน run พร้อมกันบน repo เดียวกัน |
| Cache | เก็บ action/dependency cache เพื่อใช้ซ้ำ |
| Service integration adapter | รัน integration ของ recruit100-service แบบ opt-in และแสดงสิ่งที่ปรับ |

## 4. Workflow แบบใดรันได้

generic runner รองรับ job ที่ **รันแยกได้ ใช้ runner แบบคงที่ และไม่มีเงื่อนไขที่ต้องประเมินเพื่อเลือกว่าจะรันหรือไม่** เช่น lint, unit test หรือ verify

| รูปแบบ | ขอบเขตของ MVP |
|---|---|
| Job อิสระ, static `runs-on`, executable steps ที่รองรับ | รองรับ เมื่อ runner ตรงกับ config |
| Local composite action | ตรวจ metadata และ composite ซ้อนกัน; เงื่อนไข/continue-on-error ที่ไม่รองรับถูกบล็อก |
| `needs`, job-level `if` | บล็อก; ยกเว้นรูปแบบเฉพาะที่ service adapter ตรวจผ่าน |
| Step condition | บล็อกเงื่อนไขที่ไม่ใช่ literal `true` |
| Matrix / strategy | บล็อก |
| Dynamic runner / needs | บล็อก |
| Reusable workflow | บล็อก |
| Job `container` / `services` | บล็อก |
| `continue-on-error` | บล็อกที่ระดับ job หรือ step |
| Secret references | บล็อก รวม reference ที่พบใน local composite |
| Job ว่าง หรือมีแต่ step ที่ข้ามการทำงาน | ไม่นับว่ารันผ่าน |

remote action internals ให้ act เป็นผู้จัดการ การตรวจ local metadata ไม่ได้หมายถึงตรวจพฤติกรรมภายใน action ทุกตัว งาน deploy ไม่อยู่ใน selection ของ profile ตัวอย่าง; ควรเลือกเฉพาะงานตรวจโค้ดที่ตั้งใจรัน

## 5. ข้อกำหนดการทำงานของ MVP

| ID | สิ่งที่กำหนด | พฤติกรรมที่ผู้ใช้ควรได้ |
|---|---|---|
| R01 | ใช้ workflow เดิม | คำสั่ง lint/test/build อยู่ใน workflow; config เลือกงาน |
| R02 | ทดสอบ source ปัจจุบัน | working tree มีลำดับเหนือ staged content และเก็บ hash ของ snapshot |
| R03 | แยกจาก source ต้นฉบับ | รันสำเนาในพื้นที่ชั่วคราว และตรวจว่า source ไม่เปลี่ยนระหว่าง capture |
| R04 | เลือกงานอย่างชัดเจน | ไม่เพิ่ม dependency job เอง; งานไม่รองรับมีเหตุผลให้ดู |
| R05 | รายงานตามหลักฐาน | exit 0 ต้องมี outcome ของ job ที่เลือก; ความไม่แน่ชัดไม่กลายเป็น passed |
| R06 | ผลรวมมีลำดับแน่นอน | cancelled `130` → executed failure `1` → partial/preflight `2` → success `0` |
| R07 | เก็บผลที่เกิดขึ้นแล้ว | failure ของ job ก่อนหน้ายังอยู่ แม้ job ถัดไป launch ไม่ได้ |
| R08 | JSON อ่านด้วยโปรแกรมได้ | schema version `1`, หนึ่งเอกสารบน stdout; progress บน stderr |
| R09 | แสดงความครบและการปรับ | แยก selected/workflow coverage; forced integration แสดง adjustments |
| R10 | แยก local configuration | ใช้ HOME/config ชั่วคราวและไม่ส่ง host token, secret หรือ actrc เป็นค่าให้ engine |
| R11 | หยุดและเก็บกวาดตามขอบเขต | ยกเลิก/timeout ได้ และ cleanup เฉพาะ runner ที่ติด ownership label ของ run |
| R12 | ย้อนตรวจได้ | เก็บ report, log, source manifest, workflow hashes, เวลารันและ argv; ลบ snapshot/HOME ชั่วคราวเมื่อจบ |

tracked `.env` ยังเป็น source input ตาม R02 การแยก local configuration ใน R10 ไม่ได้ตัดไฟล์ที่อยู่ใน Git ออกจาก source

## 6. ขอบเขต integration adapter

adapter `recruit100-service-integration-v1` ใช้ได้เมื่อ config เลือก `integration` และผู้ใช้ระบุ `--force-integration` เท่านั้น ไม่เปิดเองเพียงเพราะมี adapter ใน config

- ตรวจ job และคำสั่ง integration ว่าตรงกับรูปแบบที่กำหนด
- ลบเฉพาะ `needs` และ `if` ของ integration ในสำเนา workflow
- ต้องใช้ local Unix Docker endpoint; integration runner เข้าถึง Docker socket เพื่อใช้ Testcontainers
- เมื่อผ่าน ยังรายงาน `workflowCoverage=partial` เพราะเปลี่ยนเงื่อนไขเดิม

โหมดปกติของ service profile รัน lint/test ได้ และแสดง integration ว่า blocked

## 7. ข้อจำกัดที่ยังมี

- ไม่รองรับ submodule และ absolute source symlink; relative symlink ต้อง resolve อยู่ภายใน repo
- ไม่ส่ง local secrets/tokens เพื่อทำให้งานที่ต้องใช้ credential ผ่าน
- ยังไม่มี native support สำหรับ conditional paths-filter ของ service workflow
- ไม่รับประกันรายละเอียดหรือจำนวน step/test จาก `steps: []`; รายงานเฉพาะข้อมูลที่ยืนยันได้
- Report/log ไม่มีการล้างตามอายุอัตโนมัติ ผู้ใช้เลือกเก็บหรือลบ run ที่จบแล้วเอง
- ผลผ่านบนเครื่องยังต้องผ่าน required GitHub checks และไม่ได้ยืนยัน deployment readiness

## 8. หลักฐานและเอกสารต้นทาง

[ผลทดสอบจริง](evidence/final-validation.md) · [ผล review และการแก้ไข](evidence/final-review.md) · [Spec ต้นทาง](design/pitstop-spec.md) · [Implementation plan](superpowers/plans/2026-10-05-pitstop-mvp-1.md)
