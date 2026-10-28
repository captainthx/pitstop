"""Real nested-daemon acceptance; all projects and resources are disposable."""
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time


def command(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, capture_output=True, **kwargs).stdout.strip()


binary = str(Path(os.environ.get("PITSTOP_BIN", "./pitstop")).resolve())
docker = shutil.which("docker")
assert docker, "Docker CLI required"
root = Path(tempfile.mkdtemp(prefix="pitstop-testcontainers-"))
repo = root / "fixture with spaces"
fixture = Path(__file__).parent / "testdata" / "testcontainers"
shutil.copytree(fixture, repo)
arch = command(docker, "version", "--format", "{{.Server.Arch}}")
arch = {"aarch64": "arm64", "x86_64": "amd64"}.get(arch, arch)
command("git", "-C", str(repo), "init", "-q")
command("git", "-C", str(repo), "-c", "user.name=Smoke", "-c", "user.email=smoke@localhost", "add", ".")
command("git", "-C", str(repo), "-c", "user.name=Smoke", "-c", "user.email=smoke@localhost", "commit", "-qm", "generic fixture")
check_id = "integration-" + root.name.rsplit("-", 1)[-1].replace("_", "a")
config = {
    "schemaVersion": 2,
    "runner": {"image": "golang:1.26.5", "architecture": "linux/" + arch},
    "jobs": [check_id],
    "checks": [{"id": check_id, "testcontainers": True, "steps": [
        {"id": "service", "run": "go test -race -count=1 -tags integration -v ./..."}]}],
}
config_path = repo / ".pitstop.json"


def write_config(run):
    config["checks"][0]["steps"][0]["run"] = run
    config_path.write_text(json.dumps(config))


def assert_clean(run_id):
    for args in [("ps", "-aq"), ("network", "ls", "-q"), ("volume", "ls", "-q")]:
        remaining = command(docker, *args, "--filter", "label=com.pitstop.run=" + run_id)
        assert not remaining, (args, remaining)


def run_case(name, expected, extra=(), env=None, cancel=False):
    out_path = root / (name + ".json")
    with out_path.open("w") as out, (root / (name + ".stderr")).open("w") as err:
        process = subprocess.Popen([binary, "run", "--repo", str(repo), "--json", *extra],
                                   stdout=out, stderr=err, env=env)
        try:
            if cancel:
                deadline = time.monotonic() + 120
                while time.monotonic() < deadline:
                    # Wait for a runner: cancellation then exercises daemon + network + data cleanup.
                    ids = command(docker, "ps", "-q", "--filter", "label=com.pitstop.run").split()
                    found = False
                    for cid in ids:
                        name_value = command(docker, "inspect", "--format", "{{.Name}}", cid)
                        if name_value.endswith("-" + check_id):
                            found = True
                            break
                    if found:
                        process.send_signal(signal.SIGINT)
                        break
                    if process.poll() is not None:
                        break
                    time.sleep(.2)
                else:
                    process.kill()
                    raise AssertionError("runner did not start for cancellation")
            code = process.wait(timeout=900)
        finally:
            if process.poll() is None:
                process.send_signal(signal.SIGINT)
                try:
                    process.wait(timeout=30)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=10)
    result = json.loads(out_path.read_text())
    assert code == expected and result["exitCode"] == expected, (name, code, result)
    if "result" in result:
        value = result["result"]
        assert_clean(value["runId"])
        assert json.loads(Path(value["reportPath"]).read_text()) == result
    return result


