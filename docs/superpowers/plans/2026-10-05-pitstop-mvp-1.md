> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../../README.th.md) สำหรับการใช้งาน Docker checks.

# Pitstop MVP 1 — Detailed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Execute natively and sequentially; subagents require explicit user authorization.

**Goal:** สร้าง CLI `pitstop` ที่รัน validation jobs จาก GitHub Actions workflow เดิมกับ working-tree snapshot และให้ผล human/JSON ที่ตรวจสอบย้อนหลังได้

**Architecture:** อ่าน config และตรวจ workflow ก่อนสร้าง source snapshot แยกจาก checkout ผู้ใช้ จากนั้นเรียก act ทีละ job เก็บ stdout/stderr และรายงานขอบเขตที่ตรวจจริง ใช้ subprocess และ Go standard library เป็นหลัก โดยไม่สร้าง Actions interpreter หรือ backend framework

**Tech Stack:** Go 1.26.5, Cobra, `go.yaml.in/yaml/v3`; external Git, act 0.2.89, Docker; initial tested target macOS ARM64 / Linux ARM64 runner

**Spec:** [pitstop-spec.md](../../design/pitstop-spec.md)
**Context:** [pitstop-context.md](../../design/pitstop-context.md)
**Original plan:** [pitstop-implementation-plan.md](../../design/pitstop-implementation-plan.md)
**Existing evidence:** [act-react-ci-report.md](../../evidence/act-react-ci-report.md)

วันที่: 2026-10-05 (Asia/Bangkok). เอกสารนี้ขยายแผนเดิมเพื่อ implementation โดยไม่เปลี่ยน product requirements สเปกเป็นข้อกำหนดหลัก; interfaces ด้านล่างเป็น implementation decisions และต้องแก้ caller/test/docs พร้อมกันเมื่อเปลี่ยน

## Global Constraints

- Workspace: `/Users/thanyatab/Desktop/project/my-lab/pitstop`; งานครั้งนี้เขียนเอกสารเท่านั้น ยังไม่สร้าง product code หรือ hosted repository
- Go toolchain baseline 1.26.5 และ act baseline 0.2.89 ตามสเปก; ตรวจเครื่องจริงใน Task 1 ก่อนอ้างว่าพร้อม
- ชื่อ executable `pitstop`; commands `doctor`, `list`, `run`, `version`; ไม่มี prompt หรือ automatic installation
- ใช้ Cobra, YAML v3 และ standard library; ไม่เพิ่ม Docker SDK, Viper, DB, daemon, scheduler หรือ plugin framework
- รัน YAML เดิม; config เก็บ selection/settings เท่านั้น ไม่คัดลอกคำสั่ง lint/test/build เข้า Go
- ห้ามแก้ recruit100-react/service เพื่อให้ probe ผ่าน; ห้าม deploy, migrate, seed, backfill หรืออ่าน credentials เพิ่ม
- Event `pull_request`; config schemaVersion=1; หนึ่ง workflow ต่อ config; `--job` repeatable และเป็น subset ของ configured jobs
- Architectures `linux/arm64`, `linux/amd64`; ไม่มี fallback อัตโนมัติ; compiled artifact ไม่เท่ากับ tested platform
- Generic execution รองรับ independent unconditional nonmatrix jobs บน static runner ที่ตรง config; integration adapter เป็นข้อยกเว้นเฉพาะ
- Default timeout 20m ต่อ act invocation; positive Go duration เท่านั้น; graceful stop 5s ก่อน kill
- Exit precedence: cancellation 130 > launched failure 1 > preflight/blocked/unknown 2 > success 0
- JSON mode: progress ไป stderr; stdout เป็น JSON document เดียว; help เป็น conventional text
- Execution ใช้ independent local clone `--no-hardlinks` + working-tree overlay; ห้าม bind original checkout
- ไม่รับ host cloud/token environment, implicit actrc/env/secret files หรือ gh authentication token
- `os.UserCacheDir()/pitstop`: persistent caches; unique run directory 0700; logs/reports 0600
- Cleanup เฉพาะ owned resource IDs; ห้าม docker prune และห้ามหยุด PostgreSQL/MinIO ที่มีอยู่
- Shell commands ในเครื่องนี้ขึ้นต้นด้วย `rtk`; `rtk proxy` เมื่อจำเป็นต้องรักษา output จริง
- ทำทีละ task ตาม dependency; แต่ละ task จบด้วย runnable checks และ focused commit เมื่อเริ่ม implementation ใน Git repo แล้ว

## Review Focus

1. **Working tree ต่างจาก index หรือเปลี่ยนระหว่าง capture:** ใช้ working-tree bytes จริงหรือหยุดด้วย SNAPSHOT_CHANGED — Task 6
2. **actrc/host env เพิ่ม job หรือ secrets:** isolation ต้องกันทุก lookup path รวมกรณี `-C` และ array flags — Tasks 1, 7
3. **exit 0 แต่ไม่รัน job ที่เลือก:** preflight + measured selection evidence ต้องกัน false success — Tasks 1, 5, 8
4. **Cancel/timeout ทิ้ง containers หรือฆ่างานคนอื่น:** process group + exact owned IDs + real cancellation check — Tasks 1, 7, 11
5. **Malformed/huge logs หรือ JSON error หลายเอกสาร:** drain สอง streams, bounded parsing และ output owner จุดเดียว — Tasks 7, 8, 9

---

## 1. สถานะเริ่มต้นและสิ่งที่ต้องส่งมอบ

หลักฐานเดิมพิสูจน์ React `verify` ด้วย act 0.2.89 บน Linux ARM64: 111.08s cold, 45.86s warm, exit 0 ทั้งสองรอบ และ tests 15/15 ผ่าน เป็นผลทดลองเดิม ไม่ใช่ผลการทดสอบ implementation ใหม่

ยังไม่พิสูจน์ Go lint/test/integration, dirty/untracked source, hostile actrc isolation, act JSON contract, paths-filter semantics และ cancellation cleanup ไม่มีงานใดใช้คำว่า supported จนมีหลักฐานของงานนั้น

Deliverables:

