# Pitstop

[English](README.md) · **ไทย**

รัน lint, test และ build ของโปรเจกต์ใน Linux container ใหม่ โดยใช้ไฟล์ที่กำลังแก้บนเครื่อง แล้วเก็บ log กับ report ไว้ตรวจสอบก่อน commit หรือ push

**เส้นทางสั้น:** ติดตั้ง CLI → เตรียม `.pitstop.json` → validate → ตรวจความพร้อม → run

## Pitstop ทำอะไรให้

คุณเลือก Docker image และ checks เอง ส่วน Pitstop จะ snapshot working tree, สร้าง container ใหม่ต่อ check, รัน steps ตามลำดับ, เก็บหลักฐาน และ cleanup ไฟล์ที่แก้แล้วแต่ยังไม่ commit ไฟล์ใหม่ที่ไม่ถูก ignore และไฟล์ที่ลบจะถูกรวมด้วย คำสั่งของโปรเจกต์รันใน container และไม่เขียนกลับ repo ต้นฉบับ

## 1. ติดตั้ง CLI

binary ที่เผยแพร่รองรับ macOS/Linux arm64/amd64 วิธีหลักใช้ installer ผ่าน HTTPS สาธารณะ ต้องมี curl กับเครื่องมือ SHA-256 แต่ไม่ต้องมี Go หรือ GitHub CLI อ่าน installer ก่อนใช้ และหยุดทันทีหากดาวน์โหลดไม่สำเร็จ:

```sh
set -eu
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
cat "$PITSTOP_INSTALLER"
```

อ่าน script แล้วจึงรัน block ถัดไป:

```sh
sh "$PITSTOP_INSTALLER"
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

[ลิงก์ `install.sh` โดยตรง](https://github.com/captainthx/pitstop/releases/latest/download/install.sh) ใช้ script เดียวกัน ส่วน [ติดตั้งผ่าน GitHub CLI](docs/install.md#authenticated-github-cli-route) และ [build จาก source](#build-from-source) เป็นทางเลือก ดูรายละเอียดติดตั้ง อัปเกรด และขอบเขตความเชื่อถือใน [docs/install.md](docs/install.md) ตัวติดตั้งตรวจ checksum/เวอร์ชันก่อนแทนที่ ไม่ใช้ sudo ไม่แก้ shell profile และต้องใช้ `--force` เมื่อจะแทนที่ binary เดิม

อัปเดตครั้งถัดไปด้วย `pitstop update` เพื่อเช็ก stable ล่าสุดและอัปเดต CLI พร้อม skills ของ user/โปรเจกต์ปัจจุบันที่ติดตั้งไว้ คำสั่งนี้มีใน source build ปัจจุบัน แต่ยังไม่มีใน binary v0.1.0; การอัปเกรดครั้งแรกจากรุ่นนั้นยังใช้ installer ดู[วิธีอัปเดตและขอบเขต](docs/install.md#อัปเกรด--upgrade)

### Build from source

ใช้ Git และ Go **1.26.5** ตาม `go.mod`:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop --help
```

