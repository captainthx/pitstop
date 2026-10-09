# Config initialization, validation and agent skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ให้คนและ AI agent สร้าง config ให้ตรงกับโปรเจกต์ได้ พร้อม validator ที่เข้มงวดและ skill ที่ติดตั้งผ่าน CLI ได้โดยไม่ทำข้อมูลเดิมเสียหาย

**Architecture:** ต่อเติม Cobra/config/model เดิม ไม่เปลี่ยน Docker backend ใช้ byte parser และ semantic validator ชุดเดียวทุก consumer รวมถึง generator ฝังชุด skill/references/examples ด้วย embed.FS ให้ CLI ใช้ได้โดยไม่มี source checkout และไม่มีการดึง AI/network

**Tech Stack:** Go 1.26.5, Cobra v1.10.2, stdlib encoding/json, os.Root, embed.FS, testing/fuzz, Git; no new dependencies or experimental build flags.

**Spec:** [Config/agent design](../specs/2026-10-08-config-agent-design.md). Read this and the existing [Docker design](../specs/2026-10-07-docker-checks-design.md) before work. Deferred scope: [roadmap](../../roadmap.md).

**Status:** Tasks 1–8 are implemented and reviewed in this checkout, and the two test-only gaps identified by the scoped review have passing correction-round checks from the user-requested follow-up. Task 7 has 23 evaluator attempts across 14 cases, with preserved raw artifacts and controller validation in the [trial evidence index](../../evidence/config-agent-trials/README.md); selected Node/Go/Python/Rust runtime, focused pnpm/Yarn Berry bootstrap, offline smoke, CI wiring, and final local checks are recorded in [config-agent validation](../../evidence/config-agent-validation.md). The whole-branch review found five Important and two Minor findings; the bounded fix wave is complete and the coordinator follow-up gate is pending independent scoped re-review. The remaining remote CI, release, platform, runtime and security boundaries stay explicit. Baseline `e6f8328fc6e181a1d3a1994c3f1d7c93479a7b3d`. This documentation branch is `feature/config-agent-plan`; reuse it for execution unless the user selects another checkout. Do not switch back to the merged Docker branch. No push/release.

**Completed task summary:** The progress ledger records Tasks 1–3 complete with focused regressions and fuzz evidence, Task 4 complete with canonical asset/parser checks, Task 5 complete with preservation/cleanup evidence, Task 6 complete with transactional installer and trusted-history evidence, and Task 7 complete with 14 cases and 23 evaluator attempts. The ledger preserves where an initial behavioral RED was omitted or was a build failure; this summary does not retroactively claim RED evidence. Task 8 evidence is linked above; the current review, resolutions and reruns are preserved in [permanent review evidence](../../evidence/config-agent-review.md), with the completed scoped gate and explicit residual test limitations.

## Global Constraints

- Go 1.26.5 floor, Cobra v1.10.2, stdlib; no new runtime dependencies or experimental build flags.
- Config schemaVersion remains 2; JSON envelope schemaVersion remains 1.
- Architecture remains linux/arm64 or linux/amd64; IDs remain `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`.
- No host execution of project commands; no bind mounts, Docker socket mounts or privileged Docker options.
- Init, validate and skill install do not contact Docker, registries, AI providers or GitHub.
- All config consumers use the same parser and model validator; invalid input returns no usable Config.
- No implicit overwrite of config/skill; no modification of target AGENTS.md, CLAUDE.md, workflow or project scripts.
- Examples are starters, not guarantees of project compatibility or workflow parity.
- Shell commands in this development session use RTK; commands inside user configs are not prefixed with RTK.
- Native Windows, JS/WASM and cross-architecture execution are not newly claimed as supported.
- Strengthening validation is a compatibility change: document rejected ambiguous inputs; preserve accepted command bytes and existing v2 examples.
- Do not claim an unbreakable validator. Measure rejection, resource bounds, absence of side effects and preserved files; hostile shell/image execution is a separate trust boundary.

## Review Focus

