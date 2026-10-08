# แนวทางพัฒนา Pitstop / Development roadmap

**Updated:** 2026-10-08 (Asia/Bangkok). This is planning guidance, not a release promise or a list of installed CLI features.

[Current docs / เอกสารปัจจุบัน](README.md) · [Requirements / ข้อกำหนด](requirements.md) · [English requirements](requirements.en.md)

## งานที่วางแผนอยู่ / Planned work

AI-agent-led configuration, manual starters, strict shared validation and CLI installation of a portable skill are covered by the [design](superpowers/specs/2026-10-08-config-agent-design.md) and [implementation plan](superpowers/plans/2026-10-08-config-agent.md).

งานนี้ยังไม่ได้ implement: `pitstop init`, `pitstop validate` และ `pitstop skill install` ยังไม่มีใน CLI ปัจจุบัน ตัวอย่างปัจจุบันมี Node และ Go ส่วนตัวอย่างภาษา/build tools อื่นและชุด skill อยู่ในแผน เมื่อส่ง implementation ต้องอัปเดต README, guides, requirements และสารบัญทั้งสองภาษาให้ตรงกับผลตรวจจริง

This work is not implemented yet. Current examples cover Node and Go; other starters and the portable skill are planned. Update both README languages, guides, requirements and index as implementation and actual verification land.

## งานที่ยังไม่รวมในรอบนี้ / Deferred scope

รายการต่อไปนี้เก็บไว้พิจารณาเมื่อมี use case จริง ไม่ทำโดยอนุมานจากแผน config/skill และไม่รับประกันลำดับหรือวันที่ส่ง

| แนวทาง / Direction | พิจารณาเมื่อ / Trigger and prerequisites |
| --- | --- |
| AI provider / API key / embedded LLM | เมื่อจำเป็นต้องสร้าง config โดยไม่ใช้ agent ภายนอก ต้องกำหนด privacy, cost, credential storage และ consent ก่อน ปัจจุบันใช้ agent ของผู้ใช้ / When a standalone AI product is needed; external agents suffice today. |
| GitHub Actions executor | เมื่อจำเป็นต้องรัน workflow เดิมโดยตรง ต้องแยก feasibility/compatibility/security work รวม uses, expressions และ reusable workflows; agent-assisted mapping is not execution parity. |
| Automatic workflow importer | เมื่อมีชุด workflow ที่แปลงซ้ำได้อย่างชัดเจน ต้องกำหนด supported subset, provenance และ fail-on-unsupported behavior; do not silently drop steps. |
| Automatic manifest/preset detection | เมื่อ explicit presets และ agent inspection ไม่พอ ต้องรองรับ multiple manifests, package manager versions และ monorepo ambiguity without guessing. |
| Matrix / needs / services | เมื่อ checks ต้องใช้หลาย environment หรือ services จริง ต้องออกแบบ scheduling, lifecycle, networking, failure propagation และ evidence ใหม่. |
| Config env / secrets / credential forwarding | เมื่อมี integration checks ที่จำเป็น ต้องมี allowlist, redaction, storage/lifetime policy และไม่ส่ง host environment ทั้งชุด. |
| MCP server / richer agent integration | เมื่อ file access + CLI JSON ไม่พอ มี remote/tool discovery need ที่พิสูจน์ได้; don't add another execution backend just for skill use. |
| Automatic image build / per-check images | เมื่อ single prebuilt image ไม่พอสำหรับ mixed toolchains ต้องกำหนด provenance, build context, cache และ image selection; current v2 has one runner image. |
| Dependency cache volumes | เมื่อมี performance evidence ต้องกำหนด isolation, ownership, invalidation และ cleanup; no shared writable cache by default. |
| Artifact export / report retention | เมื่อจำเป็นต้องนำ build artifacts กลับหรือจัดการ cache growth ต้องกำหนด safe paths, quotas, symlink behavior และ retention rules. |
| Hostile-code sandbox / resource policy | เมื่อรองรับ code/images ที่ไม่ไว้ใจ ต้องมี separate threat model, network/CPU/memory policy และ daemon/kernel isolation. Strict config validation alone cannot supply this guarantee. |
| Skill uninstall / remote distribution / plugin package | เมื่อ local bundle install ไม่พอ ต้องมี ownership/version/update/recovery policy; no arbitrary destination or user-file deletion. |
| JSON Schema artifact / editor integration | เมื่อ editors ต้องใช้ autocomplete/schema ให้ derive/check against canonical model/parser and rejection tests; no independent validator that drifts. |
| Self-hosted runner registration / upload source | เมื่อมีความต้องการใช้ GitHub runner จริง ต้องกำหนด authentication, source handling และ execution trust separately. |
| Native Windows / broader platforms | เมื่อมี demand ให้แก้ Unix locking/file safety and verify on target OS; do not advertise unsupported portability. |
| Docker engine / snapshot / cleanup refactors | แผน config/agent ไม่เปลี่ยน algorithms เหล่านี้ งานปรับโครงสร้างต้องมีปัญหาและหลักฐานแยก; reuse the existing backend for this release. |
| Release installer / publishing | งานเดิมยังพักไว้ ต้องมีคำสั่งจากผู้ใช้และ release verification; planning/docs updates do not authorize push, release or publish. |

## หลักการอัปเดต / Maintenance

- เพิ่ม future work พร้อมปัญหาจริงและเกณฑ์เริ่ม ไม่เพิ่ม feature เผื่ออนาคต / Tie additions to observed needs.
- เมื่อเริ่มงาน ให้ทำ design/plan ของขอบเขตนั้นและตรวจ compatibility/security ก่อน / Plan that scope separately.
- เมื่อส่งงาน ย้ายรายการไป current docs พร้อมหลักฐาน ไม่อ้างความสามารถจาก spec อย่างเดียว / Promote only verified behavior to current documentation.
- เก็บ limitations ของ validator, AI mapping และ runtime ให้แยกกัน / Keep config validity, mapping coverage and execution safety distinct.
