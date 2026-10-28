# Agent testing workflow implementation plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement one task at a time. Track progress with the checkboxes below. Do not start parallel implementation or spawn agents unless the user asks.

**Goal:** ทำให้ผู้ใช้และ agent ใช้ Pitstop ทดสอบโปรเจกต์ได้จากคู่มือที่ชัดเจน พร้อม skill ที่ตรวจผลจริงก่อนสรุป

**Architecture:** ใช้ CLI และ portable skill เดิม ไม่เพิ่ม execution engine หรือ AI provider ใหม่ แยกงานเอกสารก่อน ส่วน skill, embedded manifest และ trusted upgrade ต้องเปลี่ยนพร้อมกันในงานเดียว

**Tech Stack:** Go/Cobra, Markdown, embedded agent assets, Docker และ shell smoke tests

**Spec:** ขอบเขตจาก handoff วันที่ 2026-10-23 ที่ผู้ใช้ให้อ่าน ประกอบกับข้อกำหนดในแผนนี้; เอกสารระบบเดิมคือ [config-agent design](../specs/2026-10-08-config-agent-design.md) และ [release installation design](../specs/2026-10-20-release-install-design.md)

## กติกาการทำงาน

- ทำตามลำดับ 1 → 2 → 3 → 4 ทีละงาน ห้ามรวมทุกงานไว้ branch เดียว
- งานวางแผนนี้ใช้ `feature/agent-testing-plan`; แต่ละงานด้านล่างต้องสร้าง branch ของตัวเองจากฐานที่รวมงานก่อนหน้าแล้ว
- ก่อนเริ่มแต่ละงาน ตรวจ working tree และอัปเดตข้อมูล remote; ห้ามทับงานที่มีอยู่
- จบงานด้วยการตรวจผลและสรุป diff ให้ review ก่อนเริ่มงานถัดไป ห้าม merge/push/publish โดยอนุมานจากคำขอให้เขียนแผน
- ถ้างานก่อนหน้ายังไม่รวมเข้าฐาน ให้หยุดที่จุดส่งมอบ ไม่สร้าง branch ซ้อนต่อโดยเงียบ ๆ
- Branch แผนไม่ใช่ dependency ของ product: เก็บแผนเป็น local commit บน `feature/agent-testing-plan` และเริ่มงาน 1 จาก `main` ที่ตรวจเทียบ remote แล้ว อ่านแผนข้าม branch ด้วย `git show feature/agent-testing-plan:docs/superpowers/plans/2026-10-23-agent-testing-workflow.md`
- ทุก shell command ขึ้นต้นด้วย `rtk`; ใช้ `rtk proxy` เมื่อต้องการ output เต็ม
- ไม่ติดตั้ง skill ลง home จริงเพื่อทดสอบ ไม่พิมพ์ token และไม่แก้ published v0.1.0
- การรัน checks ตามคำขอผู้ใช้ไม่ต้องขออนุมัติซ้ำ แต่ห้ามขยายเป็น deploy/publish, secrets forwarding หรือ arbitrary host execution

## จุดที่ต้องตรวจเป็นพิเศษ

1. คู่มือต้องตรงกับ help ของ binary ที่ build ใหม่ รวม nested/generated commands และ defaults
2. config เดิมที่ใช้ได้ต้องคงไว้; config เสียต้องรายงานหรือแก้เฉพาะที่จำเป็น และ job IDs ต้องอ่านจาก config
3. Docker ใช้ไม่ได้, toolchain/dependency error, test fail, timeout และ cancellation ต้องไม่ถูกสรุปเป็น pass
4. อัปเกรด bundle v0.1.0 ได้โดยไม่ยอมทับ unknown หรือ user-modified bundle
5. หลักฐาน agent เข้าใจคำขอต้องแยกจาก Docker runtime evidence และไม่อ้าง automatic provider discovery โดยไม่มีการทดลอง

## งาน 1 — เติมคู่มือคำสั่งให้ครบ (ง่ายก่อน)

**Branch:** `feature/cli-command-reference`

**ไฟล์:** `docs/user-guide.md`, `docs/user-guide.en.md`; อ่าน `internal/cli/*.go` และ help ที่ build จริงเป็นแหล่งอ้างอิง

**รับเข้า:** CLI ปัจจุบัน โดยไม่เปลี่ยน behavior

**ส่งมอบ:** คู่มือไทย/อังกฤษที่ใช้อ้างอิงคำสั่งและ flags ได้ครบ เพื่อให้ README ในงาน 2 ลิงก์มาได้