1. Binary `pitstop` และตัวอย่าง `.pitstop.json` สอง profiles
2. CLI/config/workflow/snapshot/engine/report ที่เชื่อมกันจริง
3. Tests สำหรับ trust boundaries และ lifecycle; explicit opt-in smoke สำหรับ Docker
4. Raw logs, engine contract, acceptance report และ measured support matrix
5. README สำหรับคนและ agent: prerequisites, commands, JSON, exits, scope, cleanup

## 2. โครงสร้างไฟล์และความรับผิดชอบ

| File | Responsibility |
|---|---|
| `cmd/pitstop/main.go` | สร้าง signal-aware context, ส่ง args/streams ให้ CLI, os.Exit ครั้งเดียว |
| `internal/cli/root.go` | Cobra commands/flags, command errors และเลือก human/JSON output |
| `internal/model/types.go` | Config, inventory, plan, job result, envelope และ typed reason codes |
| `internal/config/config.go` | Strict JSON, Git root, config และ repo-contained paths |
| `internal/workflow/inspect.go` | YAML inventory, capability checks, dependencies/local composites |
| `internal/workflow/integration.go` | Shape guard และแก้ needs/if ใน workflow copy |
| `internal/snapshot/snapshot.go` | Source inventory, independent clone/overlay, deterministic manifest |
| `internal/actengine/environment.go` | Tool checks, Docker endpoint, child environment/config isolation |
| `internal/actengine/engine.go` | act argv, raw logs, bounded optional JSONL parsing |
| `internal/actengine/process_unix.go` | Process groups, timeout/cancel, owned Docker cleanup |
| `internal/report/report.go` | JSON/human rendering, coverage และ exit aggregation |
| `internal/run/run.go` | Lock, run artifacts, orchestration และ final persistence |
| `examples/recruit100-{react,service}.json` | Selection profiles ตามสเปก |
| `testdata/workflows/` | Supported/blocked workflows และ local composite fixtures |
| `testdata/engine/` | Sanitized success/failure/cancel logs จาก act จริง |
| `scripts/smoke.sh` | Explicit opt-in real-engine acceptance commands |
| `docs/evidence/` | หลักฐานเดิมและใหม่; แยกชื่อ ไม่เขียนทับผลเดิม |

Tests อยู่ข้าง Go file ที่รับผิดชอบ (`*_test.go`) และสร้าง disposable Git repos ด้วย `t.TempDir()` ไม่ commit copied application repos, secrets, build artifacts หรือ source snapshots

ไม่สร้าง interface ต่อ module เพื่อ mock ทุกอย่าง ใช้ fake executables/helper subprocess ใน test ของ process boundary แยกไฟล์เพิ่มเมื่อความรับผิดชอบจำเป็นจริง

## 3. Shared contracts ที่ต้องตกลงก่อนเชื่อมโมดูล

### 3.1 Functions

```go
// config
ResolveRepo(ctx context.Context, path string) (string, error)
Load(repo string) (model.Config, error)
ResolveWorkflow(repo, relativePath string) (string, error)

// workflow
Inspect(repo, workflowPath string) (model.Inventory, error)
Plan(repo string, cfg model.Config, inv model.Inventory, opts model.Options) (model.Plan, error)
PrepareIntegration(sourcePath, destinationPath string) ([]model.Adjustment, error)

// snapshot
Capture(ctx context.Context, repo, runDir string) (model.Snapshot, error)
Remove(snapshot model.Snapshot) error

// actengine: resolve host settings once, before isolated HOME
Check(ctx context.Context) (Environment, error)
Run(ctx context.Context, env Environment, snapshot model.Snapshot,
    plan model.Plan, job model.PlannedJob, runDir string,
    timeout time.Duration, progress io.Writer) (model.JobResult, error)
Cleanup(ctx context.Context, runDir string) error

// run/report/cli
Execute(ctx context.Context, opts model.Options, progress io.Writer) (model.Envelope, error)
ExitCode(jobs []model.JobResult, cancelled bool) int
WriteEnvelope(w io.Writer, envelope model.Envelope) error
Human(w io.Writer, envelope model.Envelope) error
Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int // cli
```

ชื่อ `Execute` อยู่คนละ package; `Environment` เป็น struct ภายใน actengine เก็บ resolved executable paths, Docker connection settings และ actual EngineInfo ห้าม serialize child environment หรือ credentials

`Cleanup` อ่าน ownership evidence ของ run directory; ไม่อาศัย global mutable state ใช้ bounded fresh cleanup context หลัง execution context ถูก cancel แล้ว

### 3.2 Types / JSON

ใช้ชนิดข้อมูลใน original plan พร้อม explicit JSON tags และเพิ่ม fields ที่ coordinator ต้องส่งจริง:

- `Options`: Repo, Jobs, JSON, Timeout, Architecture, ForceIntegration
- `Config`: SchemaVersion, Workflow, Event, Jobs, Runner, Adapter
- `RunnerConfig`: Label, Image, Architecture
- `Inventory`: WorkflowPath, Jobs; `JobInfo` เก็บ ID/Name/RunsOn/If/Needs/SecretReferences/CapabilityReasons
- `Plan`: Repo, Config, Inventory, Jobs, ForceIntegration และ `ExecutionWorkflowPath` สำหรับ original/prepared YAML
- `PlannedJob`: ID, Eligible, Reasons
- `Snapshot`: Path, Source, ManifestPath
- `JobResult`: id, status, provenance, durationMs, engineExitCode, failureCategory, reasonCode, reason, steps, logPath
- `RunReport`: runId, repo, source, engine, jobs, selectedCoverage, workflowCoverage, adjustments, warnings, durationMs, reportPath
- `Envelope`: schemaVersion, command, exitCode, result, error; `ErrorInfo`: code, message

`JobResult` unavailable scalars ใช้ pointers เพื่อ JSON null; collections ต้องเป็น `[]` ไม่ใช่ null ทุก command result ที่ใส่ใน `Envelope.Result` เป็น concrete struct ไม่ใช่ map ที่ประกอบกระจายหลายจุด

DoctorResult: repo, checks[]; แต่ละ check มี name/status/reasonCode/message และ readiness ตามขอบเขตที่ตรวจ ListResult: repo, workflowPath, jobs[] พร้อม selection selected/excluded และ capability eligible/blocked VersionResult: version, commit, buildTime (null เมื่อไม่มีข้อมูล)

