package main

import (
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseRejectsInvalidInputs(t *testing.T) {
	for _, args := range [][]string{{}, {"v1.2.3/evil", "unused"}, {"v1.2.3\nevil", "unused"}, {"v01.2.3", "unused"}, {"v1.2.3", t.TempDir()}} {
		cmd := exec.Command("sh", append([]string{"release.sh"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "release:") {
			t.Fatalf("args=%q err=%v out=%s", args, err, out)
		}
	}
}

func TestReleasePackage(t *testing.T) {
	if os.Getenv("PITSTOP_TEST_RELEASE") != "1" {
		t.Skip("set PITSTOP_TEST_RELEASE=1 to cross-build four release binaries")
	}
	dist := filepath.Join(t.TempDir(), "release with spaces")
	cmd := exec.Command("sh", "release.sh", "v1.2.3", dist)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	manifest, err := os.ReadFile(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	wantNames := map[string]bool{"install.sh": true, "pitstop_darwin_arm64": true, "pitstop_darwin_amd64": true, "pitstop_linux_arm64": true, "pitstop_linux_amd64": true}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !wantNames[fields[1]] {
			t.Fatalf("unexpected/duplicate manifest entry: %s", line)
		}
		delete(wantNames, fields[1])
		data, err := os.ReadFile(filepath.Join(dist, fields[1]))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("checksum mismatch for %s: %v", fields[1], err)
		}
	}
	if len(wantNames) != 0 {
		t.Fatalf("missing assets: %v", wantNames)
	}
	for _, arch := range []string{"arm64", "amd64"} {
		m, err := macho.Open(filepath.Join(dist, "pitstop_darwin_"+arch))
		if err != nil {
			t.Fatal(err)
		}
		wantCPU := macho.CpuArm64
		if arch == "amd64" {
			wantCPU = macho.CpuAmd64
		}
		if m.Cpu != wantCPU {
			t.Fatalf("darwin %s CPU=%v", arch, m.Cpu)
		}
		m.Close()
		e, err := elf.Open(filepath.Join(dist, "pitstop_linux_"+arch))
		if err != nil {
			t.Fatal(err)
		}
		wantMachine := elf.EM_AARCH64
		if arch == "amd64" {
			wantMachine = elf.EM_X86_64
		}
		if e.Machine != wantMachine {
			t.Fatalf("linux %s machine=%v", arch, e.Machine)
		}
		e.Close()
	}
	native := filepath.Join(dist, "pitstop_"+runtime.GOOS+"_"+runtime.GOARCH)
	out, err := exec.Command(native, "version", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("native version: %v %s", err, out)
	}
	var envelope struct {
		Result struct{ Version, Commit string }
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil || envelope.Result.Version != "1.2.3" || envelope.Result.Commit != strings.TrimSpace(string(commit)) {
		t.Fatalf("metadata=%s giterr=%v", out, err)
	}
	entries, _ := os.ReadDir(dist)
	if len(entries) != 6 {
		t.Fatalf("unexpected release inventory: %v", entries)
	}

	// Install the real native package, not only the tiny shell fixture.
	fake := filepath.Join(t.TempDir(), "tools")
	if err := os.Mkdir(fake, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(fake, "curl"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) out=$2; shift 2;;
    --proto|--proto-redir|--connect-timeout|--max-time) shift 2;;
    --*) shift;;
    *) url=$1; shift;;
  esac
done
cp "$TEST_RELEASE_ASSETS/${url##*/}" "$out"
`, 0o700)
	installHome := t.TempDir()
	install := exec.Command("sh", filepath.Join(dist, "install.sh"), "--version", "v1.2.3")
	install.Env = append(os.Environ(), "HOME="+installHome, "PATH="+fake+":"+os.Getenv("PATH"), "TEST_RELEASE_ASSETS="+dist)
	if out, err := install.CombinedOutput(); err != nil || !strings.Contains(string(out), "Installed Pitstop v1.2.3") {
		t.Fatalf("real native install: %v %s", err, out)
	}
	installed, err := exec.Command(filepath.Join(installHome, ".local", "bin", "pitstop"), "version", "--json").CombinedOutput()
	if err != nil || string(installed) != string(out) {
		t.Fatalf("installed metadata differs: %v %s", err, installed)
	}
}
