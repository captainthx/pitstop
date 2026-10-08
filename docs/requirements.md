# ข้อกำหนดและขอบเขต Pitstop

[English](requirements.en.md) | [คู่มือใช้งาน](user-guide.md) | [สารบัญ](README.md)

เอกสารนี้ระบุความสามารถปัจจุบันของ Docker checks ส่วนวิธีติดตั้งและตัวอย่างคำสั่งอยู่ในคู่มือ

## ข้อกำหนด

- Host มี Git, Docker CLI และ daemon ที่เชื่อมต่อได้; build CLI ด้วย Go 1.26.5 ตาม `go.mod`
- Repo เป้าหมายมี Git HEAD อย่างน้อยหนึ่ง commit และ `.pitstop.json` schema version 2 ที่ Git root
- ผู้ใช้คัดลอก/เขียน config ครั้งเดียวต่อ repo: ระบุ image, architecture, check definitions และ selection ไม่มี config generator
- Image เป็น Linux `arm64` หรือ `amd64` มี `/bin/sh` และ tools ที่ commands ใช้
- ใช้ registry/network ตามที่การ pull image และ project commands ต้องการ
- การรันจริงที่ยืนยัน local คือ macOS ARM64 / Docker Linux ARM64; native Windows และ cross-architecture execution ยังไม่ยืนยัน

## พฤติกรรมที่รองรับ

| ส่วน | ข้อตกลง |
| --- | --- |
| Config | JSON document เดียว, version 2, strict fields/IDs; schema เดิมต้องย้ายเอง |
| Commands | `doctor`, `list`, `run`, `version`; help `-h`/`--help` และ JSON output |
| Selection | `jobs` กำหนด default selection/order; `--job` เลือก subset จากรายการนี้ |
| Environment | ใช้ image ใน daemon หรือ pull เมื่อจำเป็น; เตรียมสูงสุด 2 นาที; ไม่ build image ให้ |
| Snapshot | ตรวจ working tree รวม tracked edits/deletions และ new non-ignored files; ไม่ mount repo ต้นฉบับ |
| Isolation | Container ใหม่ต่อ check; ไม่มี host bind mount, Docker socket mount หรือ privileged mode |
| Steps | `/bin/sh -e`, cwd `/workspace`, user `0:0`; แชร์ files แต่เป็น shell ใหม่ทุก step |
| Failure | step failed ทำให้ steps ที่เหลือใน check skipped; independent check ถัดไปรันต่อ |
| Timeout | 20 นาทีต่อ check โดยปริยาย; override ด้วย positive `--timeout`; รวม create/copy/start/run |
| Cancellation | Ctrl+C/SIGTERM หยุด scheduling และพยายาม stop/remove owned containers ด้วย bounded cleanup |
| Concurrency | ล็อก repo เดียวกัน; repo อื่นรันแยกได้ |
| Evidence | Docker terminal state + exit records; success text อย่างเดียวไม่ถือว่าผ่าน |
| Reports | log, source/image identity, config, arguments, exit evidence และ ownership records ใน user cache |
| Cleanup | ลบ owned container volumes, snapshot และ temporary scripts; เก็บ reports/logs; ไม่ prune images/resources อื่น |

## ผลลัพธ์

Config schema เป็น 2 แต่ JSON envelope schema เป็น 1 ค่า `workflowCoverage` เป็น `not_applicable` เสมอ `selectedCoverage=complete` หมายถึงมีหลักฐานครบสำหรับ checks ปกติ ไม่ได้หมายถึงทุก check ผ่าน ส่วน timeout/runtime/unknown ทำให้ coverage เป็น partial

ลำดับ exit code: cancellation `130` → failed job `1` (รวม timeout/runtime failure) → invalid/blocked/unknown/log/report/cleanup problem `2` → ทุก check ผ่าน `0` ปัญหาบันทึกผลหรือ cleanup ไม่กลบ failure ที่ตรวจพบแล้ว

Step durations เป็น `null` ส่วน check/run มี duration จริง ข้อผิดพลาดก่อนเริ่ม execution อาจไม่มี report ไม่มี automatic report retention หรือ artifact export

## ขอบเขตที่ยังไม่มี

- การอ่านและ execute `.github/workflows/*.yml`, แปล `uses:` หรือ expressions
- Matrix, job dependencies (`needs`) และ service containers
- Secret/environment injection ผ่าน config, host environment forwarding หรือ cache volumes
- Automatic image build, automatic workflow/config conversion และ `pitstop init`
- Self-hosted runner registration หรือส่ง source ไป GitHub
- Installer link/release installation ที่จัดทำไว้

Snapshot ไม่รองรับ submodules, unresolved merges, absolute/broken/escaping symlinks ไฟล์ untracked ที่ ignore หรือชื่อกลุ่ม `.env`/`.env.*`/`.secrets`/`.vars`/`.input` ถูก exclude แต่ tracked files ยังรวมอยู่ การ exclude ไม่ใช่ secret scanning

การใช้ shared script ร่วมกับ CI ทำได้ แต่ความเหมือนของผลขึ้นกับ image/toolchain/OS/network ที่ตั้งไว้ ไม่มีการรับประกัน workflow parity

## แผน config และ AI agent (2026-10-08)

มี [design](superpowers/specs/2026-10-08-config-agent-design.md) และ [implementation plan](superpowers/plans/2026-10-08-config-agent.md) สำหรับ base/starter configs หลายภาษา/build tools, `init`, `validate` และ `skill install` ใช้ AI agent ของผู้ใช้ช่วยอ่าน manifests/scripts/workflows แล้วสร้าง config ไม่ฝัง AI provider ใน Pitstop และไม่ execute workflow YAML โดยตรง งานเหล่านี้ยังไม่ได้ implement

Validation จะใช้ acceptance pipeline เดียวกับ run/doctor/list รวม strict JSON, bounded input, safe file access, semantic limits และ tests/fuzz ตามแผน การผ่าน validation หมายถึง config ถูกต้องตามกฎ ไม่ได้รับรองว่า shell/image ปลอดภัยหรือ checks ครอบคลุม CI ทั้งหมด ดู [roadmap](roadmap.md) สำหรับขอบเขตที่ยังไม่รวมและเกณฑ์พัฒนาต่อ

## หลักฐานและการตรวจสอบ

ดู [Docker validation ปัจจุบัน](evidence/docker-checks-validation.md) สำหรับ tests, smoke และข้อจำกัดของหลักฐานที่รันจริง GitHub CI มี smoke แต่ผล local ไม่ได้ยืนยันว่า remote CI รันผ่านแล้ว เอกสารในหมวดประวัติของ [สารบัญ](README.md) ไม่ใช่ขอบเขตของ CLI ปัจจุบัน