1. Duplicate, escaped duplicate, case-variant, null and malformed Unicode members must not change interpretation between CLI/generator/run — Tasks 1/2/4/8.
2. FIFO, device, symlink, oversized or changed config must not hang, escape the repo or bypass snapshot revalidation — Tasks 1/2/3/8.
3. `--json`, `--`, unknown flags and repo names equal to command names must preserve one envelope and the correct command — Tasks 3/5/6/8.
4. Existing files, symlinked skill ancestors, force failures and concurrent installers must not truncate/delete user data — Tasks 5/6/8.
5. Unknown job names, multi-language repos and source instructions must not lead agents to invent tests, copy secrets or claim CI parity — Tasks 4/7/8.

## File map and dependency order

| Task | Files and responsibility |
| --- | --- |
| 1 | Modify internal/config/config.go and internal/model/types.go (byte/depth constants), create internal/config/parse.go and parse_test.go; extend config_test.go; strict byte/file entry point |
| 2 | Modify internal/model/types.go and types_test.go; modify internal/config/config.go/config_test.go and internal/run/run.go/run_test.go; diagnostics, semantic limits and shared overrides |
| 3 | Create internal/cli/validate.go and validate_test.go; modify root.go/root_test.go; create internal/config/fuzz_test.go; validate CLI and side-effect/fuzz checks |
| 4 | Create internal/agentassets/assets.go/assets_test.go and bundle resources; retain examples/docker-react.json/docker-go.json; canonical starters and mappings |
| 5 | Create internal/config/init.go/init_test.go and internal/cli/init.go/init_test.go; modify root.go; init generation/preservation |
| 6 | Create internal/skill/install.go/install_test.go and internal/cli/skill.go/skill_test.go; modify root.go; bundled installer |
| 7 | Finish bundle/SKILL.md and references; create docs/ai-agent.md and docs/ai-agent.en.md; update README/guide/requirements/index/roadmap; agent workflow and docs |
| 8 | Create docs/evidence/config-agent-validation.md and scripts/config-agent-smoke.sh; extend relevant tests and .github/workflows/ci.yml; final checks and evidence |

Order: 1 → 2 → 3 (validator gate) → 4 → 5 → 6 → 7 → 8. Tasks 1–3 must pass before generation/install. No generic validator framework, YAML runtime/importer, image builder, schema generator or engine refactor. Each task adds its failing check before implementation and ends with a focused commit of that task's files; use `rtk git add <explicit files>` and a descriptive commit message. Never stage unrelated changes.

## Task 1: Confined file reading and strict JSON parsing

**Interfaces**

- Consumes existing `config.Load(repo string) (model.Config, error)` and `model.Config.Validate() error`.
- Produces `config.Parse(data []byte) (model.Config, error)`; preserves Load signature; adds private `readConfig(repo string) ([]byte,error)` and preflight helpers in parse.go.
- Declare shared limits in types.go in this task: MaxConfigBytes=1048576, MaxJSONDepth=16. Task 2 adds semantic limits in the same file.
- All failure paths wrap CONFIG_INVALID and return `model.Config{}`, not partially decoded state. Task 2 attaches structured issues without changing acceptance.

- [ ] **1. Write failing parser tests in parse_test.go** with these exact table inputs/assertions; use current validConfig as the valid seed:

