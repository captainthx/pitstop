> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../README.th.md) สำหรับการใช้งาน Docker checks.

# Pitstop — Context and Execution Handoff

Date: 2026-10-05, Asia/Bangkok. Read this first, then pitstop-spec.md and pitstop-implementation-plan.md.

## Quick context in Thai

ผู้ใช้ต้องการรัน validation ก่อนเปิด PR บนเครื่องตัวเอง เพราะรอ GitHub CI ช้า ต้องใช้ไฟล์ workflow เดิม ไม่แยกเขียนคำสั่ง lint/test/build อีกชุด เลือกรันเฉพาะงานที่เหมาะกับ local และรายงานงานที่รันไม่ได้ให้ชัด เครื่องมือต้องเรียกง่ายทั้งคนและ AI agent

ชื่อที่ผู้ใช้เลือกคือ **Pitstop** ส่วน CLI แนะนำให้เขียน **Go** ครอบ **act + Docker** เราลอง React CI จริงแล้วผ่านครบ: รอบแรก 111.08 วินาที และรอบมี cache 45.86 วินาที โดยไม่ใช้ secret ไม่แก้ YAML และไม่เปลี่ยน checkout ต้นฉบับ

ยังไม่มี Pitstop product code และยังไม่ได้ทดสอบ Go CI ตอนนี้เป็นชุดสเปก/แผนสำหรับนำไป implement โปรดอ่านส่วน proven/unproven ก่อนอ้างว่ารองรับความสามารถใด

## 1. What the user asked and decided

1. Wanted local CI validation before a PR, reducing push/wait/fix loops.
2. Explicitly chose existing `.github/workflows` as input rather than shared scripts outside YAML.
3. Supplied two repos as real examples; acknowledged that some workflows require secrets and should not run locally.
4. Wants a simple CLI usable manually and by an AI agent.
5. Accepted the wrapper direction for MVP 1 and selected the name Pitstop.
6. Asked about language/act tradeoffs. Go was recommended for standalone CLI distribution and subprocess/file/report handling, not as a claim that Go accelerates builds.
7. Explicitly asked to try act against React CI before building the wrapper. That experiment succeeded.
8. Current request: detailed spec, implementation plan and good context for another implementation session. It does not ask to implement/deploy now.

## 2. Current state and authorization

- No new Pitstop Git repo or hosted repository exists.
- No product code was written; these are planning artifacts.
- Neither recruit100 application repo was modified during the experiment.
- act 0.2.89 was installed through Homebrew, as necessary to run the authorized trial.
- Runner image/actions/Node/pnpm caches remain available on the user's machine.
- The local probe clone lives at `/Users/thanyatab/Documents/Codex/2026-10-05/new-chat/work/act-react-probe`.
- User has not authorized delegation or creating a separate Codex chat. Follow native execution unless subsequently authorized.
- Select/confirm the new Pitstop repo location before creating product files. Existing absolute paths are this user's examples, not hardcoded product defaults.

## 3. Environment

- macOS ARM64, Docker reports linux/aarch64.
- `/opt/homebrew/bin/brew`; installed act via `rtk proxy brew install act`.
- Docker CLI observed at `/usr/local/bin/docker`; CLI successfully reaches the daemon.
- Host Node 24 exists, but the workflow installs Node 20 inside its runner. Host Node is irrelevant to the proven validation toolchain.
- Two preexisting unrelated containers were running: `recruit100-service-postgres-1`, `recruit100-service-minio-1`. Never stop them while cleaning a Pitstop run.
- User's shell is zsh. Prefix shell commands with `rtk` according to `/Users/thanyatab/.codex/RTK.md`; use `rtk proxy` when raw tool output matters.

## 4. Repository inventory

### React

Path: `/Users/thanyatab/Desktop/project/techcombine/recruit100-react`.

Observed HEAD: `c65e803404e1538c96d2162014eb7e03ee942881`; clean during trial and after both runs.

Workflows: ci.yml, deploy-beta.yml, deploy-prod.yml, deploy-careers-page-beta.yml, deploy-careers-page-prod.yml.

CI job verify executes pnpm setup, Node 20, frozen install, ATS lint/typecheck/build and Recruit100 lint/typecheck/test/build:worker. No direct secret references. packageManager is pnpm@12.5.1; apps/ats is Vite, apps/recruit100 is Next.js/OpenNext.

Deploy jobs use Firebase service-account/API-key secrets or Cloudflare token/account configuration; they stay outside default validation. App test scripts are not automatically part of CI: do not add extra checks behind the user's back.

Instructions found at root AGENTS.md and apps/recruit100/AGENTS.md. If implementation later changes product files, read applicable instructions again; the present plan should not need application source changes.

### Go service

Path: `/Users/thanyatab/Desktop/project/techcombine/recruit100-service`.

Observed HEAD: `e58f8403f675e49e3e9681ad9492d158622bb0c2`; clean at last read-only inspection.

Workflows: ci.yml, deploy.yml, migrate.yml, seed-skills.yml, backfill-candidate-names.yml, backfill-organization-slugs.yml.

CI jobs lint/test/changes/integration. Go 1.26.5 in go.mod. Local setup-go composite at `.github/actions/setup-go/action.yml` uses setup-go and fallback Go caches. Integration uses PostgreSQL Testcontainers. changes uses dorny/paths-filter@v3 and controls integration via persistence output.

No service act run has happened yet. CI has no direct secret reference, but engine readiness and Docker access remain unproven. GCP deployment/data-maintenance workflows are out of scope.

## 5. Exact experiment and evidence

An independent local clone was used because the original React checkout was clean. This trial proves committed source only, not dirty/untracked snapshot handling.

Cold argv (executed with cwd=probe clone):

