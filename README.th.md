# Pitstop

[English](README.md) | **ไทย**

รัน lint, test และ build ของโปรเจกต์ใน Linux container ก่อน commit หรือ push โดยใช้ไฟล์ที่กำลังแก้บนเครื่อง แล้วเก็บ log กับรายงานผลไว้ให้ตรวจย้อนหลัง

**เริ่มต้น:** ติดตั้ง CLI → ใช้ `pitstop init --dry-run --json` หรือคัดลอกตัวอย่างไปยัง repo เป้าหมาย → ตรวจด้วย `pitstop validate --json` → `pitstop doctor` → `pitstop run`

**สถานะเครื่องมือ config (2026-10-09):** checkout ปัจจุบันมี `init`, `validate` และ `skill install` พร้อม starter สำหรับ Node, Go, Python, Java, .NET, Rust, PHP และ Ruby รวมถึง portable `pitstop-config` skill สำหรับ Codex และ Claude Code มีบันทึก trial ด้าน mapping/prerequisite 23 ครั้งใน [ดัชนีหลักฐาน config-agent](docs/evidence/config-agent-trials/README.md); ผล runtime ที่เลือกสำหรับ Node/Go/Python/Rust และ focused pnpm/Yarn Berry อยู่ใน [หลักฐาน local validation](docs/evidence/config-agent-validation.md) ส่วน findings ที่ตรวจพบปิดครบแล้วใน [สถานะ review ปัจจุบัน](docs/evidence/config-agent-review.md) รวมถึง [ผลติดตามข้อค้าง test](docs/evidence/config-agent-test-gaps.md) ผล local นี้ยังไม่ยืนยัน remote CI หรือ workflow parity ดู [คู่มือปัจจุบัน](docs/user-guide.md), [คู่มือ AI agent](docs/ai-agent.md) และ [แนวทางพัฒนาต่อ](docs/roadmap.md)

## Pitstop ทำอะไรให้

คุณกำหนด **environment** ผ่าน Docker image และ **คำสั่งตรวจ** ผ่าน config ส่วน Pitstop จัดการขั้นตอนที่เหลือ:

```text
working tree → snapshot → container ใหม่ต่อ check → รัน steps → เก็บผล → cleanup
```

- ตรวจไฟล์ที่แก้แล้วแต่ยังไม่ commit รวมถึงไฟล์ใหม่ที่ไม่ถูก ignore และไฟล์ที่ลบ
- รันเครื่องมือของโปรเจกต์ใน container; step ที่เขียนไฟล์จะไม่แก้ repo ต้นฉบับ
- รัน steps ตามลำดับ แชร์ไฟล์ภายใน check เดียวกัน และแยก filesystem ของแต่ละ check
- จัดการ timeout, Ctrl+C, log และ JSON report พร้อมหลักฐานแต่ละ step