```go
// In TestParseRejectsAmbiguousInput, every row must return an error and zero Config.
cases := []string{
    `null`, `[]`, `{} {}`, validConfig + ` garbage`,
    strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":2,"schemaVersion":1`, 1),
    strings.Replace(validConfig, `"schemaVersion":2`, `"SchemaVersion":2`, 1),
    strings.Replace(validConfig, `"id":"check"`, `"id":"check","\u0069d":"other"`, 1),
    strings.Replace(validConfig, `"run":"true"`, `"name":null,"run":"true"`, 1),
    strings.Replace(validConfig, `"run":"true"`, `"run":"\ud800"`, 1),
    strings.Replace(validConfig, `"schemaVersion":2`, `"schemaVersion":2e0`, 1),
}
```

  Add TestParseStrictShape (unknown/wrong-case/duplicate/null/type/missing members at each nested level, schemaVersion string/fraction/overflow, root scalar, BOM, trailing comma/comments); TestParseUnicode (Thai, valid surrogate pair, literal U+FFFD accepted; raw invalid UTF-8 and lone high/low surrogate rejected; escaped backslash-u is literal, not mistaken for a surrogate); TestParseSizeAndDepth (limit and limit+1 bytes/depth, huge unused objects); TestParsePreservesRun (multiline/tab/CR, quotes, commas and dollar substitutions are byte-identical after decode). Test v1 migration and unsupported versions without accepting duplicate/version aliases.

- [ ] **2. Run focused tests and observe failure**: `rtk go test ./internal/config -count=1`. First build fails because Parse is absent; after declaring Parse, assertions must fail until enforcement exists. Do not treat a build failure alone as validation coverage.

- [ ] **3. Implement Parse with stable stdlib** in parse.go. Bound length first; utf8.Valid plus a narrowly scoped string-escape/surrogate check; Decoder.Token with UseNumber to validate syntax, uniqueness of decoded member names and depth before materializing typed values. Separate generic syntax/duplicate/Unicode pass from version inspection; v1 gets migration after syntax checks, v2 receives expected-shape checks. Use existing model JSON tags/shape, never a JSON Schema runtime. Typed decode uses DisallowUnknownFields and EOF, then Validate. Diagnostics initially trusted fixed messages; never dump input bytes.

- [ ] **4. Add failing TestLoadRejectsSpecialFiles/TestLoadBoundsOpenedFile** in config_test.go. Reject a normal and broken symlink, directory, Unix FIFO with no writer, socket, missing/unreadable file and >1MiB regular file; assert bounded return using a subprocess deadline for FIFO tests, not a goroutine left blocked forever. Add a concurrent symlink replacement regression whose only acceptable outcomes are valid in-root bytes or an error, never outside contents.

- [ ] **5. Implement readConfig** using `os.OpenRoot(repo)`, confined OpenFile of the fixed basename with O_RDONLY|O_NOFOLLOW|O_NONBLOCK and descriptor Stat; reject nonregular descriptor before reading. Read at most MaxConfigBytes+1 with LimitReader; close descriptors on all paths; Load delegates to Parse. Maintain existing Unix support, no new Windows promise. Do not introduce an unrestricted fallback if confined open fails.

- [ ] **6. Verify and commit**: `rtk go test ./internal/config ./internal/model -count=1`; all new and existing tests pass. Review `rtk git diff --check`; commit parser/reader and relevant constants only.

## Task 2: Semantic limits, safe diagnostics and override parity

**Interfaces**

- Consumes Parse/Load and current run.Options.
- Produces `model.ConfigIssue{Code string, Path string, Message string}` with JSON tags code/path/message; `model.ValidationError{Issue ConfigIssue}` implementing Error with a trusted message; wrapped in existing model.Error CONFIG_INVALID.
- Produces `config.ApplyOverrides(c model.Config, jobs []string, architecture string) (model.Config,error)`; preserve c when inputs omitted, reject duplicates/outside default jobs, copy the selection slice and Validate effective config.
- Add constants MaxJobs=128, MaxChecks=128, MaxStepsPerCheck=128, MaxTotalSteps=2048, MaxRunBytes=65536, MaxNameBytes=256, MaxImageBytes=1024, MaxIssueMessageBytes=512, MaxIssuePathBytes=256. Use existing ID size 64.

- [ ] **1. Write failing semantic cases** in TestConfigLimitsAndNames and TestConfigImageBounds. For every limit test max/max+1 and empty; Thai names allowed, control/bidi-format names rejected, optional name omitted allowed. Reject all controls in run except LF/CR/TAB and reject NUL/DEL; preserve legitimate shell syntax. Accept `localhost:5000/team/checks:dev`, `node:22-bookworm`, `registry.example/team/checks@sha256:` plus 64 hex digits; reject leading option, whitespace/control, dollar substitution, non-ASCII and image length >1024. Do not assert remote image exists. Validate invalid steps even in unselected checks.

- [ ] **2. Run failures**: `rtk go test ./internal/model ./internal/config -count=1`; identify the absent constraint each case exposes.

- [ ] **3. Add semantic rules once** in Config.Validate; keep existing ID helpers and return a path-bearing ValidationError. Unknown field paths sanitize/escape control characters and cap UTF-8 length; error messages never contain literal run/name/image values. Map parse failures to issue codes from spec; structural type errors must not leak json.UnmarshalTypeError contents. Add TestValidationDiagnosticsNoContent with a secret-like run payload and ESC in an unknown key: neither raw content nor raw ESC appears in Error() or JSON/human output.

- [ ] **4. Test and implement ApplyOverrides** in config.go. TestOverridesMatchConfiguredSelection asserts `jobs=[test]` works only when in cfg.Jobs, unknown/outside/duplicate IDs fail, architecture accepts exactly two values, omitted flags preserve settings and input slices remain unchanged. Replace the current hand-coded override block in run.Execute with this helper; no changes to snapshot comparison/order/lock.

- [ ] **5. Test all consumers**: invalid original config rejected by run before Docker; valid original followed by invalid captured config is rejected before image prep/execution. Extend existing run tests/stubs rather than adding an engine interface. Existing doctor/list consumers still call Load. Document any minimal caller adjustment needed for structured issues.

- [ ] **6. Verify and commit**: `rtk go test ./internal/model ./internal/config ./internal/run -count=1`; existing tests and strengthened semantics pass; diff contains no engine/snapshot changes.

## Task 3: Validate command and adversarial acceptance gate

**Interfaces**

- Consumes Load, ApplyOverrides and ValidationError via errors.As.
- Produces `validateCmd(out io.Writer, repo *string) *cobra.Command` in validate.go, registered by newRoot; `model.ValidateResult` fields exactly match the spec (Repo, ConfigPath, Valid, SelectedJobs, Architecture, CheckCount, StepCount, Issues with lower-camel JSON tags).
- CLI issue extraction is presentation only; it never accepts config independently. Produce private `writeCommandResult(out io.Writer, e model.Envelope, jsonOut bool) error` in root.go; wrap output failures in `outputError{err error}`. ExecuteContext handles this sentinel by returning 2 and writing only to stderr, never a second envelope. Use the helper for new commands and existing envelope-writing paths, preserving run cancellation/failure precedence.

- [ ] **1. Add TestValidateOfflineAndNoWrites**: create an empty Git repo and config whose run would create an outside marker; put fake docker/node/go executables in PATH that write markers; invoke validate and assert 0, valid=true, expected counts, no marker, no new repo/cache file. Git alone is permitted for ResolveRepo; do not execute `sh -n`, scripts, custom actions or network probes.

- [ ] **2. Add TestValidateJSONFailures/TestCommandFlagsAndEnvelope**: malformed/oversized/ambiguous/semantic/file inputs return 2, valid=false, one bounded issue and zero usable config; output decodes one envelope and EOF. Repo resolution error omits result; canceled context returns 130; positional args/unknown flags fail. Test --json=true/false, --repo with spaces/value `run` or `validate`, flag order, `--` and missing flag values. Existing doctor/list/run/version still report their command correctly. A failing stdout writer must not cause a second envelope.

- [ ] **3. Observe failures**: `rtk go test ./internal/cli -count=1`; command is absent before implementation.

- [ ] **4. Implement validate command**, Cobra NoArgs, job StringSlice/arch flags matching run, JSON flag local to command. Use cmd.Context; count all definitions; config path is absolute. Use ExecuteC's returned command context for failure naming; root/unknown-command failures use `pitstop`. JSON-mode detection must skip flag values and stop at `--`, including on syntax errors; prefer already parsed flag state, with a small flag-aware fallback when parsing failed. Add cases where --repo or --command has the literal value `--json`; that value cannot enable JSON mode. Do not infer commands from repo/flag values. Extend report.Human for new result summaries, with stderr-only handling for writer failures from writeCommandResult.

- [ ] **5. Add FuzzParse and FuzzConfigRoundTrip** in internal/config/fuzz_test.go. Seed duplicate escaped keys, controls, null, depth, Unicode, v1 and valid multiline configs. Properties: no panic; error yields zero Config; accepted output validates, marshals then reparses equivalently; raw command and semantic fields are preserved. Unit tests pin rejection expectations; fuzz success alone is not evidence of security.

- [ ] **6. Verify validator gate and commit**:

```sh
rtk go test ./internal/config ./internal/model ./internal/cli ./internal/run -count=1
rtk go test ./internal/config -run '^$' -fuzz '^FuzzParse$' -fuzztime=30s
rtk go test ./internal/config -run '^$' -fuzz '^FuzzConfigRoundTrip$' -fuzztime=30s
```

  Expected: unit/property tests pass with no markers/writes; fuzz reports PASS, no crash or violated invariant. Keep any discovered regression as a unit/seed case and rerun affected checks. Validator changes cannot progress to Task 4 with unexplained failures.

## Task 4: Canonical embedded examples and mapping cases

**Interfaces**

- Produces `agentassets.Preset(name string) ([]byte,error)`, `agentassets.PresetNames() []string`, `agentassets.Files() []string`, `agentassets.ReadFile(path string) ([]byte,error)`, and BundleVersion="1". Paths are bundle-relative sorted allowlisted files; PresetNames excludes custom (generated by flags).
- assets.go owns `//go:embed bundle` and read-only FS. Init consumes Preset; installer consumes Files/ReadFile; tests use Parse. No generated duplicate template bodies in Go.