Run metadata แยกจาก RunReport: UTC startedAt/finishedAt, source HEAD/status evidence, original/prepared workflow SHA-256, argv ที่ไม่มี credentials, resource ownership และ cleanup outcomes ไม่ใส่ exitCode ซ้ำใน RunReport

Typed error ขั้นต่ำใช้ `model.Error` มี Code/Message/Cause และ Unwrap; ใช้ `errors.As/Is` ห้าม map exit จากข้อความ arbitrary error Stable reasons ตามสเปก: CONFIG_INVALID, TOOL_MISSING, ACT_VERSION_UNSUPPORTED, DOCKER_UNAVAILABLE, WORKFLOW_UNSUPPORTED, SECRET_REQUIRED, ADAPTER_SHAPE_CHANGED, SNAPSHOT_CHANGED, ENGINE_OUTCOME_UNKNOWN, TIMEOUT, CANCELLED, RUN_ALREADY_ACTIVE, CLEANUP_INCOMPLETE; integration runtime ใช้ INTEGRATION_RUNTIME_UNSUPPORTED

### 3.3 ตัวอย่าง assertions ที่ใช้เป็น contract tests

ใส่ใน `internal/report/report_test.go` โดยใช้ package report และ imports `testing`, `encoding/json`, `bytes`, `io`, `pitstop/internal/model` หลัง Task 3 สร้าง types แล้ว:

```go
func TestExitPrecedence(t *testing.T) {
    cases := []struct { statuses []string; cancelled bool; want int }{
        {[]string{"passed"}, false, 0},
        {[]string{"failed", "blocked"}, false, 1},
        {[]string{"passed", "blocked"}, false, 2},
        {[]string{"skipped"}, false, 2},
        {nil, false, 2},
        {[]string{"failed"}, true, 130},
    }
    for _, tc := range cases {
        jobs := make([]model.JobResult, 0, len(tc.statuses))
        for _, status := range tc.statuses {
            jobs = append(jobs, model.JobResult{Status: status})
        }
        if got := ExitCode(jobs, tc.cancelled); got != tc.want {
            t.Fatalf("statuses=%v cancelled=%v: got %d, want %d",
                tc.statuses, tc.cancelled, got, tc.want)
        }
    }
}

func TestEnvelopeIsOneDocument(t *testing.T) {
    var output bytes.Buffer
    want := model.Envelope{SchemaVersion: 1, Command: "run", ExitCode: 2,
        Error: &model.ErrorInfo{Code: "CONFIG_INVALID", Message: "missing config"}}
    if err := WriteEnvelope(&output, want); err != nil { t.Fatal(err) }
    dec := json.NewDecoder(&output)
    var got model.Envelope
    if err := dec.Decode(&got); err != nil { t.Fatal(err) }
    if got.ExitCode != 2 || got.Error == nil || got.Error.Code != "CONFIG_INVALID" {
        t.Fatalf("unexpected envelope: %#v", got)
    }
    var extra any
    if err := dec.Decode(&extra); err != io.EOF {
        t.Fatalf("expected one document, got trailing decode: %v", err)
    }
}
```

Tests อื่นใช้ assertions ในแต่ละ task เป็น table cases ไม่ต้องสร้าง per-function suite ซ้ำสำหรับสิ่งที่ behavior เดียวพิสูจน์แล้ว

### 3.4 Result invariants

| Condition | Job/report | Exit |
|---|---|---:|
| Eligible selected jobs รันและมี outcome ครบ | selectedCoverage complete | 0 |
| Engine launched แล้ว nonzero หรือ timeout | failed, provenance engine_exit; TIMEOUT ตามกรณี | 1 |
| มี blocked/unsupported skip/unknown/no eligible jobs | selectedCoverage partial | 2 |
| Failed + blocked | failed คงอยู่, selectedCoverage partial | 1 |
| User interrupt | CANCELLED; remaining jobs ไม่ถูกนับ passed | 130 |
| Forced integration steps ผ่าน | selectedCoverage complete ได้ แต่ workflowCoverage partial และมี adjustments | 0 |
| Cleanup ไม่สมบูรณ์หลัง validation ผ่าน | คง job outcome, warning CLEANUP_INCOMPLETE, run exit 2 | 2 |

การเลือกเฉพาะ lint/test สามารถ selectedCoverage complete แต่ workflowCoverage partial เพราะ workflow ยังมี changes/integration งาน excluded ไม่ทำให้ selectedCoverage partial โดยตัวมันเอง

---

## 4. ลำดับงานและ release gates

```text
1 Runtime contract → 2 Go feasibility
→ 3 CLI/config → 4 workflow inventory → 5 capability planning
→ 6 source snapshot → 7 engine/lifecycle → 8 outcome/report
→ 9 coordinator → 10 integration adapter → 11 acceptance → 12 packaging
```

**Gate A หลัง Task 1:** isolated React/success/failure/cancel ต้องมีหลักฐานพอออกแบบ engine ได้ หาก isolation ทำ core validation ใช้ไม่ได้ ให้แก้ feasibility ก่อนเขียน wrapper

**Gate B หลัง Task 2:** ระบุ service integration proven/blocked; integration blocked ไม่ขวาง React MVP แต่ต้องแสดงใน CLI/docs/report

**Gate C หลัง Task 9:** disposable fixture ใช้ CLI จริงได้ครบ exit 0/1/2/130 และ checkout เดิมไม่เปลี่ยน

**Gate D หลัง Task 11:** real React, source/cancel/isolation acceptance ผ่านก่อนเรียก MVP ready ไม่บังคับว่า service validation ต้องผ่านหากเป็น actual source defect แต่ต้องรายงาน failure ถูกต้อง

## Task 1: พิสูจน์ act JSON, isolation และ cancellation contract

**Files:** Create `docs/evidence/engine-contract.md`, `testdata/engine/{success,failure,cancel}.jsonl`, matching `.stderr`, ignored `.pitstop-work/` fixtures
**Interfaces:** Produces measured fields/outcome evidence, effective isolation recipe, cache settings และ owned-resource label behavior ให้ Tasks 7–8

