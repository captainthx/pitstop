#!/bin/sh
set -eu

PITSTOP_BIN=${PITSTOP_BIN:-./pitstop}
IMAGE=${PITSTOP_SMOKE_IMAGE:-alpine:3.22}
ROOT=$(mktemp -d)
UNRELATED=""
trap 'if [ -n "$UNRELATED" ]; then docker rm -f "$UNRELATED" >/dev/null 2>&1 || true; fi; if [ "${PITSTOP_SMOKE_KEEP:-}" != 1 ]; then rm -rf "$ROOT"; else printf "smoke fixture retained at %s\\n" "$ROOT" >&2; fi' EXIT INT TERM
REPO="$ROOT/fixture with spaces"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.name Test
git -C "$REPO" config user.email test@localhost
printf 'clean\n' > "$REPO/tracked.txt"
printf 'ignored.txt\n' > "$REPO/.gitignore"
printf 'deleted\n' > "$REPO/deleted.txt"
git -C "$REPO" add .
git -C "$REPO" commit -qm fixture
printf 'dirty\n' > "$REPO/tracked.txt"
rm "$REPO/deleted.txt"
printf 'ignored\n' > "$REPO/ignored.txt"
printf 'new\n' > "$REPO/new.txt"
ARCH=$(docker version --format '{{.Server.Arch}}')
ARCH=${ARCH#linux/}
case "$ARCH" in aarch64) ARCH=arm64 ;; x86_64) ARCH=amd64 ;; esac
cat > "$REPO/.pitstop.json" <<EOF
{
  "schemaVersion": 2,
  "runner": {"image": "$IMAGE", "architecture": "linux/$ARCH"},
  "jobs": ["verify", "independent"],
  "checks": [
    {"id":"verify","steps":[
      {"id":"source","run":"test \"\$(cat tracked.txt)\" = dirty && test -f new.txt && test ! -e deleted.txt && test ! -e ignored.txt"},
      {"id":"write","run":"printf generated > generated.txt"},
      {"id":"reuse","run":"test \"\$(cat generated.txt)\" = generated"}
    ]},
    {"id":"independent","steps":[{"id":"fresh","run":"test ! -e generated.txt"}]}
  ]
}
EOF
BEFORE=$(git -C "$REPO" status --porcelain=v1)
OUT="$ROOT/result.json"
if ! "$PITSTOP_BIN" run --repo "$REPO" --json > "$OUT"; then
  cat "$OUT"
  exit 1
fi
python3 - "$OUT" "$REPO" <<'PY'
import json, pathlib, subprocess, sys
r = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert r['exitCode'] == 0, r
assert r['result']['engine']['name'] == 'docker', r
assert r['result']['engine']['imageId'].startswith('sha256:'), r
assert r['result']['selectedCoverage'] == 'complete', r
assert r['result']['workflowCoverage'] == 'not_applicable', r
assert all(j['status'] == 'passed' for j in r['result']['jobs']), r
assert all(s['status'] == 'passed' for j in r['result']['jobs'] for s in j['steps']), r
assert pathlib.Path(r['result']['reportPath']).exists(), r
assert not (pathlib.Path(sys.argv[2]) / 'generated.txt').exists()
PY
AFTER=$(git -C "$REPO" status --porcelain=v1)
[ "$BEFORE" = "$AFTER" ]
RUN_ID=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["result"]["runId"])' "$OUT")
[ -z "$(docker ps -aq --filter "label=com.pitstop.run=$RUN_ID")" ]

python3 - "$REPO/.pitstop.json" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1]); c = json.loads(p.read_text())
c['checks'][0]['steps'][1]['run'] = 'exit 7'
c['checks'][0]['steps'][2]['run'] = 'touch must-not-run'
p.write_text(json.dumps(c))
PY
if "$PITSTOP_BIN" run --repo "$REPO" --json > "$ROOT/failure.json"; then exit 1; fi
python3 - "$ROOT/failure.json" "$REPO" <<'PY'
import json, pathlib, sys
r = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert r['exitCode'] == 1, r
jobs = {j['id']: j for j in r['result']['jobs']}
assert jobs['verify']['status'] == 'failed', r
assert jobs['verify']['steps'][2]['status'] == 'skipped', r
assert jobs['independent']['status'] == 'passed', r
assert not (pathlib.Path(sys.argv[2]) / 'must-not-run').exists()
PY

python3 - "$REPO/.pitstop.json" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1]); c = json.loads(p.read_text())
c['checks'][0]['steps'][1]['run'] = 'sleep 30'
p.write_text(json.dumps(c))
PY
if "$PITSTOP_BIN" run --repo "$REPO" --timeout 1s --json > "$ROOT/timeout.json"; then exit 1; fi
test -s "$ROOT/timeout.json" || { cat "$ROOT/timeout.json"; exit 1; }
python3 - "$ROOT/timeout.json" <<'PY'
import json, pathlib, sys
r = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert r['exitCode'] == 1, r
assert any(j.get('reasonCode') == 'TIMEOUT' for j in r['result']['jobs']), r
PY

UNRELATED=$(docker run -d --label com.pitstop.run=unrelated-smoke "$IMAGE" sleep 30)
PITSTOP_BIN="$PITSTOP_BIN" REPO="$REPO" ROOT="$ROOT" UNRELATED="$UNRELATED" python3 - <<'PY'
import json, os, pathlib, signal, subprocess, time
binary, repo, root, unrelated = os.environ['PITSTOP_BIN'], os.environ['REPO'], os.environ['ROOT'], os.environ['UNRELATED']
unrelated_short = unrelated[:12]
baseline = set(subprocess.run(['docker', 'ps', '-q', '--filter', 'label=com.pitstop.run'], capture_output=True, text=True, check=True).stdout.split())
out_path = pathlib.Path(root) / 'cancel.json'
err_path = pathlib.Path(root) / 'cancel.progress'
with out_path.open('w') as out, err_path.open('w') as err:
    p = subprocess.Popen([binary, 'run', '--repo', repo, '--json'], stdout=out, stderr=err)
    owned = ''
    for _ in range(100):
        ids = subprocess.run(['docker', 'ps', '-q', '--filter', 'label=com.pitstop.run'], capture_output=True, text=True, check=True).stdout.split()
        owned = next((x for x in ids if x != unrelated_short and x not in baseline), '')
        if owned: break
        time.sleep(0.1)
    assert owned, 'owned container did not start'
    p.send_signal(signal.SIGINT)
    code = p.wait(timeout=30)
assert code == 130, code
r = json.loads(out_path.read_text())
assert r['exitCode'] == 130, r
assert 'result' in r and any(j.get('reasonCode') == 'CANCELLED' for j in r['result']['jobs']), r
assert subprocess.run(['docker', 'ps', '-q', '--filter', 'label=com.pitstop.run=unrelated-smoke'], capture_output=True, text=True, check=True).stdout.strip() == unrelated_short
remaining = subprocess.run(['docker', 'ps', '-q', '--filter', 'label=com.pitstop.run'], capture_output=True, text=True, check=True).stdout.split()
assert all(x == unrelated_short for x in remaining), remaining
PY
printf 'docker smoke passed (%s, linux/%s)\n' "$IMAGE" "$ARCH"
