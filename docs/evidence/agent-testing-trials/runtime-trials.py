#!/usr/bin/env python3
"""Replay harmless real-Docker failure cases; all state stays in --output."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import time


def command(argv, **kwargs):
    return subprocess.run(["rtk", "proxy", *argv], check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=False)
    endpoint = os.environ.get("DOCKER_HOST") or subprocess.check_output(
        ["rtk", "proxy", "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}"], text=True
    ).strip()
    platform = subprocess.check_output(
        ["rtk", "proxy", "docker", "version", "--format", "{{.Server.Os}}/{{.Server.Arch}}"], text=True
    ).strip()
    assert platform in ("linux/arm64", "linux/amd64"), platform
    cases = {
        "pass": ('test "$(cat answer.txt)" = 42\nprintf "fixture assertion passed\\n"\n', 0),
        "test-failure": ('test "$(cat answer.txt)" = 99 || { printf "fixture assertion failed: expected 99\\n" >&2; exit 7; }\n', 1),
        "missing-tool": ('command -v pitstop_fixture_tool_not_installed || { printf "missing fixture tool\\n" >&2; exit 127; }\n', 1),
        "docker-unavailable": ('printf "must not run\\n"\n', 2),
        "timeout": ('printf "fixture sleeping\\n"; sleep 30\n', 1),
        "cancellation": ('printf "fixture sleeping\\n"; sleep 30\n', 130),
    }
    summary = []
    for name, (script, expected) in cases.items():
        case = root / name
        repo = case / "repo"
        repo.mkdir(parents=True)
        home = case / "home"
        home.mkdir()
        env = dict(os.environ, HOME=str(home), XDG_CACHE_HOME=str(case / "cache"), DOCKER_HOST=endpoint)
        if name == "docker-unavailable":
            env.pop("DOCKER_CONTEXT", None)
            env["DOCKER_HOST"] = "unix://" + str(case / "nonexistent.sock")
        (repo / "answer.txt").write_text("42\n")
        (repo / "check.sh").write_text("#!/bin/sh\nset -eu\n" + script)
        cfg = {
            "schemaVersion": 2,
            "runner": {"image": "alpine:3.22", "architecture": platform},
            "jobs": ["quality_gate"],
            "checks": [{"id": "quality_gate", "steps": [{"id": "assert_answer", "run": "sh check.sh"}]}],
        }
        (repo / ".pitstop.json").write_text(json.dumps(cfg, indent=2) + "\n")
        for argv in (["init", "-q"], ["add", "."], ["-c", "user.name=Pitstop Trial", "-c", "user.email=trial@example.invalid", "commit", "-qm", "fixture"]):
            command(["git", "-C", str(repo), *argv], env=env)
        config_before = (repo / ".pitstop.json").read_bytes()
        for subcommand in ("validate", "list", "doctor"):
            argv = [str(binary), subcommand, "--repo", str(repo), "--json"]
            p = subprocess.run(["rtk", "proxy", *argv], env=env, capture_output=True, timeout=60)
            (case / (subcommand + ".json")).write_bytes(p.stdout)
            (case / (subcommand + ".stderr")).write_bytes(p.stderr)
            envelope = json.loads(p.stdout)
            code = 2 if name == "docker-unavailable" and subcommand == "doctor" else 0
            assert p.returncode == envelope["exitCode"] == code, (name, subcommand, envelope)
        argv = [str(binary), "run", "--repo", str(repo), "--job", "quality_gate", "--timeout", "1s" if name == "timeout" else "1m", "--json"]
        (case / "command.json").write_text(json.dumps(argv, indent=2) + "\n")
        with (case / "run.json").open("wb") as stdout, (case / "run.stderr").open("wb") as stderr:
            p = subprocess.Popen(argv, env=env, stdout=stdout, stderr=stderr)
            if name == "cancellation":
                deadline = time.monotonic() + 20
                while time.monotonic() < deadline and p.poll() is None:
                    if b"fixture sleeping" in (case / "run.stderr").read_bytes():
                        break
                    time.sleep(0.05)
                assert p.poll() is None, "cancellation fixture exited before signal"
                p.send_signal(signal.SIGINT)
            try:
                actual = p.wait(timeout=60)
            except subprocess.TimeoutExpired:
                p.send_signal(signal.SIGINT)
                try:
                    p.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    p.kill()
                    p.wait(timeout=5)
                raise RuntimeError("harness deadline exceeded; inspect retained run/cleanup evidence")
        envelope = json.loads((case / "run.json").read_text())
        assert actual == envelope["exitCode"] == expected, (name, actual, envelope)
        result = envelope.get("result") or {}
        jobs = result.get("jobs", [])
        if name == "pass":
            assert jobs and all(j["status"] == "passed" for j in jobs)
        elif name in ("test-failure", "missing-tool"):
            assert jobs[0]["status"] == "failed", jobs
            assert jobs[0]["engineExitCode"] == (7 if name == "test-failure" else 127), jobs
        elif name in ("timeout", "cancellation"):
            assert any(j.get("reasonCode") == ("TIMEOUT" if name == "timeout" else "CANCELLED") for j in jobs), jobs
        else:
            assert not jobs and envelope.get("error"), envelope
        inspected = []
        report_path = result.get("reportPath")
        if report_path:
            report = Path(report_path)
            saved = case / "report-files"
            saved.mkdir()
            # Preserve raw report, logs and small step evidence, not environment credentials.
            for path in report.parent.rglob("*"):
                if path.is_file() and (path.name == "report.json" or path.suffix in (".stdout", ".stderr", ".exit") or path.name == "complete"):
                    relative = path.relative_to(report.parent)
                    target = saved / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(path.read_bytes())
                    inspected.append(str(relative))
            assert json.loads(report.read_text())["exitCode"] == expected
        assert (repo / ".pitstop.json").read_bytes() == config_before
        row = {"runId": result.get("runId"), "case": name, "exitCode": actual, "expectedExitCode": expected, "jobs": [{"id": j["id"], "status": j["status"], "reasonCode": j.get("reasonCode")} for j in jobs], "error": envelope.get("error"), "warnings": result.get("warnings", []), "inspectedReportFiles": inspected}
        (case / "assertions.json").write_text(json.dumps(row, indent=2) + "\n")
        summary.append(row)
        print(name + ": expected exit " + str(actual) + " verified", flush=True)
    owned = ""
    for row in summary:
        if row["runId"]:
            owned += subprocess.check_output(["rtk", "proxy", "docker", "ps", "-aq", "--filter", "label=com.pitstop.run=" + row["runId"]], text=True)
    assert not owned.strip(), "trial-owned containers remained after cleanup"
    (root / "remaining-pitstop-containers.txt").write_text(owned)
    (root / "summary.json").write_text(json.dumps({"binarySha256": hashlib.sha256(binary.read_bytes()).hexdigest(), "platform": platform, "runtime": "real Docker", "cases": summary}, indent=2) + "\n")
    print("All six real-Docker scenario assertions passed", flush=True)


if __name__ == "__main__":
    main()