- [ ] ตรวจ `rtk proxy act --version`, `rtk proxy docker version`, `rtk proxy go version`; บันทึก version/platform โดยไม่ dump environment หรือ token-bearing remotes
- [ ] อ่าน `rtk proxy act --help` และ pinned engine config behavior เพื่อยืนยัน flags ที่ใช้จริง; resolve current Docker endpoint ก่อนเปลี่ยน HOME ตรวจ DOCKER_HOST/DOCKER_CONTEXT/context inspect ตาม Docker precedence
- [ ] สร้าง disposable Git fixture มี workflow pull_request และ independent jobs success/failure/cancel; failure ใช้ `exit 7`, cancel ใช้ long-running process fixtures ไม่มี credentials
- [ ] รัน success ด้วย `--json` และ selected job; เก็บ stdout/stderr แบบ raw, argv, act exit, timing และ selected-job fields Expect exit 0 และ evidence ของ job จริง
- [ ] รัน failure แบบเดียวกัน Expect act nonzero; บันทึก actual exit ไม่ assume act คืน 7 ตาม shell step
- [ ] ทดลอง neutral cwd + isolated HOME/XDG/macOS config path + explicit cache directories และ `-C <snapshot>` กับ hostile actrc ทั้ง home, XDG, macOS, invocation, snapshot root; actrc ใส่ dummy secret และ extra platform/job settings
- [ ] Assert ผ่าน fixture ที่อ่าน dummy env ว่า host/token/dummy secret ไม่เข้ารัน และ extra jobs ไม่ถูก launch; originals actrc ไม่เปลี่ยน หาก `-C` ทำ act โหลด snapshot actrc ให้หา tested invocation isolation ก่อนผ่าน Gate A ไม่พึ่ง scalar CLI override สำหรับ array flags
- [ ] ตรวจ resolved Docker endpoint ยังใช้งานภายใต้ isolated HOME; clear DOCKER_CONTEXT ใน child, pass explicit DOCKER_HOST; registry configuration เฉพาะที่จำเป็น ไม่บันทึก credentials
- [ ] ติด unique label ผ่าน `--container-options`; inspect exact labeled IDs ระหว่างรัน cancel fixture ส่ง interrupt, รอ 5s แล้ว kill ตามจำเป็น; เก็บ before/after resources และผล cleanup
- [ ] รัน React verify ใน independent clone โดย isolated settings และ persistent Pitstop cache paths; workflow unchanged Expect job complete หรือบันทึก root cause ที่ยัง block
- [ ] เขียน contract table: structured fields, observed lifecycle selection, unknown/malformed behavior, act exit semantics, effective actrc isolation, Docker endpoint, labels, cancellation residuals
- [ ] Sanitise captured fixtures เฉพาะ credentials/irrelevant absolute paths โดยเก็บ field shape และ semantic messages; raw local evidence แยกจาก committed fixtures ตรวจว่าไม่มี secrets ก่อน commit
- [ ] Verify fixtures parse เป็น JSONL เท่าที่ engine ส่งจริง; หาก stdout มี non-JSON เก็บไว้ ไม่ลบทิ้งเพื่อทำ fixture ดูดี Commit `test: record act execution and isolation contract`

**Done:** core isolated execution ทำงาน, selection evidence ระบุได้, owned runner cleanup พิสูจน์จริง; หากไม่ผ่าน ยังไม่เริ่ม Task 7 ด้วย assumptions

## Task 2: ทดลอง Go CI และกำหนด integration runtime support

**Files:** Create `docs/evidence/service-runtime.md`, `docs/evidence/paths-filter-trial.md`; logs/results ใต้ `docs/evidence/service-probe/`
**Interfaces:** Consumes Task 1 environment recipe; produces integration runtime decision และ prerequisite list โดยไม่เปลี่ยน application commands

- [ ] อ่าน current service ci.yml, local composite setup-go และ pgtest instructions/source เฉพาะที่จำเป็น; record HEAD/workflow hash ใหม่ ไม่ assume revision เดิมยังอยู่
- [ ] สร้าง independent clone; ทดลอง lint และ test ทีละ act invocation จาก original workflow บันทึก raw logs/exit/time แยก validation/environment/unknown ตาม evidence
- [ ] ทำ temporary YAML copy: ตรวจ integration needs/if/command แล้วลบเฉพาะ job-level needs/if; เก็บ source/prepared hashes และ adjustment diff
- [ ] ทดลอง integration โดยเปิดเฉพาะ resolved local Docker socket ที่จำเป็น ตรวจ Testcontainers mapped ports, PostgreSQL connectivity และ residual resources หลังรัน
- [ ] หาก runtime ใช้ไม่ได้ บันทึก INTEGRATION_RUNTIME_UNSUPPORTED พร้อม log reference; ห้ามแทนด้วย host-native go test และห้ามแก้ service source
- [ ] ทดลอง paths-filter ไม่มี token ด้วย actual local event payload/base SHA ใน clone; case committed persistence change, unrelated change, dirty tracked file, untracked persistence file; บันทึก actual outputs ไม่อ้างว่าเหมือน GitHub merge-ref
- [ ] ทำ support table: React proven, service lint/test measured outcome, integration proven/blocked, paths-filter observations; native conditional execution ยัง deferred
- [ ] Commit `docs: record service and integration runtime feasibility`

**Done:** มีการตัดสินใจ integration ชัดเจน หาก unsupported ให้ Task 10 block แบบชัดเจน; หาก proven ให้ระบุ settings สำหรับการรันจริงบนเครื่องที่ทดสอบ

## Task 3: ตั้ง CLI, shared model และ strict config

**Files:** Create `go.mod`, `go.sum`, `cmd/pitstop/main.go`, `internal/{cli/root,model/types,config/config,report/report}.go`, adjacent tests, `.gitignore`
**Interfaces:** Produces config.ResolveRepo/Load/ResolveWorkflow, model contracts, cli.Execute, report.WriteEnvelope