```text
act pull_request -W .github/workflows/ci.yml -j verify
-P ubuntu-latest=catthehacker/ubuntu:act-latest
--container-architecture linux/arm64
--env-file /dev/null --secret-file /dev/null
--var-file /dev/null --input-file /dev/null
--container-daemon-socket - --rm
```

Warm run appended `--action-offline-mode --pull=false`. No --bind and no --no-skip-checkout. Act copied the clone source into the runner. Host cloud/token environment variables were omitted from the child process environment; no secret file values were read. No actrc file was found at the checked repo/home paths, but full actrc isolation was not proven in this trial.

Both runs exited 0 and logged Job succeeded. Both passed seven validation steps and all 15 Recruit100 tests. Node 20.20.2/pnpm 12.5.1 were installed inside Linux ARM64. OpenNext produced `.open-next/worker.js`; this did not deploy anything.

Cold 111.08s, warm 45.86s. Warm restored pnpm store (`cache-hit=true`) and used cached image/actions/Node. It still installed packages and built anew; Next.js reported no build cache. Vite warned about bundle size and Node emitted deprecation warnings; none were fatal.

Evidence in this package:

- `act-react-ci-report.md`: human summary and repeatable command.
- `act-react-ci.log`: full cold-run log.
- `act-react-ci-warm.log`: full warm-run log.
- `act-react-ci-result.json`: actual argv, commit, runtime, engine exit.
- `act-react-ci-warm-result.json`: corresponding warm evidence.

## 6. Why a wrapper is worthwhile

Act alone already selects workflows/jobs and supports config/caching. A wrapper is worthwhile for explicit repo profiles, working-tree snapshots, robust preflight, noninteractive JSON/exit contract, partial-coverage reporting, timeout/cancellation ownership and log paths. Do not build a wrapper whose only value is shortening an act command.

Go is chosen for a binary that needs no Node/Python runtime to start. Go does not remove act/Docker requirements and does not make underlying CI scripts faster. The wrapper uses act as a subprocess, not as a Go library, keeping engine upgrades separately testable.

## 7. Known act limitations and design consequences

- Runner containers have different preinstalled tools from GitHub VMs. Record image/platform identity.
- GitHub event/context is incomplete locally. Do not fabricate real PR numbers or assume original merge-ref semantics.
- OIDC and environment-scoped secrets are not equivalent. Cloud deploy jobs remain excluded.
- Some job behaviors such as concurrency/timeouts are not faithfully implemented. Pitstop enforces its own lifecycle limit.
- --json supplies log records rather than a documented completion API. Process exit is authoritative only after supported selection preflight; optional step summaries may be unknown.
- Act reads XDG, home and invocation actrc before explicit CLI arguments; array flags can accumulate. Isolate those paths rather than trusting /dev/null file flags alone.
- Git-diff fallback exists in paths-filter@v3 when there is no token. It might eliminate the need for some overrides, but has not been tested against dirty/new files or required base semantics.
- Containerized Testcontainers needs Docker connectivity plus reachable mapped ports. Test this on the real machine before claiming integration support.

Primary source links are in pitstop-spec.md. The plan pins the tested engine baseline rather than guessing a broad version compatibility range.

## 8. Design decisions to preserve

- YAML is the source of validation commands; config stores selection/settings only.
- Default profiles run approved validation jobs only, not all workflows.
- Generic MVP supports independent unconditional jobs; blocked dependency/conditional work remains visible.
- Service integration is a guarded, explicit optional adapter that removes only job needs/if in a copy and records adjustments.
- Snapshot reflects current working-tree source and leaves user checkout unchanged.
- No implicit secrets/env imports or gh auth token retrieval.
- JSON stdout exactly one envelope; live progress stderr.
- Exit 0 success, 1 launched failure, 2 preflight/blocked/incomplete, 130 cancellation.
- Report selected scope separately from fidelity/coverage of original workflow.
- Retain raw logs and report unknown diagnosis rather than inventing a cause.
- No GUI/MCP/deploy/plugin framework/custom expression interpreter in MVP.

## 9. First implementation actions

1. Read spec and plan; choose the new implementation repo path and native/delegated method with the user.
2. Copy this planning/evidence package into that repo and preserve original measurements.
3. Perform Task 1 runtime probes: JSON success/failure, isolated settings, cancellation labels, Go lint/test, integration.
4. If act cannot support a needed behavior, record the exact limit and implement honest blocking rather than silently falling back to a different engine.
5. Execute tasks in order with meaningful failing tests, minimal implementation, verification and focused commits.

## 10. Suggested prompt for the next agent

> Build Pitstop MVP 1 in a dedicated repository. Read pitstop-context.md, pitstop-spec.md and pitstop-implementation-plan.md first. The name is Pitstop and the stack is Go + Cobra + YAML v3, with act 0.2.89 and Docker as external runtime dependencies. React CI has been proven on macOS ARM64 with Linux ARM64 runner: 111.08s cold, 45.86s warm, both exit 0. Go CI/integration and dirty snapshots still need proof. Start with Task 1 feasibility probes. Preserve original workflow commands and application repositories. Never count blocked/skipped unsupported work as passed. Run natively unless I explicitly authorize subagents. Follow applicable AGENTS.md instructions and prefix shell commands with rtk. Present implementation limits using measured evidence.

## 11. Package reading order

1. This context.
2. `pitstop-spec.md` — requirements/contracts and boundaries.
3. `pitstop-implementation-plan.md` — files, signatures, tasks and checks.
4. `act-react-ci-report.md` and raw logs/results — evidence.

The earlier `pitstop-mvp-spec.md` is superseded and excluded from the handoff archive to avoid conflicting requirements.
