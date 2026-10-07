> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../README.th.md) สำหรับการใช้งาน Docker checks.

# act React CI — ผลทดลอง

วันที่ 2026-10-05 (Asia/Bangkok)

รัน job `verify` จาก `.github/workflows/ci.yml` ของ recruit100-react สำเร็จทั้งสองรอบ ไม่แก้ workflow และไม่ส่ง secrets/vars/env files เข้าไป

| รอบ | เวลารวม | Exit code |
|---|---:|---:|
| รอบแรก รวมดาวน์โหลด image/actions/dependencies | 111.08 วินาที | 0 |
| รอบที่สอง ใช้ cached image/actions/Node/pnpm store | 45.86 วินาที | 0 |

## ผ่านครบตาม workflow

- ATS lint
- ATS typecheck
- ATS Vite build
- Recruit100 lint
- Recruit100 typecheck
- Recruit100 tests: 2 files, 15 tests ผ่านทั้งหมด
- Recruit100 OpenNext Worker build

## Environment และขอบเขต

- act 0.2.89 ติดตั้งผ่าน Homebrew ในการทดลองนี้
- Linux ARM64 Docker runner: `catthehacker/ubuntu:act-latest`
- Image digest: `sha256:c58e2b364da03b0c804c7d660f2ecbedf2f221a382b9baa0b344b0144780ff43`
- Node 20.20.2 ตาม major version ที่ workflow ระบุ; pnpm 12.5.1 ตาม packageManager
- Commit: `c65e803404e1538c96d2162014eb7e03ee942881`
- ใช้ local clone ของ repo ที่ working tree สะอาด; ไม่คัดลอก ignored local env หรือ node_modules
- act ใช้การ copy source แทน checkout action ตามพฤติกรรม default
- ตรวจหลังรันว่า checkout ต้นฉบับยังสะอาด และ workflow ตรงกับสำเนาทุก byte
- ไม่ deploy; runner container ถูก cleanup เมื่อจบ ไม่แตะ containers PostgreSQL/MinIO ที่มีอยู่
- เก็บ act, downloaded runner image และ action/dependency caches ไว้สำหรับการรันถัดไป

## สิ่งที่เรียนรู้สำหรับ Pitstop

1. React profile ใช้ workflow เดิมได้โดยไม่ต้องปรับ steps และไม่ต้องใช้ secrets
2. Node/pnpm dependency cache ของ act ใช้งานได้จริง (`cache-hit=true` ในรอบที่สอง)
3. รอบ warm ยังติดตั้ง dependencies จาก store และ build ใหม่ ไม่ได้ cache ทุกอย่าง: Next.js ยังรายงาน No build cache found
4. ต้องแสดง setup overhead แยกจากเวลา validation: รอบแรก 111.08 วินาที แต่ cached run 45.86 วินาที
5. ยังไม่ได้เทียบกับเวลา GitHub CI หรือทดสอบ AMD64/Go integration และยังไม่ได้พิสูจน์ uncommitted/untracked input เพราะ repo สะอาดในการทดลองนี้
6. มี Vite bundle-size และ Node deprecation warnings แต่ไม่ได้ทำให้ job ล้มเหลว

## คำสั่งสำหรับเรียกเอง

คำสั่งนี้ใช้ repo ต้นฉบับเป็น input และ act copy source เข้า runner container; ไม่จำเป็นต้องเปิด PR ก่อน

```sh
rtk proxy act pull_request \
  -C /Users/thanyatab/Desktop/project/techcombine/recruit100-react \
  -W .github/workflows/ci.yml -j verify \
  -P ubuntu-latest=catthehacker/ubuntu:act-latest \
  --container-architecture linux/arm64 \
  --env-file /dev/null --secret-file /dev/null \
  --var-file /dev/null --input-file /dev/null \
  --container-daemon-socket - --rm \
  --action-offline-mode --pull=false
```

`--action-offline-mode` ลดการ fetch/pull Actions และ images ที่มีแล้ว ไม่ได้ทำให้ทุก step ทำงานโดยไม่ใช้อินเทอร์เน็ต

## หลักฐาน

- `act-react-ci.log`: log เต็มรอบแรก
- `act-react-ci-warm.log`: log เต็มรอบที่สอง
- `act-react-ci-result.json` และ `act-react-ci-warm-result.json`: คำสั่ง, revision, เวลารวมและ exit code
