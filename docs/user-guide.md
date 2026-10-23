# คู่มือ Pitstop

[English](user-guide.en.md) | [เริ่มใช้งานแบบสั้น](../README.th.md) | [สารบัญเอกสาร](README.md)

คู่มือนี้อธิบาย CLI ที่รัน Docker checks ด้วย `.pitstop.json` schema version 2

สถานะ 2026-10-09: checkout ปัจจุบันมี `init`, `validate` และ `skill install` พร้อม `doctor`, `list`, `run` และ `version` ดูขั้นตอน portable skill ใน [คู่มือ AI agent](ai-agent.md) และ [หลักฐาน trial ของ config-agent](evidence/config-agent-trials/README.md) ซึ่งบันทึก mapping กับ prerequisite จาก evaluator 23 ครั้ง หลักฐานนี้ยังไม่ยืนยัน runtime หรือ CI parity ดูขอบเขตที่ยังไม่รวมใน [roadmap](roadmap.md)

ใน checkout มี [ตัวติดตั้งจาก release](install.md) สำหรับ macOS/Linux arm64/amd64 แล้ว เลือกติดตั้ง release ที่เผยแพร่หรือ build จาก source ได้ การติดตั้ง binary ใช้ curl และเครื่องมือ SHA-256 โดยไม่ต้องมี Go หรือ GitHub CLI ส่วนวิธีใช้ GitHub CLI ยังเป็นทางเลือกได้

## สิ่งที่ต้องเตรียม

| ส่วน | สิ่งที่ต้องมี |
| --- | --- |
| เครื่องที่เรียก Pitstop | Git, Docker CLI และ daemon ที่เชื่อมต่อได้ |
| Build CLI จาก source | Go 1.26.5 ตาม `go.mod` |
| Repo เป้าหมาย | Git repo; `run` ต้องมี commit อย่างน้อยหนึ่งรายการและ `.pitstop.json` ที่ root ส่วน `init` และ `validate` ใช้ได้ก่อน commit แรก |
| Docker image | Linux image ที่มี `/bin/sh` และเครื่องมือที่ steps ใช้ |
| การเชื่อมต่อ | เข้าถึง registry เมื่อยังไม่มี image และเข้าถึง package registry ถ้าคำสั่งติดตั้ง dependencies ต้องใช้ |

ปัจจุบันทดสอบการรันจริงบน macOS ARM64 กับ Docker daemon Linux ARM64 และมี Docker smoke ใน GitHub CI โค้ดใช้ Unix file locking; ยังไม่ยืนยันการรองรับ Windows แบบ native [หลักฐานปัจจุบัน](evidence/docker-checks-validation.md)

Docker CLI ต้องรองรับ `docker image inspect --platform` ตรวจด้วย `docker image inspect --help` หากใช้ installation เก่า

