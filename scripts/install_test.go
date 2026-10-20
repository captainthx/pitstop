package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, name, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseInstaller(t *testing.T) {
	binary := "#!/bin/sh\nprintf 'pitstop 1.2.3\\n'\n"
	tests := []struct {
		name, host, arch, manifest, download, binary, latest string
		args                                                 []string
		existing, symlink, directory, wantFail               bool
		want                                                 string
	}{
		{name: "linux amd64", want: "Installed Pitstop v1.2.3"},
		{name: "linux arm64", arch: "aarch64", want: "Installed"},
		{name: "mac arm64", host: "Darwin", arch: "arm64", want: "Installed"},
		{name: "mac amd64", host: "Darwin", want: "Installed"},
		{name: "latest pinned", latest: "v1.2.3", want: "Installed"},
		{name: "existing refused", existing: true, wantFail: true, want: "--force"},
		{name: "upgrade", existing: true, args: []string{"--force"}, want: "Installed"},
		{name: "wrong checksum preserves old", manifest: "wrong", existing: true, args: []string{"--force"}, wantFail: true, want: "checksum"},
		{name: "missing checksum", manifest: "missing", wantFail: true, want: "checksum"},
		{name: "duplicate checksum", manifest: "duplicate", wantFail: true, want: "checksum"},
		{name: "partial binary preserves old", download: "partial", existing: true, args: []string{"--force"}, wantFail: true, want: "download"},
		{name: "rename failure preserves old", download: "rename", existing: true, args: []string{"--force"}, wantFail: true, want: "replace"},
		{name: "nonexecutable download directory", download: "noexec", want: "Installed"},
		{name: "missing manifest", download: "manifest", wantFail: true, want: "download"},
		{name: "version mismatch preserves old", binary: "#!/bin/sh\necho pitstop 9.9.9\n", existing: true, args: []string{"--force"}, wantFail: true, want: "version"},
		{name: "unsupported OS", host: "FreeBSD", wantFail: true, want: "Unsupported"},
		{name: "unsupported arch", arch: "riscv64", wantFail: true, want: "Unsupported"},
		{name: "invalid version", args: []string{"--version", "v1.2.3/evil"}, wantFail: true, want: "version"},
		{name: "metacharacter version", args: []string{"--version", "v1.2.3; touch injected"}, wantFail: true, want: "version"},
		{name: "multiline version", args: []string{"--version", "v1.2.3\nevil"}, wantFail: true, want: "version"},
		{name: "unknown flag", args: []string{"--wat"}, wantFail: true, want: "Unknown"},
		{name: "missing flag value", args: []string{"--version"}, wantFail: true, want: "requires"},
		{name: "bad latest redirect", latest: "../evil", wantFail: true, want: "release"},
		{name: "symlink refused", symlink: true, args: []string{"--force"}, wantFail: true, want: "regular"},
		{name: "directory refused", directory: true, args: []string{"--force"}, wantFail: true, want: "regular"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fake := filepath.Join(root, "tools")
			assets := filepath.Join(root, "assets")
			bin := filepath.Join(root, "bin with spaces ; literal")
			for _, dir := range []string{fake, assets, bin} {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			host, arch := tc.host, tc.arch
			if host == "" {
				host = "Linux"
			}
			if arch == "" {
				arch = "x86_64"
			}
			osName, cpu := "linux", "amd64"
			if host == "Darwin" {
				osName = "darwin"
			}
			if arch == "arm64" || arch == "aarch64" {
				cpu = "arm64"
			}
			asset := "pitstop_" + osName + "_" + cpu
			content := binary
			if tc.binary != "" {
				content = tc.binary
			}
			writeFixture(t, filepath.Join(assets, asset), content, 0o600)
			manifest := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(content)), asset)
			switch tc.manifest {
			case "wrong":
				manifest = strings.Repeat("0", 64) + "  " + asset + "\n"
			case "missing":
				manifest = strings.ReplaceAll(manifest, asset, "unrelated")
			case "duplicate":
				manifest += manifest
			}
			writeFixture(t, filepath.Join(assets, "checksums.txt"), manifest, 0o600)
			writeFixture(t, filepath.Join(fake, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo \"$TEST_HOST\";; -m) echo \"$TEST_ARCH\";; *) exit 1;; esac\n", 0o700)
			writeFixture(t, filepath.Join(fake, "curl"), `#!/bin/sh
[ "$1" = --disable ] || exit 94
out=
secure=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) out=$2; shift 2;;
    --proto) [ "$2" = '=https' ] || exit 90; secure=1; shift 2;;
    --proto-redir) [ "$2" = '=https' ] || exit 91; shift 2;;
    --write-out|--connect-timeout|--max-time|--retry) shift 2;;
    --*) shift;;
    *) url=$1; shift;;
  esac
done
[ "$secure" = 1 ] || exit 92
printf '%s\n' "$url" >> "$TEST_REQUESTS"
case "$url" in
  https://github.com/captainthx/pitstop/releases/latest)
    printf 'https://github.com/captainthx/pitstop/releases/tag/%s' "$TEST_LATEST"; exit 0;;
  https://github.com/captainthx/pitstop/releases/download/v1.2.3/*) ;;
  *) exit 93;;
esac
name=${url##*/}
if [ "$TEST_DOWNLOAD" = manifest ] && [ "$name" = checksums.txt ]; then exit 22; fi
if [ "$TEST_DOWNLOAD" = partial ] && [ "$name" != checksums.txt ]; then printf partial > "$out"; exit 18; fi
cp "$TEST_ASSETS/$name" "$out"
`, 0o700)
			if tc.download == "rename" {
				writeFixture(t, filepath.Join(fake, "mv"), "#!/bin/sh\nexit 1\n", 0o700)
			}
			realChmod, err := exec.LookPath("chmod")
			if err != nil {
				t.Fatal(err)
			}
			if tc.download == "noexec" {
				// Keep the download non-executable, modeling the noexec failure boundary.
				writeFixture(t, filepath.Join(fake, "chmod"), "#!/bin/sh\ncase \"$2\" in */pitstop-install.*/binary) exit 0;; esac\nexec \"$TEST_REAL_CHMOD\" \"$@\"\n", 0o700)
			}
			target := filepath.Join(bin, "pitstop")
			old := "old executable bytes\n"
			if tc.existing {
				writeFixture(t, target, old, 0o700)
			}
			if tc.directory {
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.symlink {
				writeFixture(t, filepath.Join(root, "victim"), old, 0o600)
				if err := os.Symlink(filepath.Join(root, "victim"), target); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"install.sh", "--bin-dir", bin}
			if tc.latest == "" {
				args = append(args, "--version", "v1.2.3")
			}
			args = append(args, tc.args...)
			cmd := exec.Command("sh", args...)
			requests := filepath.Join(root, "requests")
			cmd.Env = append(os.Environ(), "PATH="+fake+":"+os.Getenv("PATH"), "HOME="+root, "TMPDIR="+root, "TEST_HOST="+host, "TEST_ARCH="+arch, "TEST_ASSETS="+assets, "TEST_REQUESTS="+requests, "TEST_DOWNLOAD="+tc.download, "TEST_LATEST="+tc.latest, "TEST_REAL_CHMOD="+realChmod)
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantFail || !strings.Contains(string(out), tc.want) {
				t.Fatalf("err=%v output=%s; want failure=%v containing %q", err, out, tc.wantFail, tc.want)
			}
			if !tc.wantFail {
				got, err := os.ReadFile(target)
				if err != nil || string(got) != content {
					t.Fatalf("installed bytes: %q %v", got, err)
				}
				info, _ := os.Stat(target)
				if info.Mode().Perm() != 0o755 {
					t.Fatalf("mode=%v", info.Mode())
				}
				urls, _ := os.ReadFile(requests)
				if strings.Contains(string(urls), "/latest/download/") || !strings.Contains(string(urls), "/v1.2.3/"+asset) {
					t.Fatalf("release not pinned: %s", urls)
				}
			} else if tc.existing || tc.symlink {
				got, err := os.ReadFile(target)
				if err != nil || string(got) != old {
					t.Fatalf("old bytes changed: %q %v", got, err)
				}
			}
			entries, _ := os.ReadDir(bin)
			for _, entry := range entries {
				if entry.Name() != "pitstop" {
					t.Errorf("staging file left: %s", entry.Name())
				}
			}
			entries, _ = os.ReadDir(root)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "pitstop-install.") {
					t.Errorf("download directory left: %s", entry.Name())
				}
			}
		})
	}
}
