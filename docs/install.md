# ติดตั้งและออก release / Installation and releases

[ไทย README](../README.th.md) · [English README](../README.md) · [Docs](README.md)

**อัปเดต 2026-10-23 / Updated:** ติดตั้ง stable release จาก [GitHub Releases](https://github.com/captainthx/pitstop/releases) ด้วยคำสั่งด้านล่าง หรือ [build จาก source](../README.md#build-from-source) / Install a published stable GitHub Release using the commands below, or build from source.

## ติดตั้งจากลิงก์ / Install from a link

รองรับ macOS และ Linux บน arm64/amd64 ต้องมี curl และเครื่องมือ SHA-256 (`sha256sum` หรือ `shasum`) และไม่ต้องมี Go สำหรับ binary สำเร็จรูป การรัน checks ยังต้องมี Git กับ Docker / Supports macOS/Linux arm64/amd64, requires curl and a SHA-256 utility, and does not require Go for prebuilt binaries. Checks still require Git and Docker.

**Repository เป็น public / This repository is public.** ดาวน์โหลด installer และ binary ผ่าน HTTPS ได้โดยไม่ต้องมีบัญชีหรือ GitHub CLI / Installer and binary downloads require neither an account nor GitHub CLI.

### Public repository HTTPS route

1. ดาวน์โหลดไปยังไฟล์ชั่วคราว ถ้า download ล้มเหลวให้หยุด / Download to a temporary file; stop on failure:

```sh
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location \
  --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
```

2. อ่าน script ก่อนรัน / Review before executing:

```sh
cat "$PITSTOP_INSTALLER"
```

3. ติดตั้งแล้วตรวจเวอร์ชัน / Install and check:

```sh
sh "$PITSTOP_INSTALLER"
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

### Authenticated GitHub CLI route

GitHub CLI ยังใช้เป็นทางเลือกได้ โดยล็อกอินก่อนดาวน์โหลด / GitHub CLI remains an optional authenticated alternative:

```sh
gh auth login --hostname github.com
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
gh release download --repo captainthx/pitstop --pattern install.sh \
  --output "$PITSTOP_INSTALLER" --clobber
cat "$PITSTOP_INSTALLER"
```

หลังอ่านแล้วใช้ `sh "$PITSTOP_INSTALLER" --github-auth` ตัวติดตั้งใช้ session ของ gh กับ host/repo ที่กำหนดตายตัว ไม่รับ token เป็น flag / After review, run with --github-auth. Credentials stay in gh; the installer fixes the host/repository and does not accept tokens as flags. Authenticated network handling follows gh defaults; explicit curl timeouts apply only to the HTTPS route.

Installer จะเลือกแพลตฟอร์มและ resolve latest เพียงครั้งเดียว จากนั้นดาวน์โหลด binary และ checksum จาก tag เดียวกัน ตรวจ SHA-256 และเวอร์ชัน ก่อนย้าย binary เข้า `~/.local/bin/pitstop` ไม่เรียก sudo และไม่แก้ shell profile / The installer resolves latest once, fetches assets from that pinned tag, checks SHA-256 and executable version, then installs to `~/.local/bin/pitstop`. It never invokes sudo or edits shell profiles.

เพิ่ม `export PATH="$HOME/.local/bin:$PATH"` ลง `~/.zshrc` สำหรับ zsh หรือ `~/.bashrc` สำหรับ interactive bash ด้วยตัวเอง แล้วเปิด terminal ใหม่ หากมี Pitstop หลายชุด ใช้ `command -v pitstop` ตรวจตัวที่ shell เลือก / Add that export to your shell configuration yourself for future terminals. Use `command -v pitstop` to find the executable selected by PATH.

## ระบุเวอร์ชันและตำแหน่ง / Version and directory

ดู tag ที่เผยแพร่จริงใน [Releases](https://github.com/captainthx/pitstop/releases) ตัวอย่างด้านล่าง pin เป็น `v0.1.0` เปลี่ยนเป็น tag ที่ต้องการได้ / The commands below pin v0.1.0; choose another published stable tag when needed:

```sh
sh "$PITSTOP_INSTALLER" --version v0.1.0
sh "$PITSTOP_INSTALLER" --version v0.1.0 --bin-dir "$HOME/tools/bin"
sh "$PITSTOP_INSTALLER" --help
```

รองรับ stable tag รูปแบบ `vMAJOR.MINOR.PATCH` เท่านั้น หากใช้ gh ให้ระบุ tag ในคำสั่ง `gh release download v0.1.0 ...` เพื่อ pin ตัว installer ด้วย หากใช้ public HTTPS ให้เปลี่ยน URL ดาวน์โหลดจาก `/releases/latest/download/install.sh` เป็น `/releases/download/v0.1.0/install.sh` ตาม tag ที่เลือก / Only stable vMAJOR.MINOR.PATCH tags are accepted. Pin the installer itself with `gh release download v0.1.0 ...`, or use the matching versioned public URL. Add --github-auth only when choosing the optional GitHub CLI route.

## อัปเกรด / Upgrade

### คำสั่งเดียว / One command

สำหรับ source build ปัจจุบัน หรือ binary release ที่มีคำสั่ง `update` ใช้เพียง / For a current source build or a binary release that includes `update`, run:

```sh
pitstop update
```

คำสั่งจะเช็ก stable ล่าสุดเอง ไม่ต้องระบุเวอร์ชัน อัปเดต executable ที่กำลังใช้งานในตำแหน่งจริง พร้อม Codex/Claude skills ที่ติดตั้งไว้แล้วของ user และ Git root ปัจจุบัน (หรือ `--repo` ที่เลือก) ไม่ติดตั้ง skill เพิ่มและไม่ค้น repo อื่น หาก CLI ล่าสุดแล้วจะคง binary และยัง refresh skills ต่อ / It resolves the latest stable release automatically, updates the running executable at its actual location, and refreshes existing Codex/Claude user and selected-project skills. It neither creates missing skills nor scans other repositories. An already-current CLI still refreshes skills.

```sh
pitstop update --repo /absolute/path/to/project --json
```

`--repo` ที่ไม่ระบุและ cwd นอก Git จะเลือกเฉพาะ user skills; ถ้าระบุ repo ผิดจะ fail ก่อนเปลี่ยนไฟล์ `--json` แยก JSON stdout จาก progress stderr / An implicit non-Git cwd selects user skills only; an explicit invalid repo fails before mutation. JSON output stays on stdout, progress on stderr.

ตรวจ binary checksum/version และให้ binary รุ่นใหม่ preflight skills ก่อนแทน CLI หาก skill ถูกแก้เองหรือไม่รู้จักจะคงไฟล์เดิมและ block การอัปเดต ถ้าเกิดข้อผิดพลาดหลังเปลี่ยน CLI แล้วจะรายงานว่าเปลี่ยนอะไรสำเร็จและอะไรยังเหลือ ไม่อ้าง rollback ทั้งชุด / Download/checksum/version and new-binary skill preflight failures preserve the current CLI and skills. Modified/unknown skill directories block the update. A late failure reports actual partial completion; CLI and multiple skill directories are not a single atomic transaction. Exit codes: `0` complete/already current, `2` blocked/failed/partial, `130` cancelled.

เลือก stable tag เองด้วย `pitstop update --version vMAJOR.MINOR.PATCH` เมื่อจงใจ pin/reinstall/downgrade เท่านั้น ไม่แก้ `.pitstop.json`, shell profiles หรือขอ sudo / Use an explicit stable `--version` only for intentional pin/reinstall/downgrade. Updating does not change project configs, shell profiles or use sudo.

### อัปเกรดครั้งแรกจาก v0.1.0 หรือทำด้วยตนเอง / First v0.1.0 upgrade or manual installation

Published v0.1.0 ยังไม่มี `pitstop update` ต้องใช้ installer ครั้งแรกเมื่อมี release ใหม่ที่รวมคำสั่งนี้ หรือ build จาก source ก่อน การเพิ่มคำสั่งใน source ไม่ได้เผยแพร่ release ใหม่ / Published v0.1.0 has no update command. Its first upgrade needs the existing installer after a new release is published, or a source build. Implementing this command does not publish that release.

ดาวน์โหลดและอ่าน installer ใหม่ตามขั้นตอนด้านบน แล้วใช้ `--force` เพื่ออนุญาตแทนที่ไฟล์เดิมอย่างชัดเจน / Download and review the installer again, then explicitly allow replacement:

```sh
sh "$PITSTOP_INSTALLER" --force
pitstop version
```

หากใช้ `--bin-dir` ต้องส่งตำแหน่งเดิมทุกครั้ง ตัวติดตั้งไม่เขียนทับ symlink หรือ directory แม้ระบุ force และ standalone installer ไม่เปลี่ยน config หรือ skill ที่เคยติดตั้ง / Repeat a custom --bin-dir for standalone-installer upgrades. Symlinks and non-regular destinations are refused even with force. The standalone installer leaves project configs and installed skills unchanged; `pitstop update` separately refreshes existing skills.

## ลบ CLI / Remove the CLI

สำหรับตำแหน่งเริ่มต้น ตรวจ path ก่อนแล้วลบเฉพาะ binary / For the default location, inspect the path and remove only the binary:

```sh
command -v pitstop
rm -i "$HOME/.local/bin/pitstop"
```

ไม่ลบรายงาน cache, `.pitstop.json` หรือ skill ของ agent โดยอัตโนมัติ / Reports, project configs and agent skills are retained.

## ขอบเขตความปลอดภัย / Trust and failure behavior

- HTTPS เท่านั้นและไม่อ่าน `.curlrc`; checksum ตรวจความตรงกันของ binary ก่อน execute ไม่ป้องกันผู้โจมตีที่แก้ทั้ง release และ checksum ได้ / HTTPS only, with curl configuration disabled. Checksums detect corruption; they do not authenticate a release against a compromised GitHub account.
- ใช้ directory ที่ผู้ใช้เป็นเจ้าของและควบคุมเอง การแข่งแก้ path โดยผู้ใช้อื่นใน shared directory อยู่นอกขอบเขต / Install only into a directory you own and control; hostile concurrent changes in shared directories are outside this boundary.
- ดาวน์โหลดหรือตรวจไม่ผ่านจะไม่แทนที่ binary เดิม staging อยู่บน filesystem เดียวกับปลายทางแล้ว rename; ไม่รับประกัน durability หากเครื่องดับ / Download, checksum, version or staging failures preserve the existing binary. Same-filesystem rename publishes the staged binary; power-loss durability is not guaranteed.
- Installer ไม่ดาวน์โหลด Go, GitHub CLI, Docker หรือ dependencies ของโปรเจกต์ / The installer does not install Go, GitHub CLI, Docker or project dependencies.
- Binary ของ macOS ยังไม่มี Developer ID signing/notarization หาก macOS บล็อก ให้ตรวจสอบแหล่งที่มาหรือ build จาก source อย่าปิดระบบป้องกันทั้งเครื่อง / macOS binaries are not Developer ID signed or notarized. If blocked, verify the source or build locally; do not disable system protections.
- การ cross-build ไม่ใช่หลักฐาน runtime บนทุก OS/architecture ดู [หลักฐาน](evidence/release-install.md) / Cross-compilation is not native runtime verification for every target; see the evidence.

## ขั้นตอน maintainer / Maintainer release procedure

1. ตรวจ source และผล CI บน commit ที่จะออก release เลือก tag ใหม่ที่ยังไม่เคยใช้ / Review the source and CI for the intended commit and choose an unused stable tag.
2. ทดสอบแพ็กเกจใน directory ใหม่ / Test packaging into a new directory:

```sh
PITSTOP_TEST_RELEASE=1 go test ./scripts -count=1
sh scripts/release.sh v0.1.0 /absolute/path/to/new-release-directory
```

3. เมื่อพร้อมออก release ให้ maintainer สร้างและ push tag จาก commit ที่ตรวจแล้วเอง / When release is authorized, the maintainer creates and pushes the tag from the reviewed commit. Implementation of this feature does not itself push or publish.
4. `.github/workflows/release.yml` ทดสอบบน macOS/Linux, ตรวจ source และสร้าง binary 4 ไฟล์, `install.sh`, `checksums.txt` ก่อนสร้าง **draft** GitHub Release สิทธิ์เขียน repo อยู่เฉพาะ job สร้าง draft และไม่ checkout source ใน job นั้น / The workflow tests and builds six assets, then creates a draft. Only the draft job receives contents:write; it does not check out or execute project source.
5. ตรวจ draft, checksums, tag/commit, native smoke และ release notes แล้วค่อย publish ผ่าน GitHub UI หรือ CLI ก่อนแจก installer link / Inspect the draft, checksums, tag/commit, native smoke and notes; publish through GitHub UI or CLI before distributing the install link.

รัน workflow ซ้ำบน tag ที่มี release แล้วจะล้มเหลวแทนการเขียนทับ assets เดิม Homebrew, Windows, signed releases, notarization และการทดสอบ runtime ทุกสถาปัตยกรรมยังไม่รวม / Rerunning a tag with an existing release fails rather than overwriting assets. Homebrew, Windows, signing, notarization and complete native coverage are deferred.
