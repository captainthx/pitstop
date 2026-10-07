> Historical record / เอกสารประวัติ: describes an earlier implementation, not the current CLI. ดู [คู่มือปัจจุบัน](../../README.th.md) สำหรับการใช้งาน Docker checks.

# SDD ledger — plan: docs/superpowers/plans/2026-10-05-pitstop-mvp-1.md

Pre-flight: shared interfaces are model/config → workflow → snapshot → engine → report → coordinator; adapter consumes workflow plan and produces a prepared workflow.
Ruling: the host has Go 1.27.1 while the plan names 1.26.5 — build and report the observed toolchain, and avoid claiming the older baseline until it is tested.
Ruling: the empty repository had no Git history — initialize a local feature branch and preserve all supplied documentation as the initial project content.
Ruling: the requested go.yaml.in/yaml/v3 module resolves at v3.0.5 on this host; use that requested path/version after the initial v3.0.1 metadata mismatch.

Task 3: complete (commit 3166bdb, tests: go test ./... → 18 passed)
Task 4: complete (commit 3166bdb, tests: workflow fixtures → passed)
Task 5: complete (commit 3166bdb, tests: list/planning fixtures → passed)
Task 6: complete (commit 3166bdb, tests: snapshot fixtures → passed; actual dirty snapshot React run produced non-empty digest)
Task 7: complete (commit 3166bdb, tests: go test ./internal/actengine and race suite → passed)
Task 8: complete (commit 3166bdb, tests: report contract tests → passed)
Task 9: complete (commit 3166bdb, tests: run/CLI contract tests → passed)
Task 10: complete (commit 3166bdb, tests: adapter shape and drift fixtures → passed; runtime evidence supplied separately)
Task 11: complete (commit 3166bdb, tests: go test ./..., go test -race ./..., go vet ./... → all passed; product React/service runs supplied separately)
Task 12: complete (commit 3166bdb, tests: go build -trimpath and binary version/help smoke → passed)
Ruling: generic engine failure results keep failureCategory null unless evidence distinguishes validation/environment/engine; timeout keeps engine category — cost if wrong: callers receive less classification for ordinary nonzero act failures.
Ruling: absolute symlinks are rejected even when their resolved target is inside the repo — this prevents snapshot execution from dereferencing the original checkout; cost if wrong: a rare valid absolute internal link requires a future relocation transform.

Task 1: complete (parent real act success/failure/no-op/cancellation neutral cwd/HOME trials; docs/evidence/engine-contract.md and runtime-probes).
Task 2: complete (parent actual original React/Go lint/test/Testcontainers forced integration and paths-filter trials; docs/evidence/service-runtime.md, paths-filter-trial.md).
Final review: fresh qa-work-auditor of entire implementation at 8d79719; 2 Important, 0 Critical, 0 Minor.
Final fix: local composite control checks RED capability reasons absent → GREEN blocked; commit 7824cff.
Final fix: late launch error RED exit2 dropped first failure/no report → GREEN exit1 retained both outcomes/report; storage failure and cancelled preflight also retained correctly; commit 1c940c6.
Final tests: go test -race -count=1 ./..., GOTOOLCHAIN=go1.26.5 go test -count=1 ./..., go vet ./... all passed (35 test functions); final binary actual React/service/default/forced/core checks passed expected exits.
Final review clean: both Important findings resolved by regressions and final acceptance, no second reviewer dispatched.
Final: Ruling: reviewer accepted absolute symlink restriction; retained explicit blocked behavior.
Final: Ruling: AMD64 execution remains untested; accepted configuration is not claimed runtime proof.
Final: Ruling: remote action internals remain delegated to act; local action metadata is inspected.
Final: Ruling: YAML merge-key bypass is not a finding because act 0.2.89 rejects schema; no merge support claimed.
Final: Ruling: previous null failure category ruling superseded; insufficient evidence now reports unknown.
Final: Ruling: previously untested Go1.26.5 baseline subsequently passed entire suite.