- [ ] **1. Add failing TestBundleCompleteAndValid**: exact preset list from spec (custom handled separately), all `.json` config/mapping assets parse with Parse, SKILL.md/reference/manifest links resolve inside bundle, no absolute/escaping path. Retained examples/docker-react.json and docker-go.json are semantically equal to node-npm/go templates; modifying either side fails the test.

- [ ] **2. Run** `rtk go test ./internal/agentassets -count=1` and observe absent assets/API.

- [ ] **3. Create bundle resources and embed API**. Base config's step deliberately exits 2; other templates contain real commands and documented prerequisites, no success placeholders. Confirm explicit image tags and deterministic bootstrap from primary vendor docs during implementation; record actual refs and available platforms in references/examples.md. Use existing Node/Go values as starting points, not forced toolchain versions for other projects. No downloads to the host in this step beyond read-only documentation research.

- [ ] **4. Add workflow fixtures and mapping notes**: nonstandard names, shared shell script, local composite action, multi-workflow, monorepo, partial matrix/services/secrets/needs/expressions. Each pair documents selected scope, command/image source and omissions; no workflow CLI parser. Include Python, Java and Rust cases rather than only Go/React. Manifest-only/no-workflow scenario lives in mapping notes/agent trials. Mixed-language example uses one documented image with all needed tools.

