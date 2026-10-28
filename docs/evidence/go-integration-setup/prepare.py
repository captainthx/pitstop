"""Prepare isolated, generic Go setup trials; never uses a private project."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

source = Path(__file__).resolve().parents[3]
root = Path(tempfile.mkdtemp(prefix="pitstop-go-setup-"))
shutil.copytree(source / "internal/agentassets/bundle", root / "bundle-before")
archive = source / "internal/agentassets/testdata/bundle-v2"
if archive.exists():
    shutil.copytree(archive, root / "bundle-before", dirs_exist_ok=True)
baseline_manifest = json.loads((root / "bundle-before/manifest.json").read_text())
assert baseline_manifest["bundleVersion"] == "2"
expected = {entry["path"] for entry in baseline_manifest["files"]} | {"manifest.json"}
for path in (root / "bundle-before").rglob("*"):
    if path.is_file() and path.relative_to(root / "bundle-before").as_posix() not in expected:
        path.unlink()
for entry in baseline_manifest["files"]:
    assert hashlib.sha256((root / "bundle-before" / entry["path"]).read_bytes()).hexdigest() == entry["sha256"], entry["path"]
fixture = source / "scripts/testdata/testcontainers"
repo = root / "existing-unit-only"
shutil.copytree(fixture, repo)
(repo / "unit_test.go").write_text('package integration_test\nimport "testing"\nfunc TestUnit(t *testing.T) { if 2+2 != 4 { t.Fatal("addition") } }\n')
(repo / "external_test.go").write_text('''//go:build integration

package integration_test

import (
    "os"
    "testing"
)

func TestExternalService(t *testing.T) {
    if os.Getenv("EXTERNAL_SERVICE_URL") == "" {
        t.Skip("external service URL is not configured")
    }
    t.Fatal("external service is outside this disposable trial")
}
''')
(repo / "tools").mkdir()
(repo / "tools/go.mod").write_text("module example.com/pitstop-tools\n\ngo 1.26.5\n")
(repo / "tools/tool_test.go").write_text('package tools\nimport "testing"\nfunc TestTool(t *testing.T) { t.Log("nested module checked") }\n')
(repo / "Makefile").write_text('''test:
	go test -race -json ./...
test-integration:
	go test -race -tags integration -json ./...
test-tools:
	go -C tools test -race -json ./...
''')
(repo / ".github/workflows").mkdir(parents=True)
(repo / ".github/workflows/test.yml").write_text('''name: test
on: [push]
jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - run: go test -race -json ./...
  integration:
    runs-on: ubuntu-latest
    steps:
      - run: go test -race -tags integration -json ./...
  tools:
    runs-on: ubuntu-latest
    steps:
      - run: go -C tools test -race -json ./...
''')
arch = subprocess.check_output(["docker", "version", "--format", "{{.Server.Arch}}"], text=True).strip()
arch = {"aarch64": "arm64", "x86_64": "amd64"}.get(arch, arch)
config = {"schemaVersion": 2, "runner": {"image": "golang:1.26.5", "architecture": "linux/" + arch},
          "jobs": ["unit"], "checks": [{"id": "unit", "steps": [{"id": "test", "run": "go test -race -json ./..."}]}]}
(repo / ".pitstop.json").write_text(json.dumps(config, indent=2) + "\n")
for case in ["baseline", "authorized", "ordinary-request", "absent-config"]:
    target = root / case
    shutil.copytree(repo, target)
    if case == "absent-config":
        (target / ".pitstop.json").unlink()
    subprocess.run(["git", "-C", str(target), "init", "-q"], check=True)
    subprocess.run(["git", "-C", str(target), "add", "."], check=True)
    subprocess.run(["git", "-C", str(target), "-c", "user.name=Trial", "-c", "user.email=trial@localhost", "commit", "-qm", "generic Go fixture"], check=True)
print(root)