- [ ] เมื่อเริ่ม implementation ตรวจ workspace instructions/status; initialize local Git หากยังไม่มี ใช้ temporary module `pitstop` จนมี actual import path; ไม่ invent GitHub owner/license
- [ ] สร้าง go.mod toolchain baseline และ resolve/pin Cobra/YAML v3 dependencies; ignore `.pitstop-work/` และ local build outputs
- [ ] เขียน table-driven config tests: unknown fields/schema, trailing JSON document, missing fields, duplicate/empty jobs, unsupported arch/event, workflow extension, absolute/traversal path, escaping symlink
- [ ] เขียน repo fixture test nested cwd + spaces Expect canonical Git top-level; outside Git Expect typed error ไม่มีการ reset/add original repo
- [ ] Run `rtk go test ./internal/config ./internal/cli ./internal/report`; ยืนยัน fail เพราะ missing behavior ไม่ใช่ test syntax ผิด
- [ ] Implement strict decoder `DisallowUnknownFields` + EOF check; validate runner fields และ workflow containment ผ่าน resolved symlinks; `--job` validation จะใช้ configured selection ใน Task 5
- [ ] Implement commands/flags ตาม spec; doctor/list/run ที่ยังไม่เชื่อม return typed NOT_IMPLEMENTED exit 2 ห้าม stub exit 0
- [ ] ใช้ Cobra SilenceUsage/SilenceErrors และ centralized output; parse errors ใน `--json` ต้องได้ envelope เดียว แม้ `--json` ตามหลัง invalid flag ใช้ minimal argv scan เคารพ `--` สำหรับกำหนด error output mode
- [ ] Add tests positive timeout 20m default/override, `0`, negative, malformed duration; help text, repeatable jobs; version --json ไม่มี banner
- [ ] Test one-document error ด้วย json.Decoder.Decode ครั้งแรกสำเร็จ ครั้งที่สอง io.EOF; exitCode=2 และ error.code มีค่า
- [ ] Run focused tests และ `rtk go build ./cmd/pitstop`; Commit `feat: add CLI and strict configuration`

**Done:** CLI boundary และ config ใช้งานได้ ไม่มี misleading run success

## Task 4: อ่าน workflow inventory และตรวจ local composite references

**Files:** Create `internal/workflow/inspect.go`, `inspect_test.go`, `testdata/workflows/` fixtures
**Interfaces:** Produces workflow.Inspect และ model.Inventory; ไม่มี act invocation

- [ ] เขียน YAML fixtures React/service shape, string/list needs, missing dependency, cycle, dynamic needs/runner, reusable workflow, matrix, containers/services, local composite secret/cycle
- [ ] Tests assert job IDs/name/dependencies ตาม YAML จริง; YAML parse/missing jobs errors ต้องมี stable reason; duplicate YAML job IDs ไม่ silently overwrite
- [ ] Run `rtk go test ./internal/workflow`; confirm red
- [ ] Implement YAML node inspection; เก็บ source bytes untouched, preserve mapping order สำหรับ inventory; static needs graph ตรวจ missing/cycles และ dynamic forms เป็น unsupported
- [ ] Resolve local action paths ใน repo; อ่าน action.yml/action.yaml สำหรับ composite references และ nested local actions; symlink containment + visited/active sets กัน cycle
- [ ] Scan selected-job YAML/local composite nodes สำหรับ secrets references ตาม supported static form รวม expressions ใน run/with/env/if; remote internals ไม่ assume safe จาก scan นี้
- [ ] Include top-level workflow env ที่ inherited เข้า selected jobs ใน scan; unselected-job secret ไม่ block unrelated selected jobs
- [ ] Run focused fixtures; assert source hash ก่อน/หลัง inspect เท่ากัน Commit `feat: inspect workflows and local actions`

**Done:** list มี inventory ของ original jobs/dependencies และ capability reasons โดยไม่ execute

## Task 5: Capability planning และ list output

**Files:** Modify `internal/workflow/inspect.go`, `internal/cli/root.go`, `internal/model/types.go`; tests `plan_test.go`, `root_test.go`
**Interfaces:** Produces workflow.Plan; consumes config/Inventory; emits PlannedJobs และ ListResult

- [ ] TestReactVerifyEligible: verify selected eligible; TestServiceConditionalIntegrationBlocked: lint/test eligible, integration blocked; changes excluded
- [ ] TestNeedsNeverBroadensSelection: selecting integration ไม่ launch changes; TestUnknownJobRejected: unknown/out-of-config --job exit 2
- [ ] Cases block dynamic runner/needs, job if, dependency, matrix/include, reusable workflows, continue-on-error, job container/services, configured runner mismatch และ secret requirements
- [ ] Cases missing config: list ยัง inventory `.github/workflows/ci.yml` default ได้; ถ้าไม่มี default workflow คืน actionable error ไม่เดาสุ่มจาก deploy workflows ไม่มี selection จนมี config
- [ ] Run `rtk go test ./internal/workflow ./internal/cli`; confirm red
- [ ] Implement eligible subset แคบตาม spec ไม่มี expression interpreter; validate pull_request applicability ของ workflow และ expected runner ไม่ assume job steps executed จาก inventory อย่างเดียว
- [ ] Render selected/excluded กับ eligible/blocked แยกกัน; JSON ใส่ reasons; `--force-integration` without matching adapter/selected integration invalid
- [ ] For service profile ยัง block integration ตาม default; Task 10 จะเตรียม exception เท่านั้น เมื่อแสดง list ไม่ต้องทำ prepared YAML หรือรัน Docker
- [ ] Run fixtures และ `pitstop list --json` ผ่าน cli.Execute tests; Commit `feat: plan supported local jobs and list coverage`

**Done:** ไม่มี preflight unsupported/no-op paths ที่ planner ปล่อยให้ engine exit 0 แล้วเรียก success

## Task 6: Capture working-tree snapshot และ source identity

**Files:** Create `internal/snapshot/snapshot.go`, `snapshot_test.go`
**Interfaces:** Produces Capture/Remove; consumes repo/runDir; returns Snapshot/SourceInfo/manifest path