Go บน host ใช้ build Pitstop เท่านั้น เครื่องมือที่ checks ใช้ต้องอยู่ใน Docker image ดู [สิ่งที่ต้องเตรียม](docs/user-guide.md#สิ่งที่ต้องเตรียม)

## 2. ตั้งค่า repo เป้าหมาย

กำหนด path แล้ว preview ก่อนเขียนไฟล์:

```sh
TARGET_REPO="/absolute/path/to/your/repo"
pitstop init --repo "$TARGET_REPO" --preset base --dry-run --json
```

ตรวจ preview แล้วเอา `--dry-run` ออกเพื่อเขียน starter:

```sh
pitstop init --repo "$TARGET_REPO" --preset base --json
```

`base` เป็น starter ที่มี failure placeholder แก้ `.pitstop.json` ให้เป็น verify command จริงก่อน validate หรือ run preset ที่มีให้ครอบคลุม Node, Go, Python, Java, .NET, Rust, PHP และ Ruby ส่วน `custom` ต้องมี `--image` และ `--command` อย่างน้อยหนึ่งครั้ง ถ้ามี `.pitstop.json` อยู่แล้ว `init` จะรักษาไฟล์เดิม ใช้ [ตัวอย่าง Node](examples/docker-react.json) หรือ [ตัวอย่าง Go](examples/docker-go.json) เมื่อต้องแก้เอง

ตัวอย่าง preset ยังต้องมี prerequisite ของโปรเจกต์ เช่น Node example ต้องมี `package-lock.json`, image ที่มี Node และ script ที่อ้างถึงใน `package.json` การ run checks ต้องมี Git, Docker CLI และ daemon ที่เชื่อมต่อได้ repo เป้าหมายต้องมี Git `HEAD` ก่อน `run` ตรวจ architecture ของ Docker daemon ด้วย `docker version --format '{{.Server.Arch}}'` แล้วเลือก `linux/arm64` หรือ `linux/amd64` ให้ตรง

## 3. Validate ตรวจความพร้อม แล้ว run

`validate` ตรวจ config แบบ offline ไม่ใช้ Docker ไม่ pull image ไม่เขียน cache และไม่ execute project commands จากนั้นใช้ `doctor` ตรวจ host/Docker/image readiness และใช้ `run` ตรวจจริง:

```sh
pitstop validate --repo "$TARGET_REPO" --json
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO" --json
pitstop run --repo "$TARGET_REPO" --json > result.json 2> progress.log
```

`run` ใช้ jobs ใน config เว้นแต่ `--job` จะเลือก IDs ที่มีอยู่ ค่า timeout เริ่มต้นคือ **20 นาทีต่อ check** `--arch` override เฉพาะครั้งนั้น Ctrl+C หรือ SIGTERM ยกเลิก run เมื่อใช้ `--json` ผลอยู่บน stdout และ progress อยู่บน stderr

### ตารางคำสั่ง

| คำสั่ง | หน้าที่ |
| --- | --- |
| `pitstop --help`, `pitstop -h` | แสดง root help |
| `pitstop help [command]`, `<command> --help` | แสดง help ของคำสั่ง |
| `pitstop init` | สร้าง config; รองรับ `--preset`, `--image`, `--command` ซ้ำได้, `--arch`, `--dry-run`, `--json` |
| `pitstop validate` | ตรวจ config; รองรับ `--job`, `--arch`, `--json` |
| `pitstop doctor` | ตรวจ repository, config, Git/Docker และ image; รองรับ `--json` |
| `pitstop list` | แสดง checks และ selection; รองรับ `--json` |
| `pitstop run` | รัน checks ที่เลือก; รองรับ `--job`, `--timeout`, `--arch`, `--allow-testcontainers`, `--json` |
| `pitstop update` | ตรวจ stable ล่าสุด แล้วอัปเดต CLI และ skills ที่ติดตั้งอยู่; รองรับ `--version`, `--json` |
| `pitstop version` | แสดง version/build metadata; รองรับ `--json` |
| `pitstop skill` | จัดการ agent skills ที่ติดตั้ง |
| `pitstop skill install` | ติดตั้ง config skill แบบ optional; รองรับ `--agent`, `--scope`, `--dry-run`, `--force`, `--json` |
| `pitstop completion bash\|zsh\|fish\|powershell` | สร้าง shell completion; แต่ละตัวรองรับ `--no-descriptions` |

flags ที่พบบ่อยคือ `--repo string` (global ค่าเริ่มต้น `.`), `--json`, `--job`, `--arch`, `--timeout` (ค่าเริ่มต้น `20m0s`) และ `--help` ดู [reference คำสั่งและ flags ฉบับเต็ม](docs/user-guide.md#คำสั่งและ-flags) สำหรับชนิด ค่าเริ่มต้น และ error behavior

## 4. จุดเริ่มต้นสำหรับ agent แบบ optional

portable `pitstop-config` skill แยกจากการติดตั้ง CLI preview project install ก่อนเขียนไฟล์:

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --json
```

คำสั่งแรกเป็น preview; เอา `--dry-run` ออกแล้วรันซ้ำเพื่อ install skill ปัจจุบัน skill อ่านหลักฐานของ repo, รักษาหรือร่าง `.pitstop.json`, ตรวจด้วย `validate`/`list`/`doctor` และรันเฉพาะ checks ที่ผู้ใช้ขอ ไม่ฝัง AI provider ไม่ execute workflow YAML และไม่อ้างว่า runtime ผ่านโดยไม่มีหลักฐาน ถ้าได้รับคำขอ “Use Pitstop to test this project” agent ควรตรวจ CLI/config ก่อน แสดง commands ที่เลือก รัน scope ที่ขอโดยไม่ถามยืนยันซ้ำ แล้วอ่าน report และ log ของแต่ละ check ก่อนสรุป user scope ติดตั้งใน directory ของ agent ส่วน project scope ติดตั้งใต้ Git root `--force` เปลี่ยนได้เฉพาะ Pitstop bundle ที่เป็นของเรา binary ที่เผยแพร่ใน v0.1.0 ฝัง skill bundle v1 ส่วน bundle v2 ใน checkout นี้ต้อง fresh source build จนกว่าจะมี release ใหม่ ดู [คู่มือ AI agent](docs/ai-agent.md)

## 5. อ่านผลลัพธ์

อ่าน `reportPath`, log ของแต่ละ check และ warnings ร่วมกัน:

| Code | ความหมาย |
| --- | --- |
| `0` | checks ที่เลือกผ่านทั้งหมด และผลสมบูรณ์ |
| `1` | มี check ล้มเหลว รวม timeout หรือ runtime failure |
| `2` | config/environment ไม่พร้อม, outcome ไม่ทราบ หรือ log/report/cleanup ไม่สมบูรณ์โดยไม่มี failed check |
| `130` | ยกเลิก มีลำดับความสำคัญสูงสุด |

ขอบเขตที่รองรับคือ Docker images ที่เตรียมไว้, snapshot, shell steps ตามลำดับ, selection, timeout/cancellation และ reports ส่วน workflow YAML, matrix, job dependencies, generic workflow service containers, secret injection, cache volumes, auto-build image, artifact export และ native Windows ยังอยู่นอกขอบเขต ดู [คู่มือฉบับเต็ม](docs/user-guide.md) สำหรับรายงาน การแก้ปัญหา และข้อจำกัดทั้งหมด

## อ่านต่อ

- [คู่มือใช้งานและ command reference](docs/user-guide.md)
- [คู่มือการตั้งค่าโดย AI agent](docs/ai-agent.md)
- [สารบัญเอกสารที่รวม requirements, engineering records และ evidence](docs/README.md)

รองรับ Testcontainers ผ่าน DinD แบบ opt-in: ดู [การตั้งค่าและข้อกำหนด](docs/testcontainers.md) สำหรับ `checks[].testcontainers` และ `run --allow-testcontainers` ต้องอนุญาต privileged daemon แยกในแต่ละครั้ง
