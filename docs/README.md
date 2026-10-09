# เอกสาร Pitstop / Documentation

## เอกสารสำหรับ CLI ปัจจุบัน / Current CLI

| เอกสาร / Document | ไทย | English |
| --- | --- | --- |
| ติดตั้งและเริ่มใช้งาน / Install and quick start | [README](../README.th.md) | [README](../README.md) |
| คู่มือและ config reference / User guide and configuration | [คู่มือ](user-guide.md) | [Guide](user-guide.en.md) |
| ข้อกำหนดและขอบเขต / Requirements and scope | [ข้อกำหนด](requirements.md) | [Requirements](requirements.en.md) |
| การตั้งค่าโดย AI agent / AI-agent configuration | [คู่มือ AI agent](ai-agent.md) | [AI-agent guide](ai-agent.en.md) |

- [Node configuration example](../examples/docker-react.json)
- [Go configuration example](../examples/docker-go.json)
- [Docker checks validation](evidence/docker-checks-validation.md)
- [Docker checks design](superpowers/specs/2026-10-07-docker-checks-design.md)
- [Docker checks implementation plan](superpowers/plans/2026-10-07-docker-checks.md)

## งาน config/agent ปัจจุบัน / Current config-agent work

Updated 2026-10-09: `init`, `validate` and `skill install` are present in the current checkout. The [config-agent validation evidence](evidence/config-agent-validation.md) records current runtime and focused package-manager checks; the [config-agent trial evidence](evidence/config-agent-trials/README.md) records 23 independent evaluator attempts and their prerequisite findings. The design and plan remain the contract record; use the current guides for command behavior and the roadmap for deferred work.

- [Config initialization, validation and agent skill design](superpowers/specs/2026-10-08-config-agent-design.md)
- [Implementation plan with validation/security acceptance checks](superpowers/plans/2026-10-08-config-agent.md)
- [Development roadmap and deferred scope](roadmap.md)
- [Whole-branch review and fix resolutions](evidence/config-agent-review.md)
- [Closed test-gap follow-up and independent review](evidence/config-agent-test-gaps.md)
- [Actual terminal output: passing, failing and invalid-config demos](evidence/config-agent-terminal-demo.md)

หลักฐาน agent trial อยู่ใน [ดัชนีหลักฐาน config-agent](evidence/config-agent-trials/README.md); runtime/toolchain verification ต้องมีหลักฐานการรันจริงแยกต่างหาก / Agent-trial evidence is in the [config-agent evidence index](evidence/config-agent-trials/README.md); runtime/toolchain verification requires separate actual-run evidence.

## เอกสารประวัติ / Historical documents

`design/`, แผนก่อน Docker checks และหลักฐานอื่นใน `evidence/` บันทึกระบบก่อนหน้า อาจมีชื่อเครื่องมือ, config schema หรือคำสั่งที่เลิกใช้แล้ว ใช้เพื่อดูที่มาของงานเท่านั้น ให้ใช้คู่มือด้านบนสำหรับการติดตั้งและใช้งานปัจจุบัน

`design/`, plans preceding Docker checks and other files in `evidence/` record earlier implementations. Tool names, schemas and commands in those records may be obsolete. Use the current guides above for installation and operation.

- [Original design](design/pitstop-spec.md)
- [Original implementation plan](design/pitstop-implementation-plan.md)
- [Earlier validation](evidence/final-validation.md)
- [Earlier review](evidence/final-review.md)