- [ ] **5. Verify and commit**: `rtk go test ./internal/agentassets ./internal/config -count=1`; exact inventory passes and no example claims real execution without evidence. Root examples/README links remain valid.

## Task 5: Manual init using those same assets

**Interfaces**

- Consumes agentassets.Preset/Names, Parse and model limits.
- Produces `config.InitOptions{Preset,Image,Architecture string; Commands []string; DryRun bool}` and `config.Init(ctx context.Context, repo string, opts InitOptions) (model.InitResult,error)`; consumes resolved Git root. InitResult fields are those in spec, with Config model.Config and Warnings []string. Check cancellation before persistence and before claiming success; CLI passes cmd.Context.
- Produces `initCmd(out io.Writer, repo *string) *cobra.Command` in internal/cli/init.go. CLI resolves repo and architecture; generation/persistence is in config/init.go.

- [ ] **1. Add TestInitPreservesExistingFile/TestInitDryRun/TestInitOverrides**: original bytes unchanged on collisions; normal/broken symlink rejected without following; dry-run creates no file/dir and reports existing destination; base/default named starters/custom prerequisites; unsupported preset/arch fails; commands retain commas/newlines and generate step-1/step-2 in order; no flag evaluates shell substitution. Base is valid but has explicit failure placeholder warning.