ติดตั้ง CLI ตาม [README](../README.th.md#1-ติดตั้ง-cli) แล้วตรวจ:

```sh
git --version
docker version
pitstop version
```

ใช้ Linux containers ตรวจ architecture ของ **daemon ที่จะใช้** ไม่ใช่ดูเฉพาะ CPU ของเครื่อง:

```sh
docker version --format '{{.Server.Arch}}'
```

`arm64`/`aarch64` → `linux/arm64`; `amd64`/`x86_64` → `linux/amd64` การรันข้าม architecture ขึ้นอยู่กับ emulation ของ Docker environment และยังไม่ได้รับประกันจากผลทดสอบ local

## ตั้งค่า repo ครั้งแรก

`pitstop init` สร้าง starter ที่ระบุชื่อโดยไม่ตรวจหา manifest อัตโนมัติ preview ก่อนเขียนได้ด้วย:

```sh
pitstop init --repo /absolute/path/to/your/repo --preset base --dry-run --json
```

ชื่อ preset ทั้งหมดดูได้จาก `pitstop init -h`; `custom` ต้องมี `--image` และ `--command` อย่างน้อยหนึ่งครั้ง หากมี `.pitstop.json` อยู่แล้ว init จะไม่เขียนทับ สำหรับเส้นทาง manual ให้คัดลอก [ตัวอย่าง Node](../examples/docker-react.json) หรือ [ตัวอย่าง Go](../examples/docker-go.json) แล้วปรับให้ตรงกับโปรเจกต์

ตัวอย่างนี้ใช้ script ของโปรเจกต์เอง:

```json
{
  "schemaVersion": 2,
  "runner": {
    "image": "node:22-bookworm",
    "architecture": "linux/arm64"
  },
  "jobs": ["verify"],
  "checks": [
    {
      "id": "verify",
      "steps": [
        {"id": "install", "name": "Install dependencies", "run": "npm ci"},
        {"id": "check", "name": "Check project", "run": "sh scripts/check.sh"}
      ]
    }
  ]
}
```

สร้าง `scripts/check.sh` ใน **repo เป้าหมาย** ถ้ายังไม่มี หรือใช้ script เดิมที่ทำหน้าที่นี้อยู่แล้ว:

```sh
#!/bin/sh
set -eu
npm run lint
npm test
npm run build
```

แต่ละคำสั่งต้องมีจริงใน `package.json` ตัวอย่างนี้ต้องมี lockfile สำหรับ `npm ci` เปลี่ยนตาม package manager/toolchain ของโปรเจกต์ แล้ว commit config และ script เพื่อใช้ร่วมกันในทีม

โครงสร้าง repo ตัวอย่าง:

```text
your-project/
├── .pitstop.json
├── package.json
├── package-lock.json
└── scripts/
    └── check.sh
```

ไม่ต้อง commit การแก้โค้ดทุกครั้งก่อน run: Pitstop ตรวจ working tree ปัจจุบัน แต่ repo ต้องมี HEAD อยู่แล้ว

## Validate และติดตั้ง agent skill

ให้รัน validation ก่อน `doctor` หรือ `run` คำสั่งนี้อ่าน `.pitstop.json` โดยไม่ใช้ Docker, pull image, เขียน cache, รัน project commands หรือประเมิน shell ถ้าถูกต้องจะได้ `valid: true`, jobs, architecture, จำนวน definitions และ `issues: []`; config ที่ไม่ถูกต้องคืน exit `2`, issue ที่จำกัดขนาดหนึ่งรายการ และ error code `CONFIG_INVALID`

```sh
pitstop validate --repo "$TARGET_REPO" --json
```

ติดตั้ง portable skill โดยระบุ agent และ scope ชัดเจน project scope หา Git root ส่วน user scope ใช้ home ของผู้ใช้และรับ `--repo` ไม่ได้ ใช้ `--dry-run` ใน target ที่แยกไว้เพื่อดูรายการไฟล์ `--force` เปลี่ยนได้เฉพาะ Pitstop bundle ที่ manifest ปัจจุบันหรือ historical manifest ที่เชื่อถือได้ยืนยันความเป็นเจ้าของ

```sh
pitstop skill install --agent codex --scope project --repo "$TARGET_REPO" --dry-run --json
pitstop skill install --agent claude --scope user --dry-run --json
```

user install ของ Codex อยู่ที่ `~/.agents/skills/pitstop-config` และของ Claude อยู่ที่ `~/.claude/skills/pitstop-config` ส่วน project install อยู่ใน directory ซ่อนของ agent ใต้ Git root การติดตั้งไม่แก้ `AGENTS.md`, `CLAUDE.md`, workflow หรือ project scripts ดูขั้นตอนและ prompt fallback ใน [คู่มือ AI agent](ai-agent.md)

## Config reference

ทุก field ด้านล่างจำเป็น ยกเว้น `name` ไม่มีค่า image/architecture/jobs เริ่มต้นให้

| Field | รูปแบบและความหมาย |
| --- | --- |
| `schemaVersion` | ต้องเป็น `2` |
| `runner.image` | ชื่อ image/tag/digest ที่มี tools ครบ ต้องไม่ว่าง ไม่มี whitespace/control characters และไม่ขึ้นต้นด้วย `-` |
| `runner.architecture` | `linux/arm64` หรือ `linux/amd64` |
| `jobs` | array ของ check IDs ที่เลือกโดยปริยาย ต้องไม่ว่าง ไม่ซ้ำ และมีอยู่ใน `checks`; ใช้ลำดับนี้ตอนรัน |
| `checks` | array ที่ไม่ว่าง; check IDs ต้องไม่ซ้ำ |
| `checks[].id` | ID ของ check ใช้อ้างใน `jobs` และ `--job` |
| `checks[].steps` | array ที่ไม่ว่าง รันตามลำดับ |
| `steps[].id` | ID ของ step ต้องไม่ซ้ำภายใน check เดียวกัน |
| `steps[].name` | ชื่อที่อ่านง่ายสำหรับ log/report; ไม่ใส่จะใช้ ID |
| `steps[].run` | shell command ต้องไม่ว่าง/มีแต่ช่องว่าง และไม่มี NUL; JSON ใช้ `\n` สำหรับคำสั่งหลายบรรทัด |

IDs ยาว 1–64 ตัวอักษร เริ่มด้วยตัวอักษรอังกฤษหรือตัวเลข ที่เหลือใช้ตัวอักษรอังกฤษ ตัวเลข `_` หรือ `-` ได้ เช่น `verify`, `unit-test`, `build_web`

Config ต้องเป็น JSON document เดียว ไม่มี comments และไม่รับ fields ที่ไม่รู้จัก เช่น `env`, `uses` หรือ `engine` ถ้าต้องกำหนด environment variable ให้กำหนดใน shell command/script ของ step ส่วนค่าใน shell ของ host ไม่ถูกส่งเข้า container อัตโนมัติ

คำสั่งใน `run` เป็น shell code ที่ตั้งใจ execute ใน container `pitstop validate` ใช้ strict parser และ bounded checks ร่วมกัน ตรวจ duplicate/unknown fields, malformed Unicode, unsafe file types, schema version ที่ไม่รองรับ และค่าที่เกิน limits การผ่าน validation ไม่ได้รับรองความปลอดภัยของ command/image, dependencies, runtime หรือความครบของ GitHub CI ดู [validation design](superpowers/specs/2026-10-08-config-agent-design.md)

**`jobs` คือ selection ไม่ใช่รายการ steps** คุณอาจนิยาม checks หลายกลุ่ม แต่เลือกรันเพียงบางกลุ่ม:

```json
"jobs": ["lint", "test"]
```

ตัวอย่างด้านบนเป็นส่วนหนึ่งของ config ไม่ใช่ config สมบูรณ์ ต้องมี checks ชื่อ `lint` และ `test` ด้วย `--job test` รันเฉพาะ `test`; `--job build` จะถูกปฏิเสธถ้า `build` ไม่ได้อยู่ใน `jobs` แม้จะนิยามไว้ใน `checks` แล้ว ต้องแก้ selection ใน config ก่อน

### ตัวอย่างแยก lint และ test

ถ้าต้องการเลือกตรวจแต่ละกลุ่มแยกกัน ใช้ config นี้ โดยสมมติว่าโปรเจกต์มี scripts `lint` และ `test`:

```json
{
  "schemaVersion": 2,
  "runner": {
    "image": "node:22-bookworm",
    "architecture": "linux/arm64"
  },
  "jobs": [
    "lint",
    "test"
  ],
  "checks": [
    {
      "id": "lint",
      "steps": [
        {
          "id": "install",
          "run": "npm ci"
        },
        {
          "id": "lint",
          "run": "npm run lint"
        }
      ]
    },
    {
      "id": "test",
      "steps": [
        {
          "id": "install",
          "run": "npm ci"
        },
        {
          "id": "test",
          "run": "npm test"
        }
      ]
    }
  ]
}
```

ทั้งสอง check ต้องติดตั้ง dependencies เอง เพราะได้ container ใหม่คนละตัว ถ้าต้องการติดตั้งครั้งเดียวแล้วตรวจทั้งหมด ให้รวม steps ใน check `verify` แบบตัวอย่างแรก

## Steps และ container ทำงานอย่างไร

1. ตรวจ config/environment และล็อก repo ป้องกัน run ซ้อนใน repo เดียวกัน
2. เก็บ snapshot ของ working tree; ถ้าข้อมูลเปลี่ยนระหว่าง capture จะหยุดด้วย `SNAPSHOT_CHANGED`
3. ตรวจ image และ pull เมื่อไม่มีหรือ platform ไม่ตรง มีเวลาเตรียมสูงสุด 2 นาที
4. สร้าง container ใหม่ต่อ check แล้ว copy snapshot เข้า `/workspace` โดยไม่ bind mount repo หรือ Docker socket
5. รันแต่ละ step ผ่าน `/bin/sh -e` ที่ `/workspace` ด้วย user `0:0` (root ภายใน container)
6. เก็บ exit records, terminal state และ logs แล้วลบ container/snapshot/scripts ชั่วคราว

Steps ใน check เดียวกันแชร์ไฟล์ เช่น `npm ci` สร้าง `node_modules` แล้ว test ใช้ต่อได้ แต่ **แต่ละ step เป็น shell ใหม่** `cd` หรือ `export` ใน step ก่อนหน้าไม่ส่งต่อ:

```json
{"id":"test","run":"cd frontend && npm test"}
```

ถ้าหลายคำสั่งต้องใช้ตัวแปรหรือ directory เดียวกัน ให้รวมใน step/script เดียว `/bin/sh -e` ไม่ได้เปิด `pipefail`; pipeline ที่ต้องตรวจทุกคำสั่งควรเขียน script ให้ตรวจ exit codes เอง หรือเรียก shell ที่มีใน image พร้อม options ที่ต้องการ

ถ้า step ล้มเหลว steps ที่เหลือของ check นั้นจะ `skipped` แต่ check ถัดไปยังรันต่อและได้ snapshot เริ่มต้นใหม่ เมื่อยกเลิก run จะไม่ schedule checks ที่เหลือ

ไม่มี cache volume ข้าม check/run และ build artifacts ใน container จะถูกลบหลังจบ รุ่นนี้ยังไม่มีคำสั่ง export artifacts กลับ repo

## เลือก image หรือสร้าง image เอง

เริ่มจาก image ที่มี tools ของโปรเจกต์อยู่แล้ว เช่นตัวอย่าง Node หรือ Go ใน repo การติดตั้ง dependencies เป็นคำสั่งใน steps; Pitstop ไม่เดาว่าต้องใช้ `npm ci`, `pnpm` หรือคำสั่งใด

ถ้าต้องมีเครื่องมือเพิ่มเติม ให้สร้าง `Dockerfile.checks` ใน repo เป้าหมาย เช่นโปรเจกต์ Node ที่ต้องเรียก Git:

```dockerfile
FROM node:22-bookworm
RUN apt-get update \
    && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
```

Build เองก่อน run:

```sh
docker build -t my-project-checks:dev -f Dockerfile.checks .
```

ตั้ง `runner.image` เป็น `my-project-checks:dev` และ architecture ให้ตรงกับ image ที่ build แล้ว Pitstop จะใช้ image ที่มีใน daemon โดยไม่ build ให้ เมื่อ Dockerfile เปลี่ยนต้อง build ใหม่

Tag อาจเปลี่ยนได้ หากต้องการตรึง toolchain ใช้ digest จริงจาก registry เช่น `your-image@sha256:<digest>` โดยแทน `<digest>` ด้วยค่าจริง รายงานบันทึก resolved platform image ID และ digest เมื่อมี

Private registry ใช้การ login ของ Docker CLI บน host ก่อนเริ่ม run; credentials ของ host ไม่ถูก mount เข้า check container

## คำสั่งและ flags

`--repo string` เป็น persistent global flag ของทุกคำสั่ง ค่าเริ่มต้น `.` และ resolve เป็น Git root; Pitstop อ่าน `.pitstop.json` จาก root นี้และไม่ค้นหา config ที่อื่น ใช้ quotes เมื่อ path มีช่องว่าง คำสั่ง `skill install` ใช้ `--repo` เฉพาะ `--scope project`; ถ้าเป็น `--scope user` อย่าส่ง `--repo` แบบ explicit เพราะจะถูกปฏิเสธ

| คำสั่ง | หน้าที่ | Flags เพิ่มเติม |
| --- | --- | --- |
| `pitstop help [command]` | แสดง help ของคำสั่งที่ระบุ | `-h`, `--help` |
| `pitstop init` | สร้าง starter `.pitstop.json` | `--preset`, `--image`, `--command` (ซ้ำได้), `--arch`, `--dry-run`, `--json` |
| `pitstop validate` | ตรวจ config โดยไม่รัน checks | `--job`, `--arch`, `--json` |
| `pitstop doctor` | ตรวจ repository/config, Git/Docker และ image readiness | `--json` |
| `pitstop list` | แสดง checks ทั้งหมดพร้อม selected/excluded จาก config; ไม่ต้องเปิด Docker | `--json` |
| `pitstop run` | รัน checks ตาม selection | `--job`, `--timeout`, `--arch`, `--json` |
| `pitstop version` | แสดง version และ build information เมื่อมี | `--json` |
| `pitstop skill install` | ติดตั้ง portable Pitstop config skill ให้ agent | `--agent`, `--scope`, `--dry-run`, `--force`, `--json` |
| `pitstop completion <shell>` | สร้าง shell completion script | `bash`, `zsh`, `fish`, `powershell`; แต่ละตัวมี `--no-descriptions` |

```sh
pitstop help run
pitstop init --repo "/path/to/project" --preset node-npm --dry-run --json
pitstop validate --repo "/path/to/project" --job lint --arch linux/amd64 --json
pitstop doctor --repo "/path/to/project"
pitstop list --repo "/path/to/project" --json
pitstop run --repo "/path/to/project" --job lint --job test
pitstop run --repo "/path/to/project" --job lint,test --timeout 10m
pitstop run --repo "/path/to/project" --arch linux/amd64
pitstop run --repo "/path/to/project" --json > result.json 2> progress.log
pitstop version --json
pitstop skill install --agent codex --scope project --repo "/path/to/project" --dry-run --json
pitstop completion bash --no-descriptions > ~/.local/share/bash-completion/completions/pitstop
```

ทุกคำสั่งและ subcommand มี `-h`/`--help`; `pitstop completion` มี subcommands `bash`, `zsh`, `fish` และ `powershell` ซึ่งแต่ละตัวมีวิธีโหลด script ของ shell นั้นเองตาม help

### รายละเอียดคำสั่งที่ใช้บ่อย

`init` ใช้ preset ต่อไปนี้: `base`, `node-npm`, `node-pnpm`, `node-yarn-classic`, `node-yarn-berry`, `go`, `python-pip`, `python-uv`, `java-maven`, `java-gradle`, `dotnet`, `rust`, `php-composer` และ `ruby-bundler` ค่าเริ่มต้นคือ `base` โดย `base` จะสร้าง starter ที่มี failure placeholder ให้แก้ก่อน run `custom` ต้องส่ง `--image` และ `--command` อย่างน้อยหนึ่งครั้ง `--command` เป็น StringArray: ส่งซ้ำได้และคำสั่งที่ส่งจะ **แทนที่** verify steps ของ preset ส่วน `--image` แทน runner image

`init --arch` รับเฉพาะ `linux/arm64` หรือ `linux/amd64` ถ้าไม่ระบุจะเดาจาก host runtime (`arm64`/`amd64`) ไม่ได้ตรวจ architecture ของ Docker daemon ระยะไกล การเขียนจริงสร้าง `.pitstop.json` ใหม่เท่านั้น: ถ้ามีอยู่แล้วจะคงไฟล์เดิมและคืน `CONFIG_EXISTS`; `--dry-run` แสดง config ที่จะสร้างและไม่เขียนไฟล์ การยกเลิกหรือ write error อาจเหลือไฟล์บางส่วน ให้ตรวจ `.pitstop.json` ก่อน retry

`validate` ใช้ parser เดียวกับ config และรองรับ `--job` กับ `--arch` เป็น effective-config ชั่วคราวโดยไม่แก้ไฟล์ ไม่ใช้ Docker ไม่ pull image ไม่เขียน cache และไม่รัน project command `--job` ต้องเป็น ID ที่อยู่ใน `jobs` และไม่ซ้ำ; selection หรือ architecture ที่ผิดคืน exit `2` พร้อม `CONFIG_INVALID`/issue

`doctor` ตรวจ repository, config, Git/Docker และความพร้อมของ image โดยไม่ pull image และไม่รัน project commands `list` อ่าน config แล้วแสดงทุก check ว่า `selected` หรือ `excluded` โดยไม่ต้องเปิด Docker ทั้งสองคำสั่งรองรับเฉพาะ `--json` นอกเหนือจาก help

`version` แบบ human แสดง `pitstop <version>` ส่วน `--json` คืน envelope ที่มี `version` และมี `commit`/`buildTime` เมื่อ binary ถูก build พร้อม metadata

`skill install` เป็นการติดตั้งไฟล์ skill ให้ agent แยกจากการติดตั้ง CLI: `--agent` ต้องเป็น `codex` หรือ `claude`, `--scope` ต้องเป็น `project` หรือ `user` `project` ใช้ Git root และส่ง `--repo` ได้; `user` ใช้ home ของผู้ใช้และห้ามส่ง `--repo` แบบ explicit `--dry-run` ไม่เขียนไฟล์ `--force` แทนที่ได้เฉพาะ bundle ของ Pitstop ที่ manifest ยืนยัน ownership และความครบถ้วน ถ้ามี bundle อื่นหรือไฟล์ผู้ใช้ปะปนอยู่ คำสั่งจะ block เพื่อรักษาไฟล์เดิม

`completion bash`, `zsh`, `fish` และ `powershell` สร้าง script สำหรับ shell ตามชื่อ โดย `--no-descriptions` ปิดคำอธิบาย completion ได้ PowerShell เป็นเพียง generated completion script; การมี subcommand นี้ไม่ได้ยืนยันการรองรับ Pitstop แบบ native Windows เพราะ CLI ยังใช้ Unix file locking และการตรวจสอบปัจจุบันยังไม่ครอบคลุม Windows

`--job` ใช้แบบซ้ำหรือคั่นด้วย comma ได้ แต่ IDs ต้องไม่ซ้ำและอยู่ใน config selection `--arch` override สำหรับ run นี้ ไม่แก้ไฟล์ config `--timeout` ต้องมากกว่า 0 และนับต่อ check ตั้งแต่ create/copy/start/run ค่าเริ่มต้น 20 นาที เวลาที่ใช้เตรียม image และ cleanup อยู่นอก budget นี้

Ctrl+C หรือ SIGTERM ยกเลิก run การ stop/remove มีเวลาจำกัดแยกจาก timeout จึงอาจใช้เวลาเพิ่มก่อนคำสั่งจบ หาก daemon ไม่ตอบให้ดู `CLEANUP_INCOMPLETE` ใน warnings

`doctor` ไม่ pull image และไม่รัน commands; image ที่ยังไม่มีอาจขึ้น `ready` พร้อมข้อความว่าจะ pull ตอน run `ready` ไม่รับประกัน network, dependencies หรือผล tests

### Docker endpoint

CLI เลือก endpoint ตามลำดับ `DOCKER_CONTEXT` → `DOCKER_HOST` → active Docker context ตัวแปรเหล่านี้ใช้เชื่อมต่อ daemon บน host ไม่ใช่ environment variables ของ steps ตรวจค่าด้วย:

```sh
docker context show
docker version
```

## อ่าน report และ log

ในโหมดปกติ progress และผลที่อ่านได้อยู่บน stdout ในโหมด `--json` stdout เป็น JSON envelope เดียว ส่วน progress ไป stderr จึงแยกไฟล์ได้ตามตัวอย่าง

Envelope `schemaVersion` เป็น **1** ส่วน config เป็น **2** เป็นคนละ schema ตรวจ `exitCode`, `result.jobs`, coverage และ warnings ด้วยกัน

| Exit code | ความหมายและลำดับความสำคัญ |
| --- | --- |
| `130` | ยกเลิก มีความสำคัญสูงสุด |
| `1` | มี job `failed` รวม timeout/runtime failure; ยังคงเป็น 1 แม้มี unknown หรือ storage/cleanup error เพิ่ม |
| `2` | ไม่มี failed job แต่ config/environment ผิด, outcome unknown หรือผล/log/report/cleanup ไม่สมบูรณ์ |
| `0` | ทุก job ที่เลือกผ่านและไม่มีปัญหาด้านผลลัพธ์ |

| Field ใน run result | อ่านอย่างไร |
| --- | --- |
| `jobs[].status` | `passed`, `failed`, `skipped`; ดู `reasonCode` และ steps ประกอบ |
| `jobs[].engineExitCode` | container exit code เมื่อมีหลักฐาน; อาจเป็น `null` |
| `jobs[].steps` | ผลแต่ละ step และ `evidenceRef`; step duration ยังเป็น `null` ในรุ่นนี้ |
| `selectedCoverage` | `complete` เมื่อมีหลักฐานครบของ check ปกติ รวม check ที่ทดสอบแล้วล้มเหลว; `partial` เมื่อ interrupted/runtime/unknown |
| `workflowCoverage` | `not_applicable` เพราะไม่ได้ประเมิน workflow YAML |
| `warnings` | ปัญหาที่เกิดเพิ่ม เช่น log/cleanup; อย่าดูเฉพาะ job status |
| `source` | HEAD, dirty state, snapshot hash และ excluded paths |
| `error` ใน envelope | ข้อผิดพลาด เช่น config หรือบันทึก report ไม่ได้; อาจมีอยู่ร่วมกับ run result |
| `engine` | image, platform, resolved image ID/digest และ Docker version |
| `reportPath` | ตำแหน่ง report บนเครื่องที่รัน Pitstop |

`complete` ไม่ได้หมายความว่า tests ผ่าน และ `skipped` อาจหมายถึงผลไม่ทราบแน่ชัด ไม่ใช่แค่ผู้ใช้ไม่ได้เลือก step

Reports อยู่ใต้ `<user-cache>/pitstop/runs/<runId>/` เช่น macOS ใช้ `~/Library/Caches/pitstop/runs/`; Linux โดยปริยายใช้ `~/.cache/pitstop/runs/` หรือ cache directory ที่ระบบกำหนด ใช้ `reportPath` ที่ CLI ส่งกลับเป็นหลัก

| ไฟล์ | เนื้อหา |
| --- | --- |
| `report.json` | JSON envelope ของ run |
| `<check-id>.stdout`, `<check-id>.stderr` | raw output ของ check |
| `evidence-<check-id>/` | exit records และ completion marker |
| `config.json`, `effective-config.json` | config ที่ capture และ config หลังใช้ overrides |
| `manifest.txt`, `metadata.json` | source identity, hashes และ command arguments |
| `argv-*.json`, `engine-meta.json`, `owned-resources.json` | arguments และข้อมูลตรวจย้อนหลังเรื่อง container ownership |

ถ้าล้มเหลวก่อนเริ่ม checks อาจไม่มี `result`/`reportPath` ให้ดู `error.code` และ `error.message` แทน ไม่มี automatic report retention policy ในรุ่นนี้

## ไฟล์ใดอยู่ใน snapshot

- ไฟล์ tracked ใช้เนื้อหาปัจจุบัน รวมการแก้และการลบ
- ไฟล์ใหม่ที่ Git ไม่ ignore รวมอยู่ด้วย
- ไฟล์ untracked ที่ ignore หรือชื่อกลุ่ม `.env`, `.env.*`, `.secrets`, `.vars`, `.input` ไม่ถูกรวม
- ไฟล์ที่ tracked แล้วจะถูกรวม แม้ชื่ออยู่ในกลุ่มข้างต้น; การ exclude ไม่ใช่ secret scanner

จึงต้องติดตั้ง dependencies ที่ถูก ignore ภายใน container ไม่อาศัย `node_modules` บน host รุ่นนี้ไม่รองรับ submodules, unresolved merge conflicts, absolute symlinks, broken symlinks หรือ symlinks ที่ออกนอก repo

## ใช้ shared script ร่วมกับ GitHub CI

วาง logic การตรวจไว้ใน project script แล้วเรียกจากทั้ง Pitstop และ CI ตัวอย่าง CI หลัง checkout และเตรียม Node แล้ว:

```yaml
- name: Install dependencies
  run: npm ci
- name: Check project
  run: sh scripts/check.sh
```

Pitstop ใช้ image เพื่อเตรียม toolchain และ snapshot เพื่อเตรียม source ส่วน CI เตรียม environment ของตัวเอง การใช้ script เดียวลดคำสั่งที่ซ้ำกัน แต่ไม่ได้รับประกันว่า OS/tool versions/network ของสอง environment เหมือนกัน

## ย้าย config เดิม

Schema version 1 ใช้ไม่ได้ ให้สร้าง config v2 ใหม่จากตัวอย่าง ไม่ใช่แก้เลข version อย่างเดียว:

| ส่วนเดิม | ส่วนใหม่ |
| --- | --- |
| `workflow`, `event`, `adapter`, `runner.label` | ลบออก |
| checkout step | Pitstop ทำ snapshot ให้อัตโนมัติ |
| setup toolchain step | เลือก image ที่มี tools นั้น |
| shell commands ใน workflow | ย้ายลง `checks[].steps[].run` หรือ shared script |
| job selection | ตั้ง `jobs` ให้ตรงกับ check IDs ใหม่ |

## แก้ปัญหา

| อาการ | ตรวจและแก้อย่างไร |
| --- | --- |
| `CONFIG_INVALID` / ไม่มี config | สร้าง `.pitstop.json` ที่ Git root, ใช้ version 2 และดู field/ID rules ด้านบน |
| `TOOL_MISSING` | ให้ Git/Docker CLI อยู่ใน PATH ของ process ที่เรียก Pitstop |
| `unknown flag: --platform` | ใช้ Docker CLI/daemon ที่รองรับ image inspect แบบระบุ platform |
| `DOCKER_UNAVAILABLE` | เปิด daemon, ตรวจ context/host, สิทธิ์เข้าถึง endpoint, registry login และ image/platform |
| doctor ready แต่ run pull ไม่ได้ | doctor ไม่ pull; ตรวจ network, credentials และชื่อ/tag ของ image |
| command not found / step failed | เพิ่ม tool ลง image หรือแก้ command; เปิด `<check-id>.stderr`/stdout |
| `RUN_ALREADY_ACTIVE` | รอ run เดิมใน repo เดียวกันจบ; repo คนละแห่งรันแยกได้ |
| `SNAPSHOT_CHANGED` | หยุด process ที่กำลังเขียนไฟล์/index ชั่วคราว, แก้ merge conflicts/submodules/symlinks แล้วลองใหม่ |
| `TIMEOUT` | ดู step ที่ยังไม่จบ; แก้คำสั่งหรือเพิ่ม `--timeout` ตามงาน |
| `ENGINE_OUTCOME_UNKNOWN` | หลักฐานไม่ครบ; ตรวจ Docker/log/evidence แล้วลองใหม่ ไม่ถือว่า passed |
| `CONTAINER_RUNTIME_FAILED` | ดู OOM/runtime reason และทรัพยากรของ Docker |
| `LOG_CAPTURE_FAILED` / `REPORT_WRITE_FAILED` | ตรวจพื้นที่/สิทธิ์ของ output และ user cache; failed check เดิมยังสำคัญกว่าปัญหาบันทึกผล |
| `CLEANUP_INCOMPLETE` | ดู warnings และ ownership metadata; ตรวจ container ด้วย exact run label ก่อนจัดการเอง |

ตัวอย่างตรวจ container ที่เกี่ยวกับ run เดียว (แทน `RUN_ID` ด้วย `result.runId`):

```sh
docker ps -a --filter "label=com.pitstop.run=RUN_ID"
```

## ขอบเขตที่รองรับ

รองรับ Docker image ที่เตรียมไว้, shell/shared scripts, ordered steps, selection, snapshot, timeout/cancellation และ reports ไม่มี workflow YAML executor, การแปล `uses:`/expressions, matrix, dependencies ระหว่าง jobs (`needs`), service containers, secret injection, cache volumes, auto-build image, artifact export, self-hosted runner registration, automatic workflow conversion หรือ automatic manifest detection

ดู [ข้อกำหนด](requirements.md) และ [หลักฐานทดสอบ](evidence/docker-checks-validation.md) เอกสารออกแบบ/หลักฐานก่อนหน้าอยู่ในหมวดประวัติของ [สารบัญ](README.md)