CLI นี้รันคำสั่งใน `.pitstop.json` โดยตรง ดูขอบเขตการรองรับใน [คู่มือ](docs/user-guide.md#ขอบเขตที่รองรับ)

## 1. ติดตั้ง CLI

ติดตั้งจาก [GitHub Releases](https://github.com/captainthx/pitstop/releases) บน macOS/Linux arm64/amd64 โดยไม่ต้องมี Go ตัวติดตั้งเลือก stable release ล่าสุด หรือเลือก build จาก source ด้านล่างได้

ดาวน์โหลดผ่านลิงก์ได้โดยไม่ต้องล็อกอินหรือมี GitHub CLI อ่าน script ก่อนรัน และหยุดหากดาวน์โหลดล้มเหลว:

```sh
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
cat "$PITSTOP_INSTALLER"
```

```sh
sh "$PITSTOP_INSTALLER"
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

ใช้ GitHub CLI เป็นทางเลือกได้ ดู [คู่มือติดตั้ง](docs/install.md#authenticated-github-cli-route)

ตัวติดตั้งตรวจ checksum และเวอร์ชันก่อนแทนที่ binary ไฟล์เดิมต้องอนุญาตด้วย `--force` ไม่ใช้ sudo หรือแก้ shell profile ดู [วิธีติดตั้ง อัปเกรด ลบ และขอบเขตความเชื่อถือ](docs/install.md) การรัน checks ยังต้องมี Git และ Docker; checksum ไม่ป้องกันแหล่งเผยแพร่ที่ถูกยึด และ binary macOS ยังไม่ notarize

### Build from source

ใน checkout ของ Pitstop ใช้ Git, Go **1.26.5** ตาม `go.mod` และ Docker CLI/daemon สำหรับรัน checks:

```sh
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/pitstop" ./cmd/pitstop
export PATH="$HOME/.local/bin:$PATH"
pitstop version
pitstop -h
```

เพิ่มบรรทัด `export PATH="$HOME/.local/bin:$PATH"` ในไฟล์ตั้งค่า shell หากต้องการให้ใช้ได้ใน terminal ครั้งถัดไป วิธี build จาก source ใช้ได้ก่อนเผยแพร่ release และเป็นทางเลือกแทน binary สำเร็จรูป

Go บน host ใช้ build ตัว Pitstop เท่านั้น เครื่องมือที่ check ใช้ เช่น Node, Java หรือ Go ต้องอยู่ใน Docker image [รายละเอียดสิ่งที่ต้องเตรียม](docs/user-guide.md#สิ่งที่ต้องเตรียม)

## 2. สร้าง config ครั้งเดียวต่อ repo

ใช้ `pitstop init` เพื่อสร้าง starter หรือให้ skill อ่านหลักฐานของ repo แล้วร่าง config `init` จะไม่เขียนทับ `.pitstop.json` ที่มีอยู่ ใช้ `--dry-run --json` เพื่อตรวจ starter ก่อนเขียน

รันจาก checkout ของ Pitstop โดยเปลี่ยน path เป็น repo ที่ต้องการตรวจ:

```sh
PITSTOP_SOURCE="$PWD"
TARGET_REPO="/absolute/path/to/your/repo"
if [ -e "$TARGET_REPO/.pitstop.json" ] || [ -L "$TARGET_REPO/.pitstop.json" ]; then
  echo "เก็บ .pitstop.json เดิมไว้ ให้แก้ไขโดยตั้งใจ"
else
  cp "$PITSTOP_SOURCE/examples/docker-react.json" "$TARGET_REPO/.pitstop.json"
fi
```

ถ้ามี config อยู่แล้ว ให้เก็บไฟล์เดิมและแก้โดยตั้งใจ เลือก preset ด้วย `--preset` หรือใช้ `custom` พร้อม `--image` และ `--command` อย่างน้อยหนึ่งครั้ง ชื่อ preset ทั้งหมดดูได้จาก `pitstop init -h` หากต้องการเส้นทาง manual ให้คัดลอก [ตัวอย่าง Node](examples/docker-react.json) หรือ [ตัวอย่าง Go](examples/docker-go.json) แล้วแก้ให้ตรงกับโปรเจกต์

preview ของ base:

```sh
pitstop init --repo "$TARGET_REPO" --preset base --dry-run --json
```

ตัวอย่าง Node คือ:

```json
{
  "schemaVersion": 2,
  "runner": {"image": "node:22-bookworm", "architecture": "linux/arm64"},
  "jobs": ["verify"],
  "checks": [{"id": "verify", "steps": [
    {"id": "install", "name": "Install dependencies", "run": "npm ci"},
    {"id": "check", "name": "Run project checks", "run": "npm run check"}
  ]}]
}
```

| ส่วน | ต้องตั้งอะไร |
| --- | --- |
| `runner.image` | image ที่มีเครื่องมือครบ เช่น Node สำหรับ `npm` |
| `runner.architecture` | `linux/arm64` หรือ `linux/amd64` ให้ตรงกับ environment ที่จะใช้ |
| `checks` | กลุ่มคำสั่งตรวจ แต่ละกลุ่มได้ container ใหม่ |
| `checks[].steps` | คำสั่งในกลุ่ม รันตามลำดับ |
| `jobs` | IDs ของ checks ที่จะรันโดยปริยาย และลำดับการรัน |

ตัวอย่าง Node ต้องมี `package-lock.json` และ script `check` ใน `package.json` ถ้าโปรเจกต์ใช้คำสั่งอื่น ให้แก้ `run` เช่น `npm test` หรือ `sh scripts/check.sh` ตัวอย่างไม่สร้าง project scripts ให้

ตรวจ architecture ของ Docker daemon ด้วย `docker version --format '{{.Server.Arch}}'`: `arm64`/`aarch64` ใช้ `linux/arm64`; `amd64`/`x86_64` ใช้ `linux/amd64` repo ต้องมี commit อย่างน้อยหนึ่งรายการก่อน `run` แนะนำ commit config และ shared scripts เพื่อให้ทีมใช้เหมือนกัน

## 3. Validate ตรวจความพร้อม แล้วรัน

`validate` ตรวจ config และ jobs ที่เลือกโดยไม่ใช้ Docker, pull image, เขียน cache หรือรันคำสั่งของโปรเจกต์ ผล JSON มี envelope เดียวพร้อม `valid`, jobs, architecture, จำนวน checks/steps และ issue ที่จำกัดขนาด

```sh
pitstop validate --repo "$TARGET_REPO" --json
```

`doctor` ตรวจ Git, Docker และความพร้อมของ image ส่วน `run` รัน checks ใน container การผ่าน validation ไม่ได้ยืนยันว่า command, image, dependencies หรือ workflow CI จะทำงานสำเร็จ

```sh
pitstop doctor --repo "$TARGET_REPO"
pitstop list --repo "$TARGET_REPO"
pitstop run --repo "$TARGET_REPO"
```

`doctor` ตรวจ config, Docker และ image โดยไม่ pull หรือเปิด container ส่วน `run` จะ pull image เมื่อจำเป็น แล้วรัน checks หาก image ยังไม่มี `doctor` อาจรายงานพร้อมพร้อมข้อความว่าจะ pull ตอน run; ไม่ได้ยืนยันว่า image มีคำสั่งของโปรเจกต์ครบ

ติดตั้ง portable skill ในตำแหน่ง user หรือ project ที่แยกไว้:

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
```

user scope ใช้ `~/.agents/skills/pitstop-config` สำหรับ Codex หรือ `~/.claude/skills/pitstop-config` สำหรับ Claude ส่วน project scope ใช้ directory ซ่อนที่ตรงกับ agent ใต้ Git root `--force` จะเปลี่ยนได้เฉพาะ bundle ของ Pitstop ที่ manifest ที่เชื่อถือได้ยืนยันความเป็นเจ้าของ

ตัวเลือกที่ใช้บ่อย:

```sh
pitstop run --repo "$TARGET_REPO" --job verify
pitstop run --repo "$TARGET_REPO" --timeout 10m
pitstop run --repo "$TARGET_REPO" --json > result.json 2> progress.log
pitstop run -h
```

`--job` เลือกจาก IDs ใน `jobs` เท่านั้น ค่า timeout เริ่มต้น **20 นาทีต่อ check** กด Ctrl+C เพื่อยกเลิก ระบบพยายามหยุดและลบเฉพาะ container ของ run นั้น หาก cleanup ไม่สำเร็จให้ดู warning ใน report

## 4. อ่านผล

เมื่อเริ่มรัน checks แล้ว ผลจะมี `reportPath` และ log ของแต่ละ check รายงานเก็บใน user cache ส่วน snapshot และ scripts ชั่วคราวถูกลบ หาก config หรือ environment ผิดตั้งแต่ก่อนเริ่ม อาจได้ error โดยไม่มี run report

| Exit code | ความหมาย |
| --- | --- |
| `0` | ทุก check ที่เลือกผ่าน และไม่มีปัญหาที่ทำให้ผลไม่สมบูรณ์ |
| `1` | มี check ล้มเหลว รวม timeout หรือ container runtime failure |
| `2` | config/environment ไม่พร้อม ผลไม่ทราบแน่ชัด หรือมีปัญหา log/report/cleanup โดยไม่มี failed check |
| `130` | ยกเลิกการทำงาน มีลำดับความสำคัญสูงสุด |

ดู [วิธีอ่าน report และแก้ปัญหา](docs/user-guide.md#อ่าน-report-และ-log) ก่อนสรุปว่าทุก check ผ่าน

## อ่านต่อ

- [คู่มือครบ: config, shared scripts, custom image, commands, reports และ troubleshooting](docs/user-guide.md)
- [ขอบเขตและข้อกำหนด](docs/requirements.md)
- [การตั้งค่าโดย AI agent](docs/ai-agent.md)
- [Design งาน config/agent](docs/superpowers/specs/2026-10-08-config-agent-design.md), [implementation plan](docs/superpowers/plans/2026-10-08-config-agent.md) และ [ขอบเขตที่เก็บไว้พัฒนาต่อ](docs/roadmap.md)
- [สารบัญเอกสาร](docs/README.md) และ [หลักฐานทดสอบ Docker ปัจจุบัน](docs/evidence/docker-checks-validation.md)