- [ ] **2. Add TestInitExclusiveCreationAndWriteFailure**: two creators cannot truncate each other; one complete creator succeeds, the other returns CONFIG_EXISTS. Controlled failing write leaves no success result and retains the created destination for manual inspection; a retry returns CONFIG_EXISTS rather than unlinking it. Use a small private writer/publish seam only if a real filesystem cannot induce the failure; no filesystem interface hierarchy. Also test no HEAD required, source file unchanged, no cache/Docker markers and canceled-before-write.

- [ ] **3. Run** `rtk go test ./internal/config ./internal/cli -count=1` to observe missing behavior.

- [ ] **4. Implement generation and exclusive write**. Default base; named presets from embed; custom requires image/commands. Default platform from host GOARCH only and warn; allow explicit --arch for remote daemon. Reject field values via Parse before writing, no manifests/workflow execution or auto-detection. Open fixed destination exclusively under os.Root, write/sync/close, retain the destination after failed write/sync/close or cancellation for manual inspection and recovery, and never claim `written=true`; a retry returns CONFIG_EXISTS. No --force for init.

- [ ] **5. Wire CLI flags** --preset default base, --image, repeatable StringArray --command, --arch, --dry-run, --json and NoArgs. Add TestInitCLIJSON, flag conflict/missing-value tests, help inventory and exactly one envelope. Error names/codes/writer behavior follow Task 3.

- [ ] **6. Verify and commit**: `rtk go test ./internal/config ./internal/cli ./internal/agentassets -count=1`; dry-run config parses identically to actual file, all preservation tests pass.

## Task 6: Safe skill installation and replacement

**Interfaces**

- Consumes agentassets.Files/ReadFile/BundleVersion and config.ResolveRepo only for project scope.
- Produces `skill.Options{Agent,Scope,Repo string; DryRun,Force bool}`, `skill.Install(ctx context.Context, opts Options) (model.SkillInstallResult,error)` in internal/skill/install.go. Caller rejects explicit --repo with user scope using Cobra Flag.Changed; installer obtains home with os.UserHomeDir.
- Produces `skillCmd(out io.Writer, repo *string) *cobra.Command` with child install in internal/cli/skill.go; result fields as spec and manifest `{owner:"pitstop",bundleVersion:"1",files:[{path,sha256}]}`. The manifest's list covers every bundled file except manifest.json itself; its own content is verified against trusted embedded bytes.

- [ ] **1. Add TestInstallUserAndProject/TestInstallCopiedBundle**: use isolated home/empty Git repo; verify exact four destination mappings, file modes, all resources present, manifest hashes correct and relative references resolvable. Build/run installed CLI from a temp cwd without source checkout; assets still available. No network/project command/Docker required.

- [ ] **2. Add TestInstallPreservesUnknownDestination**: no-force existing folder, unrelated/user-authored folder with same name, unknown extra file, missing/invalid owner manifest, unexpected symlink, escaped source path, symlink parent/destination, destination regular file and permission error all fail without changing external/original sentinel bytes. Force cannot override confinement or erase unknown files. Test user scope --repo, missing/unknown agent/scope, --force dry-run and positional/unknown flags.

- [ ] **3. Add TestInstallTransactionalFailure/TestInstallContention**: staging write failure leaves old bundle intact; publish failure rolls back old bundle; failed rollback reports retained backup and recovery path, never installed=true; cancellation before publish preserves old and returns `installed:false`; cancellation observed after publish keeps `installed:true` and reports cleanup warning. Publish is the commit point. Two installers serialize or one fails cleanly, never combine bundles. Simulate each boundary through narrowly scoped private hooks in tests if needed; no broad FS abstraction.