- [ ] Disposable repo tests: dirty tracked, staged A vs working B, deleted tracked, nonignored untracked, ignored env/build output, tracked env, executable mode, internal/external/broken symlink, spaced/newline filenames
- [ ] Assert working B wins; deleted file absent; untracked source present; ignored/conventional untracked env absent แต่ excludedPaths มีชื่อ; tracked input preserved ภายใต้ symlink boundary
- [ ] Cases unresolved merges, submodules, source changes during copy return explicit error; original HEAD/index/status/content unchanged
- [ ] Run `rtk go test ./internal/snapshot`; confirm red
- [ ] Collect HEAD/status + `git ls-files -z` tracked/nonignored untracked inventories; handle NUL parsing ไม่ line split ใช้ lstat ไม่ follow symlink โดยไม่ตรวจ
- [ ] Precompute source manifest สำหรับ selected inventory: relative path, file kind/mode, symlink target, streamed SHA-256 contents; directory entries ไม่รวม Git internals; conventional untracked exclusions ตาม spec ทุก directory
- [ ] Clone locally `--no-hardlinks` โดย os/exec argv; reconcile tracked deletions และ overlay working bytes/modes/symlinks/untracked; ignored artifacts ไม่ copy
- [ ] Validate resolved symlink targets รวม chain ให้อยู่ repo; unsupported broken link ให้ explicit preflight error ไม่เงียบข้าม source
- [ ] Collect after inventory/manifest/status; compare before/after และ compare captured manifest กับ source manifest จึงรับ capture ป้องกัน mixed snapshot; mismatch SNAPSHOT_CHANGED และ cleanup partial clone
- [ ] Manifest digest deterministic: sorted paths + length-prefixed encoded fields ป้องกัน ambiguous concatenation; ไม่ hash absolute paths/mtime; same source in different temp directories ได้ digest เดียว
- [ ] Test deterministic digest เปลี่ยนเมื่อ bytes/mode/link target เปลี่ยน; retain manifest 0600 หลัง Remove; snapshot removed
- [ ] Real act fixture อ่าน uncommitted marker จาก Capture output ด้วย default local-source-copy; source original ยังเหมือนเดิม
- [ ] Run focused tests; Commit `feat: capture isolated working-tree snapshots`

**Done:** actual working-tree input ได้ deterministic identity และมี evidence ถึง runner จริง

## Task 7: Isolated engine execution, raw logging และ lifecycle

**Files:** Create `internal/actengine/{environment,engine,process_unix}.go`, adjacent tests; use Task 1 fixtures
**Interfaces:** Produces Check/Run/Cleanup; consumes Snapshot/Plan/Environment; Run nonzero process outcome เป็น failed JobResult ไม่ใช่ launch error

- [ ] Check tests fake act versions 0.2.89 accepted, mismatch rejected, missing Git/act/Docker, unavailable daemon และ explicit endpoint under isolated HOME
- [ ] Arg/env tests shell metacharacters/spaces อยู่เป็น argv values; inherited cloud tokens/DOCKER_CONTEXT absent; config lookup hostile fixtures ตาม Task 1 recipe
- [ ] Run `rtk go test ./internal/actengine`; confirm red
- [ ] Implement Check resolve executable paths/act version/Docker endpoint; use Task 1 measured precedence; return EngineInfo actual version/platform/image digest เมื่อหาได้ มิฉะนั้น null
- [ ] Build explicit act argv: pull_request, `-C`, `-W`, `-j`, runner `-P`, arch, four empty input files, `--rm`, offline/pull=false, JSON, explicit caches และ verified unique runner label; normal React socket mounting disabled
- [ ] Create neutral launcher cwd, ephemeral HOME/config directories และ allowlisted environment PATH/locale/TMPDIR/necessary Docker settings; never serialize values that may contain credentials
- [ ] stdout/stderr concurrently drained to separate 0600 raw files; tee progress ไป provided writer; parser maximum line 1 MiB เก็บ larger/malformed lines raw แล้ว mark detail unavailable ไม่ allocate ตาม unlimited line length
- [ ] Fake subprocess tests fill both streams, malformed JSONL, >64 KiB และ >1 MiB lines; assert no deadlock/full raw bytes preserved; no fabricated step timings
- [ ] Process group on Unix: cancellation/timeout send graceful signal, wait up to 5s, kill group; reap process once; distinguish parent user cancellation กับ per-job deadline
- [ ] Owned runner labels/resources persist IDs in metadata; cleanup ใช้ fresh bounded context, exact labeled IDs ที่ยืนยัน ownership; never prune/broad names; record failures/residuals
- [ ] Test timeout child/grandchild terminated, repeated cleanup harmless, unrelated fake resources untouched; run real labeled cancel fixture และ verify existing service containers still running
- [ ] Run focused tests; Commit `feat: run act with isolated environment and cleanup`

**Done:** engine ไม่รั่ว host secrets, streams ไม่ค้าง และ cancel ไม่ทิ้ง owned runner โดยไม่มี report

## Task 8: Outcome evidence, JSON envelope และ exit aggregation

**Files:** Modify `internal/actengine/engine.go`, `internal/report/report.go`, `internal/model/types.go`; tests `outcome_test.go`, `report_test.go`
**Interfaces:** Produces job outcome validation, report.ExitCode/Human/WriteEnvelope; consumes measured Task 1 records

- [ ] Fixture tests success/intentional failure/cancel; engine zero + missing selection evidence → ENGINE_OUTCOME_UNKNOWN exit 2; selected job not observed/no-op → partial ไม่ passed
- [ ] Exit table tests 0/1/2/130 รวม failed+blocked, skipped unsupported, empty results, cancel+failure และ cleanup incomplete
- [ ] Assert unknown step status/timing remain null, empty steps/adjustments/warnings serialize []; structured record routing คง job/step/stage identities
- [ ] Run `rtk go test ./internal/actengine ./internal/report`; confirm red
- [ ] Implement supplementary selection validation จาก version-specific measured log shape; process exit authoritative เฉพาะ preflight-approved selection ที่ยืนยัน intended job ได้ English success string อย่างเดียวไม่พอ
- [ ] Failure category default unknown; validation/environment เฉพาะ evidence ที่อธิบายได้ ห้าม blanket classify nonzero เป็น code failure
- [ ] Human report แสดง selected scope, job outcomes/reasons, partial coverage, duration, logs/report paths; JSON rendering จุดเดียวและ newline ท้ายเอกสาร
- [ ] Coverage complete/partial ตาม original jobs และ adjustments ไม่ยกระดับ forced workflow fidelity เป็น complete
- [ ] Run focused tests; Commit `feat: report evidence-backed outcomes and exit codes`

