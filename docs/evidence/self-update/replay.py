#!/usr/bin/env python3
"""Exercise the updater with real compiled CLI/assets and controlled release bytes."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


def run(argv, **kwargs):
    return subprocess.run(["rtk", "proxy", *map(str, argv)], check=True, **kwargs)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    source = args.source.resolve(strict=True)
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=False)
    new_source = root / "hypothetical-release-source"
    new_source.mkdir()
    for name in ("go.mod", "go.sum", "installer_assets.go"):
        shutil.copy2(source / name, new_source / name)
    for name in ("cmd", "internal"):
        shutil.copytree(source / name, new_source / name)
    (new_source / "scripts").mkdir()
    shutil.copy2(source / "scripts/install.sh", new_source / "scripts/install.sh")
    bundle = new_source / "internal/agentassets/bundle"
    marker = "\nUpdater acceptance: hypothetical NEW embedded skill bytes.\n"
    skill = bundle / "SKILL.md"
    skill.write_bytes(skill.read_bytes() + marker.encode())
    manifest = json.loads((bundle / "manifest.json").read_text())
    for item in manifest["files"]:
        item["sha256"] = digest((bundle / item["path"]).read_bytes())
    (bundle / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")

    binary_dir = root / "custom bin with spaces"
    binary_dir.mkdir()
    current = binary_dir / "pitstop"
    release = root / "hypothetical-new-pitstop"
    build = ["env", "GOTOOLCHAIN=go1.26.5", "go", "build", "-o"]
    run([*build, current, "./cmd/pitstop"], cwd=source)
    run([*build, release, "-ldflags", "-X pitstop/internal/cli.Version=1.2.3", "./cmd/pitstop"], cwd=new_source)
    original_binary_hash = digest(current.read_bytes())
    home = root / "home"
    home.mkdir()
    repo = root / "project with spaces"
    repo.mkdir()
    run(["git", "init", "-q", repo])

    released_root = source / "internal/agentassets/testdata/released-v0.1.0"
    original_manifest = (released_root / "manifest.json").read_bytes()
    original_inventory = json.loads(original_manifest)["files"]
    targets = [home / ".agents/skills/pitstop-config", home / ".claude/skills/pitstop-config", repo / ".agents/skills/pitstop-config", repo / ".claude/skills/pitstop-config"]
    for target in targets:
        target.mkdir(parents=True)
        (target / "manifest.json").write_bytes(original_manifest)
        for item in original_inventory:
            fixture = released_root / item["path"]
            data = (fixture if fixture.exists() else source / "internal/agentassets/bundle" / item["path"]).read_bytes()
            assert digest(data) == item["sha256"], item["path"]
            path = target / item["path"]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    fake = root / "controlled-tools"
    fake.mkdir()
    # The real embedded installer verifies these controlled bytes/checksums and executable version.
    curl = fake / "curl"
    curl.write_text('''#!/bin/sh
set -eu
[ "$1" = --disable ] || exit 90
out=
url=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) out=$2; shift 2;;
    --proto|--proto-redir|--connect-timeout|--max-time) shift 2;;
    --*) shift;;
    *) url=$1; shift;;
  esac
done
case "$url" in
  https://github.com/captainthx/pitstop/releases/download/v1.2.3/checksums.txt) cp "$PITSTOP_UPDATE_FIXTURE/checksums.txt" "$out";;
  https://github.com/captainthx/pitstop/releases/download/v1.2.3/pitstop_*) cp "$PITSTOP_UPDATE_FIXTURE/hypothetical-new-pitstop" "$out";;
  *) exit 91;;
esac
''')
    curl.chmod(0o755)
    host = subprocess.check_output(["rtk", "proxy", "env", "GOTOOLCHAIN=go1.26.5", "go", "env", "GOOS", "GOARCH"], cwd=source, text=True).splitlines()
    (root / "checksums.txt").write_text(digest(release.read_bytes()) + "  pitstop_" + "_".join(host) + "\n")
    env = dict(os.environ, HOME=str(home), PATH=str(fake) + os.pathsep + os.environ["PATH"], PITSTOP_UPDATE_FIXTURE=str(root))
    argv = [str(current), "update", "--version", "v1.2.3", "--repo", str(repo), "--json"]
    p = subprocess.run(["rtk", "proxy", *argv], cwd=root, env=env, capture_output=True, timeout=60)
    (root / "update.json").write_bytes(p.stdout)
    (root / "progress.log").write_bytes(p.stderr)
    (root / "command.json").write_text(json.dumps(argv, indent=2) + "\n")
    assert p.returncode == 0, (p.stdout, p.stderr)
    result = json.loads(p.stdout)
    assert result["command"] == "update" and result["exitCode"] == 0
    assert result["result"]["cliStatus"] == "updated" and not result["result"]["partial"]
    assert len(result["result"]["targets"]) == 4
    assert digest(current.read_bytes()) == digest(release.read_bytes()) != original_binary_hash
    for target in targets:
        assert (target / "SKILL.md").read_bytes() == skill.read_bytes()
        assert (target / "manifest.json").read_bytes() == (bundle / "manifest.json").read_bytes()
    assert not list(binary_dir.glob(".pitstop-update-*/")), "private staging directories remained"
    # Exercise automatic latest resolution without selecting a version. This only
    # reads public release metadata; the hypothetical 1.2.3 candidate is newer.
    inode_before = current.stat().st_ino
    automatic = subprocess.run(["rtk", "proxy", str(current), "update", "--json"], cwd=repo, env=env, capture_output=True, timeout=45)
    (root / "automatic.json").write_bytes(automatic.stdout)
    (root / "automatic.stderr").write_bytes(automatic.stderr)
    auto = json.loads(automatic.stdout)
    if automatic.returncode == 0:
        assert auto["result"]["cliStatus"] == "up_to_date"
        assert len(auto["result"]["targets"]) == 4
        automatic_status = "passed: real public latest metadata, newer binary preserved"
    else:
        assert auto.get("error", {}).get("code") == "UPDATE_RESOLVE_FAILED", auto
        automatic_status = "blocked: public metadata unavailable; controlled resolver tests required"
    assert current.stat().st_ino == inode_before
    # Real installer ownership checks must refuse modified and unknown files.
    for label in ("modified", "unknown"):
        if label == "modified":
            (targets[0] / "SKILL.md").write_text("fixture user edits; keep unchanged\n")
        else:
            (targets[0] / "SKILL.md").write_bytes(skill.read_bytes())
            (targets[0] / "user-notes.txt").write_text("fixture user data; keep unchanged\n")
        before = {str(p.relative_to(targets[0])): digest(p.read_bytes()) for p in targets[0].rglob("*") if p.is_file()}
        inode_before = current.stat().st_ino
        blocked = subprocess.run(["rtk", "proxy", *argv], cwd=root, env=env, capture_output=True, timeout=60)
        (root / (label + ".json")).write_bytes(blocked.stdout)
        (root / (label + ".stderr")).write_bytes(blocked.stderr)
        block = json.loads(blocked.stdout)
        assert blocked.returncode == block["exitCode"] == 2, block
        assert block["error"]["code"] == "UPDATE_PREFLIGHT_BLOCKED", block
        assert not block["result"]["partial"]
        assert current.stat().st_ino == inode_before
        after = {str(p.relative_to(targets[0])): digest(p.read_bytes()) for p in targets[0].rglob("*") if p.is_file()}
        assert after == before, label
    summary = {"case": "real compiled updater/new-bundle all-four scopes", "release": "hypothetical v1.2.3, not a public release", "download": "controlled curl fixture; real embedded installer/checksum/version", "previousBinarySha256": original_binary_hash, "newBinarySha256": digest(release.read_bytes()), "newManifestSha256": digest((bundle / "manifest.json").read_bytes()), "updatedTargets": 4, "cliExit": p.returncode, "markerInstalledFromNewProcess": True, "automaticLatest": automatic_status, "modifiedAndUnknownFilesPreserved": True}
    (root / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary), flush=True)


if __name__ == "__main__":
    main()
