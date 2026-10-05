# paths-filter local trial — 2026-10-05

Original recruit100-service `changes` job was run unchanged without a token through act 0.2.89 using a local pull_request event artifact. Actual base SHA `db1afda60ca19e2a7474f75e3a033dd1f8905d56`, head SHA `e58f8403f675e49e3e9681ad9492d158622bb0c2`. No invented PR number or credential was supplied.

Baseline, dirty tracked go.mod, and nonignored untracked migrations file all exited 1. paths-filter logged that GitHub token was unavailable and it would use git diff, then `git branch --show-current` failed with `fatal: not a git repository`. This workflow job has no checkout step; act did not copy a Git checkout into that job's runner.

Therefore this trial does not establish dirty/untracked filter semantics. A committed-change comparison was not advanced after the baseline failed to access Git; generic conditional/dependency support remains deferred. Do not change the original workflow or substitute a new glob evaluator to make this feature appear supported.

Evidence: [all cases](runtime-probes/paths-results.json), [baseline](runtime-probes/paths-baseline.jsonl), [dirty](runtime-probes/paths-dirty-tracked.jsonl), [untracked](runtime-probes/paths-untracked-persistence.jsonl), [event](runtime-probes/paths-event.json).