unrelated = "pitstop-testcontainers-unrelated-" + root.name.rsplit("-", 1)[-1]
try:
    command(docker, "network", "create", "--label", "com.pitstop.run=" + unrelated, unrelated)
    command(docker, "volume", "create", "--label", "com.pitstop.run=" + unrelated, unrelated)
    write_config("go test -race -count=1 -tags integration -v ./...")
    before = command("git", "-C", str(repo), "status", "--porcelain=v1")
    before_resources = {args: command(docker, *args, "--filter", "label=com.pitstop.run") for args in [("ps", "-aq"), ("network", "ls", "-q"), ("volume", "ls", "-q")]}
    blocked = run_case("authorization-blocked", 2)
    assert all(command(docker, *args, "--filter", "label=com.pitstop.run") == value for args, value in before_resources.items()), blocked
    assert blocked.get("error", {}).get("code") == "TESTCONTAINERS_AUTH_REQUIRED", blocked
    passed = run_case("passed", 0, ("--allow-testcontainers",))
    job = passed["result"]["jobs"][0]
    assert job["status"] == "passed", passed
    log = Path(job["logPath"]).read_text()
    assert "mapped service round-trip succeeded at http://pitstop-dind:" in log, log
    assert "--- PASS: TestServiceRoundTrip" in log, log
    service = job["testcontainers"]
    assert service["daemonImageId"].startswith("sha256:"), service
    assert service["readiness"] == "ready", service
    write_config("PITSTOP_FIXTURE_FAIL=1 go test -count=1 -tags integration -v ./...")
    failed = run_case("assertion-failed", 1, ("--allow-testcontainers",))
    assert failed["result"]["jobs"][0]["status"] == "failed", failed
    assert "intentional assertion failure" in Path(failed["result"]["jobs"][0]["logPath"]).read_text()
    assert Path(failed["result"]["jobs"][0]["testcontainers"]["diagnostics"]).exists()
    write_config("printf timeout-step-started; sleep 120")
    run_case("provision-timeout", 1, ("--allow-testcontainers", "--timeout", "2s"))
    timed = run_case("runtime-timeout", 1, ("--allow-testcontainers", "--timeout", "45s"))
    timed_job = timed["result"]["jobs"][0]
    assert timed_job["reasonCode"] == "TIMEOUT", timed
    assert timed_job["testcontainers"]["readiness"] == "ready", timed
    assert "timeout-step-started" in Path(timed_job["logPath"]).read_text(), timed
    assert Path(timed_job["testcontainers"]["diagnostics"]).exists()
    run_case("cancelled", 130, ("--allow-testcontainers",), cancel=True)

    # Inject only the outer Docker command failure; provisioning still allocates real resources.
    wrapper_dir = root / "wrapper"
    wrapper_dir.mkdir()
    wrapper = wrapper_dir / "docker"
    wrapper.write_text("#!/usr/bin/env python3\n" +
        "import os, subprocess, sys\n" +
        "args=sys.argv[1:]\n" +
        "real=os.environ['PITSTOP_SMOKE_REAL_DOCKER']\n" +
        "if os.environ.get('PITSTOP_SMOKE_INJECT') == 'copy' and 'cp' in args: sys.exit(72)\n" +
        "if os.environ.get('PITSTOP_SMOKE_INJECT') != 'copy' and 'start' in args:\n" +
        "    name=subprocess.run([real,*args[:args.index('start')],'inspect','--format','{{.Name}}',args[-1]],capture_output=True,text=True).stdout.strip()\n" +
        "    if name.endswith('-dind'): sys.exit(71)\n" +
        "os.execv(real,[real,*args])\n")
    wrapper.chmod(0o755)
    injected_env = dict(os.environ, PATH=str(wrapper_dir) + os.pathsep + os.environ["PATH"],
                        PITSTOP_SMOKE_REAL_DOCKER=docker)
    startup = run_case("daemon-start-failed", 2, ("--allow-testcontainers",), env=injected_env)
    assert startup["result"]["jobs"][0]["status"] != "passed", startup
    injected_env["PITSTOP_SMOKE_INJECT"] = "copy"
    copied = run_case("source-copy-failed", 2, ("--allow-testcontainers",), env=injected_env)
    assert copied["result"]["jobs"][0]["status"] != "passed", copied
    assert copied["result"]["jobs"][0]["testcontainers"]["readiness"] == "ready", copied
    assert Path(copied["result"]["jobs"][0]["testcontainers"]["diagnostics"]).exists()
    command(docker, "network", "inspect", unrelated)
    command(docker, "volume", "inspect", unrelated)
    # Config edits are deliberate; test files and tracked source must stay byte-identical.
    assert command("git", "-C", str(repo), "diff", "--exit-code") == ""
    assert before == command("git", "-C", str(repo), "status", "--porcelain=v1")
    print("Testcontainers smoke passed (linux/" + arch + "); evidence: " + str(root))
finally:
    subprocess.run([docker, "network", "rm", unrelated], capture_output=True)
    subprocess.run([docker, "volume", "rm", unrelated], capture_output=True)
    if os.environ.get("PITSTOP_SMOKE_KEEP") != "1":
        shutil.rmtree(root)
    else:
        print("Testcontainers fixture retained at " + str(root), flush=True)
