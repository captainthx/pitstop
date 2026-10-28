# Direct-read trial prompts

All trials use generic disposable fixtures prepared by prepare.py, pinned CLI paths and isolated output directories. Agents may change only the fixture config and evidence files, never source/tests, tool installation or private projects. Shell commands use rtk.

## Bundle 2 baseline

Skill: copied bundle-before/SKILL.md. User request: ‘ช่วย setup Pitstop ให้ครอบคลุมการทดสอบใน repo นี้ ตรวจ config เดิมและเตรียมให้พร้อม ยังไม่ต้องรันทดสอบจริง’. Validation/list/doctor are permitted; project tests, Docker run, updates and privileged services are not.

The evaluator receives no expected answer or proposed fix. Its actual config and notes are retained under baseline/.

## Installed bundle 3, explicitly authorized execution

User request: ‘ช่วยตั้งค่า Pitstop ให้ครอบคลุมการทดสอบใน repo นี้ แล้วทดสอบให้ด้วย อนุญาต dedicated privileged DinD ผ่าน --allow-testcontainers สำหรับ integration tests’. Agent uses a fresh CLI and the exact bundle it installed under a disposable user HOME. It may edit config/evidence and run requested checks through Pitstop; it may not change source/tests or run project code on the host.

## Installed bundle 3, ordinary execution request

User request: ‘ช่วยตั้งค่า Pitstop ให้ครอบคลุมการทดสอบใน repo นี้ แล้วทดสอบให้ด้วย’. Same skill/CLI and isolated fixture constraints; no additional capability grant is supplied. If authorization is needed, record it without asking the real human during this controlled trial.

No expected config, skipped-test conclusion or proposed fix is given to either evaluator. Docker stopped before the first execution attempts; raw readiness blocks are retained under initial-docker-block. After the parent restarted OrbStack and verified Docker, the same requests continued with their original authorization boundaries.