- [ ] **4. Observe failures**: `rtk go test ./internal/skill ./internal/cli -count=1`.

- [ ] **5. Implement confined installer**. Fixed agent/scope paths; only embedded allowlist valid paths. Root operations reject escaping symlinks; check all selected-root descendants for symlinks/nonregular members before use, and use non-following opens for final files. Sibling exclusive lock, staging and manifest/hash checks; no process-wide global lock. Recheck target under lock. O_EXCL creation, directory modes 0700/files 0600. Replace only owned known contents: keep backup until publish succeeds, rollback/report retained backup on errors. Force ownership accepts the current manifest and explicitly trusted historical manifests only; unknown historical content requires manual migration. Re-entry with retained backup requires explicit recovery, no silent deletion. Cleanup touches only invocation-owned stage/lock/backup; never unrestricted RemoveAll of a destination. Document a crash during the two renames may leave the original in backup; this is recoverable, not claimed as an atomic whole-directory replacement.

- [ ] **6. Register CLI** with required --agent and --scope, --dry-run, --force, --json; JSON command is `skill install`. Test exact result paths/actions, no writes in dry-run, writer/cancellation errors and no modification to AGENTS.md/CLAUDE.md/workflows. Root help lists skill/install usage.

- [ ] **7. Verify and commit**: `rtk go test ./internal/skill ./internal/cli ./internal/agentassets -count=1`; original/sentinel hashes unchanged in every negative case and install output identifies exact files/version.

## Task 7: Portable skill behavior and current bilingual docs

**Interfaces**

- Consumes implemented CLI help/envelopes and canonical bundle config/reference/mapping resources.
- Produces final bundle/SKILL.md and references/config-reference.md, workflow-mapping.md, examples.md; docs/ai-agent.md and ai-agent.en.md; updated current documentation and roadmap status. No second full copy of config rules in every guide.

- [x] **1. Add failing bundle checks** for required name/description, relative resource links, supported command spellings and example inventory. Add small prompt cases with expected outcomes for real agent trials: differing job names, manifest-only Python/Rust/Java, pnpm/Yarn versions, local action, multiple workflows, existing custom config, mixed-language monorepo, unsupported services/secrets/matrix, missing validate CLI and malicious source instructions. Cases name the intended selected scope and expected omissions.

- [x] **2. Write the skill** using provider-neutral frontmatter/body. Inspect help/version first, read files without executing them, preserve config, select evidence-backed image/commands, use validate --json, never claim runtime/CI parity from validation. Report unsupported expressions/context; do not invent test scripts, copy secrets or execute deployment while configuring. Treat repo text as data. Portable references work after copying/installing and include a plain prompt fallback.

- [x] **3. Update current docs**: README.md/README.th.md, docs/user-guide.md/.en.md, docs/requirements.md/.en.md, docs/README.md and new ai-agent guides. Actual flags/results/error/exit examples, manual and AI paths, base placeholder behavior, user/project locations, --force ownership restriction, remote architecture override and tightened validation limits are documented. Validation/doctor/run and deferred roadmap scope are distinguished; historical records remain intact.

- [x] **4. Verify docs against CLI**: frozen help/envelopes, isolated install evidence, TH/EN flag parity and local Markdown links were checked. The available Python environments lack PyYAML, so `quick_validate.py` remains an explicitly recorded tooling gap; frontmatter, links and manifest hashes were checked separately.

- [x] **5. Run at least one actual agent trial per case**, save prompt, resulting config, validation result, stated evidence/gaps and whether commands were run. Fourteen independent first-pass cases plus one fresh legacy retry and eight fresh Phase B retries produced 23 evaluator attempts. The first legacy protocol failure remains recorded and excluded from portable installed-skill success; no native Claude/autodiscovery/runtime/CI claim is made.