- [x] Build binary ไป temp directory แล้วเก็บ inventory ของ root help และ help ของทุก subcommand
- [x] ใช้ `rtk proxy env GOTOOLCHAIN=go1.26.5 go build -o <temp-dir>/pitstop ./cmd/pitstop`; ตรวจ root help, `help`, `help <command>`, `<command> --help` และ nested help ทุกตัว เก็บ output ชั่วคราวสำหรับเทียบโดยไม่เพิ่ม generated help dump ลงคู่มือ
- [x] เติม `help`, `init`, `validate`, `doctor`, `list`, `run`, `version`, `skill install` และ `completion bash/zsh/fish/powershell`
- [x] ระบุ global `--repo` และ flags/defaults ที่แต่ละคำสั่งรองรับจริง รวม job selection, timeout 20m, architecture, JSON, dry-run, force และข้อห้าม user-scope กับ explicit `--repo`
- [x] อธิบาย prerequisites, offline validation, cancellation/exit codes และแยก CLI install จาก skill install
- [x] ตรวจจาก implementation เพิ่มเติมในส่วนที่ help ไม่ระบุ: init ต้องรักษา config เดิม, repeated `--command`, run progress กับ JSON แยก stream, `--job` ซ้ำ/คั่น comma, report/log paths และความหมาย exit codes; ตัวอย่างห้ามรัน project scripts หรือเขียน config ของ checkout เพื่อทดสอบเอกสาร
- [x] ตรวจตารางและตัวอย่างเทียบกับ built help ทั้งสองภาษา ตรวจ relative links/anchors; ห้ามเรียก PowerShell completion ว่ารองรับ native Windows
- [x] Review diff และส่งมอบเฉพาะงานคู่มือ โดยไม่เปลี่ยน CLI หรือ bundle

## งาน 2 — ย่อ README ให้เริ่มใช้ได้ทันที

**Branch:** `feature/readme-quickstart`

**ไฟล์:** `README.md`, `README.th.md`, `docs/README.md`; อ้างอิง `docs/install.md` และคู่มือจากงาน 1

**รับเข้า:** command reference ที่ตรวจแล้วจากงาน 1

**ส่งมอบ:** หน้าแรกอ่านง่าย พร้อมทางไปข้อมูลครบโดยไม่ทำข้อมูลประวัติหาย

- [x] จัดลำดับ: ประโยชน์ → curl install → config ที่ใช้ได้ → validate/doctor/run → command table → agent entry point → ผลลัพธ์และข้อจำกัด
- [x] คง direct install.sh link และลิงก์ทางเลือก GitHub CLI/source แบบสั้น อธิบายการ review script และกรณีติดตั้งไม่สำเร็จให้ชัด
- [x] ระบุว่า base init ยังต้องใส่คำสั่งจริง; ไม่อ้างว่า starter ทุกแบบพร้อมรันทันที
- [x] ใส่ทุก command/subcommand ในตารางแบบกระชับ พร้อมวิธีเรียก help และ common options ที่สำคัญ; flags เต็มลิงก์ไปงาน 1
- [x] แยกติดตั้ง CLI กับ optional skill พร้อมตัวอย่างติดตั้ง skill และคำขอ “ใช้ Pitstop ทดสอบโปรเจกต์นี้”; ก่อนงาน 3 เสร็จต้องระบุขอบเขต skill ปัจจุบันตามจริง
- [x] ลด Further reading เหลือประมาณสามลิงก์: user guide, AI-agent guide, docs index ย้ายทางไป design/plan/evidence/roadmap ไว้ docs index โดยรักษาข้อมูลและลิงก์
- [x] ตรวจภาษาไทย/อังกฤษให้ความหมายตรงกัน ตรวจ links/anchors และ quick-start กับ command help ส่งมอบเฉพาะงานหน้าแรก

## งาน 3 — ขยาย skill พร้อม bundle upgrade ที่ปลอดภัย

**Branch:** `feature/agent-testing-workflow`

**ไฟล์:** `internal/agentassets/bundle/SKILL.md`, `internal/agentassets/bundle/references/{config-reference,examples,workflow-mapping}.md`, `internal/agentassets/bundle/manifest.json`, `internal/agentassets/assets.go`, `internal/agentassets/assets_test.go`, historical assets ที่จำเป็น, `internal/skill/install_test.go`, `docs/ai-agent.md`, `docs/ai-agent.en.md`, `docs/README.md`, `docs/roadmap.md`; ปรับ README เฉพาะข้อความ capability ที่เปลี่ยน

**รับเข้า:** CLI เดิมและคู่มือที่ครบจากงาน 1–2

**ส่งมอบ:** candidate binary ที่ฝัง skill ใหม่และอัปเกรดจาก released bundle ได้ โดยยังป้องกัน directory ที่ไม่ได้เป็นของ Pitstop

