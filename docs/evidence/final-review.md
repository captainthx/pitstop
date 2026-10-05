# Final whole-implementation audit and rulings

A fresh qa-work-auditor reviewed all code at `8d79719` against the specification and implementation plan. It reproduced two Important defects, one using real act/Docker. No Critical or Minor findings were raised. The initial audit verdict was **With fixes**; it must not be misrepresented as an unconditional approval. The executor resolved the findings in one regression-driven fix pass; no second reviewer was dispatched.

| Finding | Root fix | Runnable regression |
|---|---|---|
| Conditional local composite produced a false green | Reuse supported-step checks in recursively inspected local composite `runs.steps`, including conditions, continue-on-error and empty/nonexecutable steps | `go test ./internal/workflow -run TestInspectBlocksConditionalAndContinueOnErrorCompositeSteps -count=1` |
| Later act launch failure discarded an earlier failed job and final report | Accumulate launch uncertainty as a skipped/unknown result, continue normal cleanup/report aggregation, retain failure exit precedence and results even if metadata/report writes fail | `go test ./internal/run -run TestLaterLaunchErrorPreservesEarlierFailureAndReport -count=1` |

The run-launch regression failed before the fix with exit 2 and no results; it passed after the fix with exit 1, both job outcomes and the persisted report. The local composite regression failed before its fix with no capability reasons; it passed after local composite checks were added. A separate cancelled-preflight regression failed with exit 2 and passed with cancellation exit 130 after checking context through the shared failure path.

## Rulings recorded during implementation

1. Host Go 1.27.1 was initially used rather than claiming untested Go 1.26.5 compatibility. The full test suite was subsequently exercised successfully on Go 1.26.5 as well.
2. The supplied checkout initially had no Git history. A local `feature/pitstop-mvp` branch was initialized; no remote, publication or PR was created.
3. The requested `go.yaml.in/yaml/v3` resolves as v3.0.5. That module path/version is used; the initial v3.0.1 metadata mismatch was corrected.
4. An early ruling retained null failure categories for ordinary nonzero engine exits. Final review superseded it: failed jobs use `unknown` when evidence is insufficient; timeout uses `engine`.
5. Absolute source symlinks are rejected even if their target is internal, because preserving them could dereference the original checkout. Relative internal links are supported. Relocating absolute links remains outside this MVP.

## Behaviors the reviewer declined to judge

- Absolute internal symlinks: executor accepts the documented fail-closed restriction; no unsafe source dereference is allowed.
- AMD64 execution: executor records it as untested, rather than claiming a runtime result from ARM64 evidence.
- Remote action internals: executor accepts delegation to act as specified. Local action metadata is inspected recursively.
- YAML merge-key dependency bypass: reviewer investigated and rejected the finding because act 0.2.89 rejects `<<` during schema validation; executor accepts that ruling. No merge-key support is claimed.

Deferred Minor findings: none. Platform/semantic restrictions above remain explicit MVP limits, not hidden passing results.