**Done:** report บอกสิ่งที่ตรวจจริงและไม่ทำ unknown/blocked เป็น green

## Task 9: Run coordinator, lock, artifacts และ doctor

**Files:** Create `internal/run/run.go`, `run_test.go`; modify CLI/main/report/models
**Interfaces:** Produces run.Execute; consumes modules Tasks 3–8; CLI เป็นผู้เขียน final stdout เพียงจุดเดียว

- [ ] Test blocked integration does not prevent lint/test; failed eligible job does not stop next independent job; cancellation stops scheduling new jobs
- [ ] Test same-repo lock immediately returns RUN_ALREADY_ACTIVE/2; different repos independent; process exit releases advisory lock ไม่ใช้ stale lockfile existence เป็น lock
- [ ] Test JSON success/preflight/launch error/timeout/cancel each exactly one document; persisted report equals stdout envelope; JSON progress only stderr
- [ ] Run `rtk go test ./internal/run ./internal/cli ./internal/report`; confirm red
- [ ] Resolve canonical repo/config/options และ acquire nonblocking advisory flock by hashed canonical repo path ก่อน capture; root caches/run IDs unique ใช้ standard library + Unix syscall ไม่เพิ่ม lock framework
- [ ] Create run directory 0700; planned blocked results ยังถูกเก็บ; ถ้าไม่มี eligible jobs ไม่ launch act สร้าง actionable preflight envelope
- [ ] Sequence: inspect/plan → Check → capture → prepared workflow if needed → execute independent selected jobs → cleanup/snapshot removal → finalize coverage/exits → persist envelope
- [ ] สร้าง minimal local event JSON artifact และ pass event path ตาม contract ที่ Task 1 พิสูจน์; ไม่ fabricate PR number/token/merge-ref หาก base SHA จำเป็นให้ใช้ Git evidence จริงและบันทึกข้อแตกต่างจาก GitHub
- [ ] Install defers ตั้งแต่ resource ถูกสร้าง; retain metadata/manifests/event/workflow copies/raw logs; delete ephemeral HOME/source snapshot หลังจบ; all owned files 0600
- [ ] Persist report atomically ใน run directory; set reportPath ก่อน marshal; stdout/persisted report ใช้ envelope เดียว หาก persistence failure คืน error exit 2/stronger existing exit พร้อมเหตุผล ไม่อ้างว่ามีไฟล์สำเร็จ
- [ ] Wire signal context จาก main; cancellation during capture/launch/run ยัง emit final envelope เมื่อ process รับ signal ได้; ไม่อ้างว่ารองรับ SIGKILL final output
- [ ] Doctor tool/config/capability readiness: ไม่มี validation process แต่ Docker/tool inspection ทำได้; list remains inventory-only ไม่ต้อง daemon พร้อม
- [ ] Fake engine executable end-to-end tests และ `rtk go test ./...`; Commit `feat: coordinate runs and persist command reports`

**Done:** Gate C ผ่าน actual CLI fixture flow และ every final path มี predictable output/cleanup

## Task 10: Guarded integration adapter และ profiles

**Files:** Create `internal/workflow/integration.go`, `integration_test.go`, `examples/recruit100-{react,service}.json`; modify planner/coordinator/engine as necessary
**Interfaces:** Produces PrepareIntegration adjustments + prepared YAML; consumes Task 2 runtime verdict

- [ ] Tests integration expected `needs: changes`, expected persistence condition และ integration go command; drift condition/needs/command fails ADAPTER_SHAPE_CHANGED
- [ ] Tests output removes only integration job-level needs/if; semantic subtree comparison ยืนยัน all steps/other jobs unchanged; original byte hash unchanged
- [ ] Test --force-integration without adapter/selection invalid; default integration blocked; forced unsupported runtime blocked; passed forced result workflowCoverage partial
- [ ] Run `rtk go test ./internal/workflow ./internal/run`; confirm red
- [ ] Implement node-based guard + edit; record precise yamlPath/operation/before/after; store original/prepared hash in metadata; formatting change จาก YAML encoding เปิดเผยได้แต่ห้ามเปลี่ยน step semantics
- [ ] Adapter enablement เฉพาะ config `recruit100-service-integration-v1` + force flag + selected integration; do not add changes dependency to selection
- [ ] If Task 2 unsupported, block force with INTEGRATION_RUNTIME_UNSUPPORTED จน runtime implementation/real acceptance ผ่าน ไม่เพิ่ม generic runtime registry/config flag เพียงเพื่อข้าม gate
- [ ] If proven, use Task 2 local socket/network recipe; new host/arch ยัง require actual engine trial before support claim; Docker connection readiness errors ไม่ใช่ validation defect
- [ ] Copy exact JSON examples จาก design; default service includes lint/test/integration และแสดง partial หาก integration blocked; explicit lint/test selection documented
- [ ] Run focused tests และ actual integration only on proven settings; Commit `feat: add guarded service integration and profiles`

**Done:** integration steps รันได้เมื่อพิสูจน์แล้ว หรือ blocked พร้อมเหตุผล; original workflow fidelity ไม่ถูกปิดบัง

## Task 11: Acceptance checks และ measured support matrix

**Files:** Create `scripts/smoke.sh`, `docs/evidence/final-validation.md`, acceptance logs/results
**Interfaces:** Consumes completed CLI; produces Gate D evidence and release readiness decision