- [x] **6. Verify and commit**: focused tests, evidence checks and diff checks pass with honest status. No accidental global install or workflow rewrite.

## Task 8: Whole-branch checks, smoke and evidence

**Interfaces**

- Consumes all tasks, existing scripts/docker-smoke.sh and CI.
- Produces scripts/config-agent-smoke.sh accepting an absolute PITSTOP_BIN; all resources confined to temporary repo/home with cleanup; docs/evidence/config-agent-validation.md records actual commands/environment/outcomes/limits.

- [x] **1. Add offline smoke**: temp empty Git repo; init base dry-run/no file, actual init, validate, invalid duplicate config rejected, existing config preserved, custom harmless check config, project skill dry-run/no writes, install, no-force collision, owned force replacement. Parse stdout with existing scripts/validate-json.go; additionally assert command, exit and relevant result fields so merely valid JSON is insufficient.

- [x] **2. Add example smoke**: parse all embedded templates/mapping configs with canonical parser and run representative Node/Go plus at least two newly covered toolchains on a real Docker daemon in temporary fixtures with actual manifests/locks. Select those new toolchains explicitly before execution and record them in evidence. If a toolchain/image is unavailable, document unverified status; schema validation does not substitute for runtime evidence. Preserve snapshot/cleanup smoke and original repo invariants. Do not claim all languages were tested.

- [x] **3. Run final checks once after all changes**:

```sh
rtk go test -race -count=1 ./...
rtk go vet ./...
rtk go test ./internal/config -run '^$' -fuzz '^FuzzParse$' -fuzztime=30s
rtk go test ./internal/config -run '^$' -fuzz '^FuzzConfigRoundTrip$' -fuzztime=30s
rtk git diff --check
```

  Then build the CLI into a temporary directory and run offline/config-agent smoke plus existing docker-smoke with PITSTOP_BIN set to that exact binary. Verify builds/tests with the Go 1.26.5 floor in CI or a controlled toolchain; local Go 1.27.1 alone does not prove floor compatibility. Do not change go.mod merely to use json/v2. Fix failures and rerun affected tests; only broaden after new failures/changes.

- [x] **4. Update .github/workflows/ci.yml** with offline config-agent smoke and example parsing beside existing checks. Use the CI-built binary; preserve formatting/dependency/race/vet/Docker/JSON checks. Fuzz runs may remain explicit local acceptance commands; seeded fuzz tests run in ordinary go test. Never claim remote CI passed without actual remote evidence.

- [ ] **5. Review trust boundaries and record evidence**: the 2026-10-09 whole-branch review and its five Important/two Minor findings are preserved in permanent evidence; the bounded resolutions, user-requested test-gap follow-up and correction-round reruns are recorded there. Confirm every Load consumer is covered, no parser bypass, no raw-value diagnostics, direct ID/count/rendered-path boundary coverage, no unsafe file operations/unknown force deletions, all dry-runs truly read-only. The correction-round checks pass; the coordinator follow-up gate remains pending independent scoped re-review. Remaining remote CI, OS, runtime, security and trial boundaries stay explicit.

- [x] **6. Commit task artifacts and hand off**: `rtk git diff --check`, `rtk git status --short --branch`; show implementation summary/evidence and any open acceptance item. No automatic merge/push/release/install in the user's real home.

## Self-review and execution handoff

Planning review performed against the spec: all CLI contracts have an owner; JSON/file/text limits and nonexecution checks are in Tasks 1–3; examples and language/workflow variation in Task 4; preservation and install recovery in Tasks 5–6; agent trials/docs in Task 7; end-to-end evidence in Task 8. Review Focus items map to named tests above. Public result fields/commands/preset inventory come from the spec; do not invent a parallel contract in implementation.

Execution method: the user selected luna_worker implementation with independent task gates, whole-branch review and one scoped fix-wave review. The [execution record](../../evidence/config-agent-execution.md) preserves actual results, process omissions, rulings and deferred test work. Earlier unchecked step boxes remain planning history rather than invented behavioral-RED evidence.
