# เอกสาร Pitstop / Documentation

## เอกสารสำหรับ CLI ปัจจุบัน / Current CLI

| เอกสาร / Document | ไทย | English |
| --- | --- | --- |
| ติดตั้งและเริ่มใช้งาน / Install and quick start | [README](../README.th.md) | [README](../README.md) |
| คู่มือและ config reference / User guide and configuration | [คู่มือ](user-guide.md) | [Guide](user-guide.en.md) |
| ข้อกำหนดและขอบเขต / Requirements and scope | [ข้อกำหนด](requirements.md) | [Requirements](requirements.en.md) |

- [Node configuration example](../examples/docker-react.json)
- [Go configuration example](../examples/docker-go.json)
- [Docker checks validation](evidence/docker-checks-validation.md)
- [Docker checks design](superpowers/specs/2026-10-07-docker-checks-design.md)
- [Docker checks implementation plan](superpowers/plans/2026-10-07-docker-checks.md)

## เอกสารประวัติ / Historical documents

`design/`, แผนก่อน Docker checks และหลักฐานอื่นใน `evidence/` บันทึกระบบก่อนหน้า อาจมีชื่อเครื่องมือ, config schema หรือคำสั่งที่เลิกใช้แล้ว ใช้เพื่อดูที่มาของงานเท่านั้น ให้ใช้คู่มือด้านบนสำหรับการติดตั้งและใช้งานปัจจุบัน

`design/`, plans preceding Docker checks and other files in `evidence/` record earlier implementations. Tool names, schemas and commands in those records may be obsolete. Use the current guides above for installation and operation.

- [Original design](design/pitstop-spec.md)
- [Original implementation plan](design/pitstop-implementation-plan.md)
- [Earlier validation](evidence/final-validation.md)
- [Earlier review](evidence/final-review.md)