- [ ] Run `rtk go test ./...`, `rtk go test -race ./...`, `rtk go vet ./...`; fix material failures ก่อน real acceptance
- [ ] Smoke binary version/help/doctor/list, missing config/unknown flags under --json; assert exits และ exactly one JSON document
- [ ] Real React run ผ่าน snapshot จาก original repo input โดย config example ส่งผ่าน disposable test setup ถ้ายังไม่มี authorized application config edit; never insert config/product files ใน application repo โดยอัตโนมัติ
- [ ] เพื่อทดสอบ original repo input โดยไม่แก้ config: CLI ต้องมี .pitstop.json ใน repo ตาม spec หากยังไม่มี config ให้ใช้ independent local clone พร้อม config และระบุ input limitation หรือขอ scope สำหรับเพิ่ม config แยกจากการเขียน code ห้ามเพิ่ม unrequested --config flag
- [ ] Repeat warm run ด้วย caches เดิม; record first observed/warm times และ cache state ไม่ลบ user cachesเพื่อสร้าง artificial cold run; separate wrapper setup/capture time กับ job invocation duration
- [ ] Real service lint/test workflow unchanged; record code/setup failures และ logs; no unrelated source fixes
- [ ] Disposable source fixture dirty/staged-vs-working/deleted/untracked/spaces plus marker inside runner; compare original bytes/status/index before/after
- [ ] Hostile actrc + host token dummy acceptance; dummy valuesไม่เผยแพร่เป็น real credentials; verify no extra jobs/input injection และ originals untouched
- [ ] Intentional failed fixture exit 1, blocked fixture exit 2, timeout exit 1/TIMEOUT, SIGINT exit 130; real cleanup exact owned runners gone, unrelated PostgreSQL/MinIO remain
- [ ] Verify reports permissions, raw long logs, report/stdout equality, selected/workflow coverage และ forced integration adjustments
- [ ] Support matrix columns capability/platform/status/proof/log path/limitation; distinguish tested, blocked, compiled-only และ untested
- [ ] Review acceptance tableท้ายแผน; material gaps mark not ready ไม่เปลี่ยน requirements เพื่อทำทุกช่องเขียว Commit `test: verify local CI acceptance and lifecycle`

**Done:** มี reproducible evidence ของทุก core requirement; unsupported integration allowed only visibly blocked

## Task 12: Packaging, README และ agent handoff

**Files:** Create root `README.md`; update examples/final-validation; build output ignored
**Interfaces:** Produces installable local binary และ user/agent documentation

- [ ] Build `rtk go build -trimpath -o .pitstop-work/bin/pitstop ./cmd/pitstop`; version build metadata ผ่าน ldflags เฉพาะเมื่อ values จริงมีอยู่
- [ ] Optional Linux AMD64 cross-build ด้วย GOOS/GOARCH ผ่าน `rtk proxy env`; label compiled-only หากไม่มี real acceptance บนแพลตฟอร์มนั้น
- [ ] README prerequisites Go build/act 0.2.89/Docker/Git; binary usage ไม่ต้อง Go runtime; ไม่มี silent dependency install
- [ ] Document config placement/examples, nested paths, commands/jobs/timeout/arch/force integration, cache/run storage และ manual deletion ของ finished run directories
- [ ] Document exit 0/1/2/130, JSON envelope example, null semantics, blocked/partial/adjustments และ engine/source identity
- [ ] Agent recipe: doctor → list → run --json → inspect reasons/logs → fix source within authorized scope → rerun; ห้ามเพิ่ม secrets/แก้ CI conditions/deploy เพื่อทำ green
- [ ] Link original/new evidence และ measured timings; no GitHub speed comparison หรือ claim GitHub equivalence
- [ ] Final `rtk go test ./...` และ binary smoke หลัง documentation/build wiring ที่เกี่ยวข้อง; Commit `docs: document verified Pitstop usage`

**Done:** binary + docs พร้อมใช้ตาม supported subset และ limitation ตรงกับหลักฐาน

---

## 5. Acceptance traceability

| Spec requirement | Tasks | Evidence required |
|---|---|---|
| CLI/config/errors/version | 3, 5, 9, 12 | command/strict decoder tests, binary smoke |
| Workflow subset/dependencies/credentials | 4, 5 | blocked fixture suite, no hidden needs execution |
| Dirty/untracked/deleted/staged source | 6, 11 | Git fixtures, runner marker, originals unchanged |
| Symlink/path containment/source races | 3, 4, 6 | escaping path tests, deterministic manifest, changed-source rejection |
| actrc/env isolation/Docker connection | 1, 7, 11 | hostile settings trial + fake process assertions |
| Logs/selection/outcome/JSON | 1, 7, 8, 9 | actual fixtures, huge/malformed streams, one envelope |
| Timeout/cancel/resource ownership | 1, 7, 9, 11 | real interrupt, exact IDs, unrelated containers preserved |
| Service CI/integration/paths-filter | 2, 10, 11 | raw trials, guarded edits, proven/blocked verdict |
| Lock/private storage/retention | 9, 11 | concurrent lock tests, permissions, artifact equality |
| Existing React workflow passes | 1, 11 | original/prepared hash, recorded outcome/logs |
| Platform/support/speed claims | 11, 12 | actual support matrix, timing cache context |

## 6. Test approach และขอบเขตที่เลื่อนไป

ใช้ Go built-in testing, table-driven tests ที่จัดตาม behavior และ subprocess boundary ไม่เพิ่ม test frameworks สร้าง helper เท่าที่ fixture tests ใช้ร่วมจริง Tests ที่รัน network/Docker ต้อง opt-in และไม่เป็น default `go test ./...`

เลื่อน GUI/TUI/MCP, generic expression/matrix/reusable workflow support, host-native backend, Windows, auto-fix/PR/check publishing, deploy/secret retrieval และ cache tuning จนมี requirement ใหม่ Native paths-filter support ยังเลื่อนแม้ probe ผ่าน เพราะ MVP ใช้ explicit forced adapter

## 7. Self-review และ execution handoff

- [x] Spec ทั้ง 15 sections มี task หรือ explicit deferred boundary รองรับ
- [x] Signatures/callers consistent: Environment resolve ก่อน isolation, ExecutionWorkflowPath สำหรับ adapter, Cleanup รับ runDir ownership evidence
- [x] Review Focus ทั้ง 5 จุดมี unit/real tests ใน owning tasks
- [x] Original measurements ไม่ถูกแปลงเป็น implementation acceptance ใหม่
- [x] No hosted repo/module owner/license/delegation ถูกเลือกแทนผู้ใช้
- [x] Real original-input acceptance ระบุ config authorization constraint โดยไม่แก้ application repo อัตโนมัติ

แผนพร้อมให้ review ก่อนเริ่ม implementation แนวทาง execution ที่คงไว้คือ native sequential เริ่ม Tasks 1–2 เพื่อปิด feasibility gaps แล้วสร้าง CLI ทีละ task ตาม gates; รอบการเขียนแผนนี้ไม่มีการรัน CI ใหม่หรือเขียน product code
