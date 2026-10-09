# Terminal demo — actual local CLI output

Captured 2026-10-09 with the current checkout, Linux arm64 Docker and cached `alpine:3.22`. Fixtures, HOME/cache and generated configurations are temporary; project commands run only inside Docker. Output below is actual human output. Run IDs, durations and temporary paths vary between runs. The fixtures and their report/log files were removed after capture; the paths below are historical output. Command arguments are shell-quoted for reproduction.

## Passing run

```text
$ pitstop init --preset custom --image alpine:3.22 --arch linux/arm64 --command "printf 'check: OK\\n'" --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass
init: preset=custom architecture=linux/arm64 config=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass/.pitstop.json dryRun=false written=true destinationExists=false
[exit 0]
```

```text
$ pitstop validate --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass
validate: valid=true repo=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass config=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass/.pitstop.json architecture=linux/arm64 checks=1 steps=1
[exit 0]
```

```text
$ pitstop skill install --agent codex --scope project --dry-run --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass
skill install: agent=codex scope=project action=install installed=false target=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass/.agents/skills/pitstop-config version=1 dryRun=true
[exit 0]
```

```text
$ pitstop doctor --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass
doctor: ready=true
  repository: ready ready
  config: ready ready
  image: ready image available
  docker: ready ready
[exit 0]
```

```text
$ pitstop run --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/pass
step: step-1 step-1
check: OK
pitstop run 1791559818857330000: exit 0 engine=docker selected=complete workflow=not_applicable duration=769ms report=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/home/Library/Caches/pitstop/runs/1791559818857330000/report.json
  verify: passed log=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/home/Library/Caches/pitstop/runs/1791559818857330000/verify.stdout
[exit 0]
```

Job log (shown separately; the CLI summary points to this file):
```text
step: step-1 step-1
check: OK
```

## Failing run

```text
$ pitstop init --preset custom --image alpine:3.22 --arch linux/arm64 --command "printf 'check: FAILED\\n'; exit 1" --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail
init: preset=custom architecture=linux/arm64 config=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail/.pitstop.json dryRun=false written=true destinationExists=false
[exit 0]
```

```text
$ pitstop validate --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail
validate: valid=true repo=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail config=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail/.pitstop.json architecture=linux/arm64 checks=1 steps=1
[exit 0]
```

```text
$ pitstop run --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail
step: step-1 step-1
check: FAILED
pitstop run 1791559819699075000: exit 1 engine=docker selected=complete workflow=not_applicable duration=560ms report=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/home/Library/Caches/pitstop/runs/1791559819699075000/report.json
  verify: failed check step exited with code 1 log=/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/home/Library/Caches/pitstop/runs/1791559819699075000/verify.stdout
[exit 1]
```

Job log (shown separately; the CLI summary points to this file):
```text
step: step-1 step-1
check: FAILED
```

## Invalid configuration

```text
$ pitstop validate --repo /var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail
pitstop: field has an invalid type (CONFIG_INVALID)
validate: valid=false repo=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail config=/private/var/folders/zr/34vf_kj17tn8k4jqs7qy5yz40000gn/T/pitstop-terminal-demo-f49xq2k2/fail/.pitstop.json architecture= checks=0 steps=0
  JSON_TYPE /runner: field has an invalid type
[exit 2]
```
