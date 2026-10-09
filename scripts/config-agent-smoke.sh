#!/bin/sh
set -eu

PITSTOP_BIN=${PITSTOP_BIN:-${1:-}}
case "$PITSTOP_BIN" in
  /*) ;;
  *) echo "PITSTOP_BIN must be an absolute path" >&2; exit 2 ;;
esac
[ -x "$PITSTOP_BIN" ] || { echo "PITSTOP_BIN is not executable: $PITSTOP_BIN" >&2; exit 2; }

ROOT=$(mktemp -d "${TMPDIR:-/tmp}/pitstop-config-agent.XXXXXX")
HOME="$ROOT/home"
CACHE="$ROOT/cache"
mkdir -p "$HOME" "$CACHE/go-build"
export HOME XDG_CACHE_HOME="$CACHE" GOCACHE="$CACHE/go-build"
trap 'if [ "${PITSTOP_CONFIG_AGENT_KEEP:-}" = 1 ]; then echo "smoke fixture retained at $ROOT" >&2; else rm -rf "$ROOT"; fi' EXIT INT TERM

JSON_VALIDATOR="$ROOT/validate-json"
go build -o "$JSON_VALIDATOR" ./scripts/validate-json.go

run_json() {
  expected=$1
  output=$2
  shift 2
  set +e
  "$PITSTOP_BIN" "$@" >"$output" 2>"$output.stderr"
  actual=$?
  set -e
  if [ "$actual" -ne "$expected" ]; then
    cat "$output.stderr" >&2
    cat "$output"
    echo "expected exit $expected, got $actual: $*" >&2
    exit 1
  fi
  "$JSON_VALIDATOR" <"$output"
}

assert_envelope() {
  python3 - "$1" "$2" "$3" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
assert data["command"] == sys.argv[2], data
assert data["exitCode"] == int(sys.argv[3]), data
assert data["schemaVersion"] == 1, data
PY
}

ARCH=$(uname -m)
case "$ARCH" in
  arm64|aarch64) ARCH=linux/arm64 ;;
  x86_64|amd64) ARCH=linux/amd64 ;;
  *) echo "unsupported host architecture: $ARCH" >&2; exit 2 ;;
esac

INIT_REPO="$ROOT/init repo"
mkdir -p "$INIT_REPO"
git -C "$INIT_REPO" init -q

OUT="$ROOT/init-dry-run.json"
run_json 0 "$OUT" init --repo "$INIT_REPO" --preset base --arch "$ARCH" --dry-run --json
assert_envelope "$OUT" init 0
python3 - "$OUT" "$INIT_REPO/.pitstop.json" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["dryRun"] is True, result
assert result["written"] is False, result
assert result["destinationExists"] is False, result
assert result["preset"] == "base", result
assert not pathlib.Path(sys.argv[2]).exists()
PY

OUT="$ROOT/init.json"
run_json 0 "$OUT" init --repo "$INIT_REPO" --preset base --arch "$ARCH" --json
assert_envelope "$OUT" init 0
python3 - "$OUT" "$INIT_REPO/.pitstop.json" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["dryRun"] is False, result
assert result["written"] is True, result
assert result["config"]["schemaVersion"] == 2, result
assert result["config"]["runner"]["architecture"] == result["architecture"], result
assert pathlib.Path(sys.argv[2]).is_file()
PY

cp "$INIT_REPO/.pitstop.json" "$ROOT/original-config.json"
OUT="$ROOT/init-collision.json"
run_json 2 "$OUT" init --repo "$INIT_REPO" --preset base --arch "$ARCH" --json
assert_envelope "$OUT" init 2
python3 - "$OUT" "$INIT_REPO/.pitstop.json" "$ROOT/original-config.json" <<'PY'
import json
import pathlib
import sys

data = json.load(open(sys.argv[1]))
assert data["error"]["code"] == "CONFIG_EXISTS", data
assert pathlib.Path(sys.argv[2]).read_bytes() == pathlib.Path(sys.argv[3]).read_bytes()
PY

OUT="$ROOT/validate.json"
run_json 0 "$OUT" validate --repo "$INIT_REPO" --json
assert_envelope "$OUT" validate 0
python3 - "$OUT" <<'PY'
import json
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["valid"] is True, result
assert result["checkCount"] >= 1, result
assert result["stepCount"] >= 1, result
assert result["issues"] == [], result
PY

cat >"$INIT_REPO/.pitstop.json" <<'JSON'
{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify","verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}
JSON
OUT="$ROOT/validate-duplicate.json"
run_json 2 "$OUT" validate --repo "$INIT_REPO" --json
assert_envelope "$OUT" validate 2
python3 - "$OUT" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
assert data["error"]["code"] == "CONFIG_INVALID", data
assert data["result"]["valid"] is False, data
assert data["result"]["issues"][0]["code"] == "ID_DUPLICATE", data
assert data["result"]["issues"][0]["path"] == "/jobs/1", data
PY

CUSTOM_REPO="$ROOT/custom"
mkdir -p "$CUSTOM_REPO"
git -C "$CUSTOM_REPO" init -q
OUT="$ROOT/init-custom.json"
run_json 0 "$OUT" init --repo "$CUSTOM_REPO" --preset custom --image alpine:3.22 --arch "$ARCH" --command 'printf custom-smoke' --json
assert_envelope "$OUT" init 0
python3 - "$OUT" "$CUSTOM_REPO/.pitstop.json" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["preset"] == "custom", result
assert result["config"]["runner"]["image"] == "alpine:3.22", result
assert result["config"]["checks"][0]["steps"][0]["run"] == "printf custom-smoke", result
assert pathlib.Path(sys.argv[2]).is_file()
PY

SKILL_REPO="$ROOT/skill-project"
mkdir -p "$SKILL_REPO"
git -C "$SKILL_REPO" init -q
SKILL_TARGET="$SKILL_REPO/.agents/skills/pitstop-config"
OUT="$ROOT/skill-dry-run.json"
run_json 0 "$OUT" skill install --agent codex --scope project --repo "$SKILL_REPO" --dry-run --json
assert_envelope "$OUT" "skill install" 0
python3 - "$OUT" "$SKILL_TARGET" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["dryRun"] is True, result
assert result["installed"] is False, result
assert result["action"] == "install", result
assert result["files"], result
assert not pathlib.Path(sys.argv[2]).exists()
PY

OUT="$ROOT/skill-install.json"
run_json 0 "$OUT" skill install --agent codex --scope project --repo "$SKILL_REPO" --json
assert_envelope "$OUT" "skill install" 0
python3 - "$OUT" "$SKILL_TARGET" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["dryRun"] is False, result
assert result["installed"] is True, result
assert result["action"] == "install", result
assert pathlib.Path(sys.argv[2], "SKILL.md").is_file()
assert pathlib.Path(sys.argv[2], "manifest.json").is_file()
PY

OUT="$ROOT/skill-collision.json"
run_json 2 "$OUT" skill install --agent codex --scope project --repo "$SKILL_REPO" --json
assert_envelope "$OUT" "skill install" 2
python3 - "$OUT" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
assert data["error"]["code"] == "SKILL_EXISTS", data
assert data["result"]["action"] == "blocked", data
assert data["result"]["installed"] is False, data
PY

OUT="$ROOT/skill-force.json"
run_json 0 "$OUT" skill install --agent codex --scope project --repo "$SKILL_REPO" --force --json
assert_envelope "$OUT" "skill install" 0
python3 - "$OUT" "$SKILL_TARGET" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["action"] == "replace", result
assert result["installed"] is True, result
target = pathlib.Path(sys.argv[2])
assert (target / "SKILL.md").is_file(), target
assert not list(target.parent.glob(".pitstop-config.*")), list(target.parent.glob(".pitstop-config.*"))
PY

for config_dir in "$SKILL_TARGET/assets/examples" "$SKILL_TARGET/assets/mappings"; do
  for source in "$config_dir"/*.json; do
    [ -f "$source" ] || continue
    name=$(basename "$source" .json)
    parse_repo="$ROOT/parse-$name"
    mkdir -p "$parse_repo"
    git -C "$parse_repo" init -q
    cp "$source" "$parse_repo/.pitstop.json"
    OUT="$ROOT/parse-$name.json"
    run_json 0 "$OUT" validate --repo "$parse_repo" --json
    assert_envelope "$OUT" validate 0
    python3 - "$OUT" <<'PY'
import json
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["valid"] is True, result
assert result["checkCount"] >= 1, result
assert result["stepCount"] >= 1, result
assert result["issues"] == [], result
PY
  done
done

echo "offline config-agent smoke passed: init/validate/custom/skill and all embedded examples/mappings"

if [ "${PITSTOP_CONFIG_AGENT_RUNTIME:-1}" = 0 ]; then
  echo "runtime toolchain smoke skipped (PITSTOP_CONFIG_AGENT_RUNTIME=0); Docker evidence is unverified"
  exit 0
fi
if ! docker info >/dev/null 2>&1; then
  echo "runtime toolchain smoke unverified: Docker daemon unavailable"
  exit 0
fi

make_runtime_fixture() {
  name=$1
  fixture="$ROOT/runtime-$name"
  mkdir -p "$fixture"
  git -C "$fixture" init -q
  git -C "$fixture" config user.name "Pitstop smoke"
  git -C "$fixture" config user.email "pitstop@example.invalid"
  echo "$fixture"
}

NODE_REPO=$(make_runtime_fixture node)
cat >"$NODE_REPO/package.json" <<'JSON'
{"name":"pitstop-smoke-node","version":"1.0.0","scripts":{"check":"node -e \"if (process.platform !== 'linux') process.exit(1)\""}}
JSON
cat >"$NODE_REPO/package-lock.json" <<'JSON'
{"name":"pitstop-smoke-node","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"pitstop-smoke-node","version":"1.0.0"}}}
JSON
cat >"$NODE_REPO/.pitstop.json" <<JSON
{"schemaVersion":2,"runner":{"image":"node:22-bookworm","architecture":"$ARCH"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"install","run":"npm ci"},{"id":"check","run":"npm run check"}]}]}
JSON
git -C "$NODE_REPO" add . && git -C "$NODE_REPO" commit -qm "node manifest"

GO_REPO=$(make_runtime_fixture go)
cat >"$GO_REPO/go.mod" <<'EOF'
module example.invalid/pitstop-smoke-go

go 1.26.5
EOF
cat >"$GO_REPO/main.go" <<'EOF'
package main

func main() {}
EOF
OUT="$ROOT/runtime-go-init.json"
run_json 0 "$OUT" init --repo "$GO_REPO" --preset go --arch "$ARCH" --json
assert_envelope "$OUT" init 0
python3 - "$OUT" "$GO_REPO/.pitstop.json" <<'PY'
import json
import pathlib
import sys

result = json.load(open(sys.argv[1]))["result"]
assert result["preset"] == "go", result
assert result["written"] is True, result
assert result["config"]["checks"][0]["steps"][-1]["run"] == "go build -buildvcs=false ./...", result
assert pathlib.Path(sys.argv[2]).is_file()
PY
git -C "$GO_REPO" add . && git -C "$GO_REPO" commit -qm "go manifest"

PYTHON_REPO=$(make_runtime_fixture python)
cat >"$PYTHON_REPO/requirements.txt" <<'EOF'
pytest==8.3.5
EOF
cat >"$PYTHON_REPO/test_smoke.py" <<'EOF'
def test_runtime_fixture():
    assert True
EOF
cat >"$PYTHON_REPO/.pitstop.json" <<JSON
{"schemaVersion":2,"runner":{"image":"python:3.13-slim-bookworm","architecture":"$ARCH"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"install","run":"python -m pip install --requirement requirements.txt"},{"id":"test","run":"python -m pytest"}]}]}
JSON
git -C "$PYTHON_REPO" add . && git -C "$PYTHON_REPO" commit -qm "python manifest lock"

RUST_REPO=$(make_runtime_fixture rust)
mkdir -p "$RUST_REPO/src"
cat >"$RUST_REPO/Cargo.toml" <<'EOF'
[package]
name = "pitstop-smoke-rust"
version = "0.1.0"
edition = "2021"
EOF
cat >"$RUST_REPO/Cargo.lock" <<'EOF'
version = 4

[[package]]
name = "pitstop-smoke-rust"
version = "0.1.0"
EOF
cat >"$RUST_REPO/src/lib.rs" <<'EOF'
pub fn smoke() -> bool { true }

#[cfg(test)]
mod tests {
    #[test]
    fn runtime_fixture() { assert!(super::smoke()); }
}
EOF
cat >"$RUST_REPO/.pitstop.json" <<JSON
{"schemaVersion":2,"runner":{"image":"rust:1.99.0-bookworm","architecture":"$ARCH"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"test","run":"cargo test --locked"},{"id":"build","run":"cargo build --locked"}]}]}
JSON
git -C "$RUST_REPO" add . && git -C "$RUST_REPO" commit -qm "rust manifest lock"

for name in node go python rust; do
  repo="$ROOT/runtime-$name"
  OUT="$ROOT/runtime-$name.json"
  echo "runtime toolchain selected: $name"
  run_json 0 "$OUT" run --repo "$repo" --json
  assert_envelope "$OUT" run 0
  python3 - "$OUT" "$name" <<'PY'
import json
import sys

data = json.load(open(sys.argv[1]))
result = data["result"]
assert result["selectedCoverage"] == "complete", data
assert result["workflowCoverage"] == "not_applicable", data
assert result["engine"]["name"] == "docker", data
assert result["jobs"] and all(job["status"] == "passed" for job in result["jobs"]), data
assert all(step["status"] == "passed" for job in result["jobs"] for step in job["steps"]), data
print("runtime passed:", sys.argv[2], result["engine"]["image"], result["engine"].get("imageId", ""))
PY
done

echo "runtime toolchain smoke passed: Node, Go, Python, Rust"
