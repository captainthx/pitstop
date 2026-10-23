# ติดตั้งและออก release / Installation and releases

[ไทย README](../README.th.md) · [English README](../README.md) · [Docs](README.md)

**อัปเดต 2026-10-23 / Updated:** ติดตั้ง stable release จาก [GitHub Releases](https://github.com/captainthx/pitstop/releases) ด้วยคำสั่งด้านล่าง หรือ [build จาก source](../README.md#build-from-source) / Install a published stable GitHub Release using the commands below, or build from source.

## ติดตั้งจากลิงก์ / Install from a link

รองรับ macOS และ Linux บน arm64/amd64 ต้องมีเครื่องมือ SHA-256 (`sha256sum` หรือ `shasum`) และไม่ต้องมี Go สำหรับ binary สำเร็จรูป การรัน checks ยังต้องมี Git กับ Docker / Supports macOS/Linux arm64/amd64, requires a SHA-256 utility, and does not require Go for prebuilt binaries. Checks still require Git and Docker.

**Repository นี้เป็น private / This repository is private.** ใช้ GitHub CLI ที่ล็อกอินด้วยบัญชีที่เข้าถึง repo ได้ การออก release ไม่เปลี่ยน visibility / Use GitHub CLI authenticated with repository access. Publishing a release does not change repository visibility.

1. ล็อกอิน GitHub CLI และดาวน์โหลด installer ถ้า download ล้มเหลวให้หยุด / Authenticate GitHub CLI and download the installer; stop on failure:

```sh
gh auth login --hostname github.com
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
gh release download --repo captainthx/pitstop --pattern install.sh \
  --output "$PITSTOP_INSTALLER" --clobber
```

2. อ่าน script ก่อนรัน / Review before executing:

```sh
cat "$PITSTOP_INSTALLER"
```

3. ติดตั้งผ่าน session ของ gh แล้วตรวจเวอร์ชัน / Install using the gh session and check:

```sh
sh "$PITSTOP_INSTALLER" --github-auth
export PATH="$HOME/.local/bin:$PATH"
pitstop version
```

ไม่ต้องคัดลอก token มาใส่ flag ตัวติดตั้งใช้ `gh` กับ host/repo ที่กำหนดตายตัว และปิด interactive prompt ของคำสั่งดาวน์โหลด / Do not pass a token as an installer flag. The installer uses gh against the fixed GitHub host/repository with interactive prompts disabled. Authenticated network handling follows gh defaults; the explicit curl timeouts below apply only to the public route.

### Public repository HTTPS route

สำหรับ public repository เท่านั้น ใช้ curl แทน gh ได้ / For a public repository, curl can replace gh:

```sh
PITSTOP_INSTALLER="$(mktemp "${TMPDIR:-/tmp}/pitstop-installer.XXXXXXXX")"
curl --disable --fail --show-error --location \
  --proto '=https' --proto-redir '=https' \
  --connect-timeout 15 --max-time 180 --output "$PITSTOP_INSTALLER" \
  https://github.com/captainthx/pitstop/releases/latest/download/install.sh
```

อ่าน script แล้วรัน `sh "$PITSTOP_INSTALLER"` โดยไม่ต้องใส่ `--github-auth` / Review it, then run without --github-auth.

Installer จะเลือกแพลตฟอร์มและ resolve latest เพียงครั้งเดียว จากนั้นดาวน์โหลด binary และ checksum จาก tag เดียวกัน ตรวจ SHA-256 และเวอร์ชัน ก่อนย้าย binary เข้า `~/.local/bin/pitstop` ไม่เรียก sudo และไม่แก้ shell profile / The installer resolves latest once, fetches assets from that pinned tag, checks SHA-256 and executable version, then installs to `~/.local/bin/pitstop`. It never invokes sudo or edits shell profiles.

เพิ่ม `export PATH="$HOME/.local/bin:$PATH"` ลง `~/.zshrc` สำหรับ zsh หรือ `~/.bashrc` สำหรับ interactive bash ด้วยตัวเอง แล้วเปิด terminal ใหม่ หากมี Pitstop หลายชุด ใช้ `command -v pitstop` ตรวจตัวที่ shell เลือก / Add that export to your shell configuration yourself for future terminals. Use `command -v pitstop` to find the executable selected by PATH.

## ระบุเวอร์ชันและตำแหน่ง / Version and directory

ดู tag ที่เผยแพร่จริงใน [Releases](https://github.com/captainthx/pitstop/releases) ตัวอย่างด้านล่าง pin เป็น `v0.1.0` เปลี่ยนเป็น tag ที่ต้องการได้ / The commands below pin v0.1.0; choose another published stable tag when needed:

```sh
sh "$PITSTOP_INSTALLER" --github-auth --version v0.1.0
sh "$PITSTOP_INSTALLER" --github-auth --version v0.1.0 --bin-dir "$HOME/tools/bin"
sh "$PITSTOP_INSTALLER" --help
```

รองรับ stable tag รูปแบบ `vMAJOR.MINOR.PATCH` เท่านั้น หากใช้ gh ให้ระบุ tag ในคำสั่ง `gh release download v0.1.0 ...` เพื่อ pin ตัว installer ด้วย หากใช้ public HTTPS ให้เปลี่ยน URL ดาวน์โหลดจาก `/releases/latest/download/install.sh` เป็น `/releases/download/v0.1.0/install.sh` ตาม tag ที่เลือก / Only stable vMAJOR.MINOR.PATCH tags are accepted. Pin the installer itself with `gh release download v0.1.0 ...`, or use the matching versioned public URL. For a public curl installation, omit --github-auth in the commands below.

## อัปเกรด / Upgrade

ดาวน์โหลดและอ่าน installer ใหม่ตามขั้นตอนด้านบน แล้วใช้ `--force` เพื่ออนุญาตแทนที่ไฟล์เดิมอย่างชัดเจน / Download and review the installer again, then explicitly allow replacement:

```sh
sh "$PITSTOP_INSTALLER" --github-auth --force
pitstop version
```

หากใช้ `--bin-dir` ต้องส่งตำแหน่งเดิมทุกครั้ง ตัวติดตั้งไม่เขียนทับ symlink หรือ directory แม้ระบุ force และไม่เปลี่ยน config หรือ skill ที่เคยติดตั้ง / Repeat a custom --bin-dir on upgrades. Symlinks and non-regular destinations are refused even with force. Project configs and installed skills are not changed by CLI upgrades.

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

รัน workflow ซ้ำบน tag ที่มี release แล้วจะล้มเหลวแทนการเขียนทับ assets เดิม Homebrew, Windows, signed releases, notarization, self-update และการทดสอบ runtime ทุกสถาปัตยกรรมยังไม่รวม / Rerunning a tag with an existing release fails rather than overwriting assets. Homebrew, Windows, signing, notarization, self-update and complete native coverage are deferred.