- [x] อ่าน skill-creator/writing-for-agents และตรวจ trusted history ปัจจุบันเทียบ exact bundle ใน tag v0.1.0 ก่อนแก้ไฟล์
- [x] เพิ่ม failing regression test สำหรับ released-bundle force upgrade และกรณี unknown/user-modified bundle ต้องถูกปฏิเสธ; ใช้ fixture/temp HOME เท่านั้น
- [x] ขยาย trigger และ workflow: CLI/version/help → inspect repo/config → preserve หรือแก้ config ตามหลักฐาน → validate/list/doctor/run ด้วย JSON ตาม command surface ที่ติดตั้ง
- [x] ระบุวิธีจัดการ CLI ที่ไม่มี, config absent/broken, arbitrary job IDs, image/toolchain/dependency error, Docker unavailable, genuine failure, timeout/cancel และ incomplete cleanup
- [x] ให้ agent อ่าน report/logs ก่อนสรุป พร้อมรายการที่รันจริง/ไม่ได้รัน และ direct-read fallback; ห้ามเปลี่ยน source/tests เพื่อทำผลให้ผ่าน
- [x] เปลี่ยน bundle version จาก `1` เป็น `2`, regenerate manifest hashes/inventory และ allowlist หากเพิ่มไฟล์; เก็บ exact released manifest เป็น trusted history เมื่อยังไม่มี equivalent entry โดยไม่แก้ historical fixtures เดิม
- [x] แก้คู่มือ agent สองภาษา/index/roadmap ให้ตรง behavior และชี้ว่าต้องใช้ binary ใหม่จึงได้ embedded skill ใหม่
- [x] ตรวจ focused asset/installer tests จากนั้น `rtk proxy env GOTOOLCHAIN=go1.26.5 go test -race -count=1 ./...` และ `rtk proxy env GOTOOLCHAIN=go1.26.5 go vet ./...`
- [x] Build candidate ใหม่ แล้วรัน `rtk proxy env PITSTOP_BIN=<fresh-binary> PITSTOP_CONFIG_AGENT_RUNTIME=0 sh scripts/config-agent-smoke.sh`; ตรวจ manifest และ install/force-upgrade ใน temp directories
- [x] Review ความปลอดภัยและ diff ส่งมอบ candidate กับผลตรวจ โดยไม่สร้าง release ใหม่

## งาน 4 — ทดลองคำขอ agent จริงและเก็บหลักฐาน

**Branch:** `feature/agent-testing-trials`

**ไฟล์:** เพิ่ม `docs/evidence/agent-testing-trials/README.md` และ fixtures/raw prompts/results ใต้ directory เดียวกัน; ปรับ `docs/README.md`, `docs/roadmap.md` และ `scripts/config-agent-smoke.sh` เฉพาะเมื่อจำเป็น

**รับเข้า:** candidate binary และ skill bundle จากงาน 3

**ส่งมอบ:** หลักฐานที่ตรวจย้อนกลับได้ว่าคำขอทดสอบโปรเจกต์ทำงานถึงขั้นใด

- [x] ใช้ harmless isolated repositories และ temp HOME/install directories ระบุ binary version/hash และวิธีส่ง skill ให้ agent
- [x] ทดลองคำขอสั้นกับ existing valid config ที่ใช้ custom job IDs, absent config และ broken config เก็บ raw prompt/result พร้อม config/report/log ที่เกี่ยวข้อง
- [x] ตรวจกรณี Docker unavailable, genuine test failure, timeout/cancellation และ missing prerequisites ว่า agent แยกผลและไม่อ้าง pass ผิด; ระบุชัดว่ากรณีใดเป็น simulated/tool output และกรณีใดรันจริง
- [x] หาก Docker พร้อม ให้รัน runtime smoke กับ fixture เล็กและเก็บ report/logs จริง; หากไม่พร้อมให้บันทึก blocked พร้อมเหตุผล ห้ามนับ offline checks เป็น runtime pass
- [x] บันทึกปัญหาที่พบตามจริง; หากต้องแก้ implementation แยกเป็นงานแก้บน branch ใหม่ก่อนทดลองซ้ำ ไม่ซ่อนการแก้ไว้ใน evidence branch
- [x] ตรวจว่า evidence เดิม 23 config trials ยังเรียกตามประเภทเดิม ไม่เปลี่ยนเป็น end-to-end evidence และไม่อ้าง provider discovery จาก direct-read trial
- [x] Review หลักฐาน ลิงก์ และสรุป passed/failed/blocked พร้อมข้อจำกัด อัปเดต roadmap เฉพาะสิ่งที่มีหลักฐาน

## สถานะส่งมอบ — 2026-10-28

ผู้ใช้สั่งให้ทำงานที่เหลือทั้งหมดให้เสร็จเมื่อ 2026-10-28 จึงทำตามลำดับบน branch แยกและตรวจแต่ละงานก่อนรวมเข้า `main`: งาน 1 `feature/cli-command-reference`, งาน 2 `feature/readme-quickstart`, งาน 3 `feature/agent-testing-workflow`, งาน 4 `feature/agent-testing-trials` งาน 1–3 รวมแล้วขณะจัดทำหลักฐานงาน 4

[หลักฐานและข้อจำกัด](../../evidence/agent-testing-trials/README.md) บันทึก baseline ที่ runtime ผ่านแต่ยังไม่ได้อ่าน report/log แยก, direct-read agent trials, Docker runtime cases, regression replay และ smoke cleanup retry ไม่ได้อ้าง automatic provider discovery หรือทดสอบครบทุก toolchain การเผยแพร่ release ใหม่ยังไม่ได้รับคำสั่ง และ v0.1.0 ไม่ถูกแก้ไข
