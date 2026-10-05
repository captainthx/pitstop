from pathlib import Path
import json, subprocess, tempfile, sys
binary=Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory(prefix="pitstop-composite-check-") as tmp:
 repo=Path(tmp)
 subprocess.run(["git","init","-q",tmp],check=True)
 action=repo/".github/actions/check";action.mkdir(parents=True)
 (action/"action.yml").write_text("runs:\n  using: composite\n  steps:\n    - if: false\n      shell: bash\n      run: exit 42\n")
 workflow=repo/".github/workflows";workflow.mkdir(parents=True)
 (workflow/"ci.yml").write_text("on: pull_request\njobs:\n  verify:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: ./.github/actions/check\n")
 (repo/".pitstop.json").write_text(json.dumps({"schemaVersion":1,"workflow":".github/workflows/ci.yml","event":"pull_request","jobs":["verify"],"runner":{"label":"ubuntu-latest","image":"catthehacker/ubuntu:act-latest","architecture":"linux/arm64"}}))
 subprocess.run(["git","-C",tmp,"add","."],check=True)
 subprocess.run(["git","-C",tmp,"-c","user.name=Test","-c","user.email=test@localhost","commit","-qm","fixture"],check=True)
 result=subprocess.run([str(binary),"run","--repo",tmp,"--json"],capture_output=True,text=True,timeout=60)
 envelope=json.loads(result.stdout)
 assert result.returncode==2 and envelope["result"]["jobs"][0]["status"]=="blocked",envelope
 print("Local composite no-op blocked; no false green.")
