# Measured act runtime contract — 2026-10-05

Parent-agent probes ran on macOS ARM64, act 0.2.89, Docker OrbStack with endpoint `unix:///Users/thanyatab/.orbstack/run/docker.sock`. Runner Linux ARM64, `catthehacker/ubuntu:act-latest`, digest `sha256:c58e2b364da03b0c804c7d660f2ecbedf2f221a382b9baa0b344b0144780ff43`. Go host is 1.27.1; Go 1.26.5 remains the declared build baseline rather than a host test claim.

| Probe | Actual outcome | Duration |
|---|---|---:|
| Success + dirty source marker + tracked snapshot actrc | act exit 0, marker visible, dummy actrc env absent | 1.02s |
| Step `exit 7` | act exit 1, structured jobResult failure | 0.43s |
| SIGINT sleeping job | act exit 1, labeled runner removed | 2.79s |
| React verify, unchanged workflow with isolated HOME/cwd | act exit 0 | 48.82s |

All probes launched from neutral cwd with isolated HOME/XDG_CONFIG_HOME and explicit DOCKER_HOST resolved before isolation. Only PATH/TMPDIR/locale were allowed from host. No cloud/token environment or input/secret/var files were imported. `--container-daemon-socket -` disabled runner socket access. Existing action/dependency caches were explicitly named, so this React run was a cached observation, not a cold benchmark.

The fixture committed `.actrc` with `--env PITSTOP_BAD=leaked` into its source snapshot; `-C` plus neutral cwd did not load it. Its success step asserted absent PITSTOP_BAD and PITSTOP_HOST_TOKEN, and read `dirty-source` from an uncommitted marker over `committed-source`. This proves dirty source copy in the engine probe, separately from Pitstop snapshot acceptance.

`--container-options '--label pitstop.parent-probe=cancel'` produced an inspectable runner with exact ID before SIGINT; after act stopped there were zero matching runner resources. Cancellation is not act exit 130: Pitstop must map its own cancellation context to 130. Cleanup must still identify and remove owned IDs when act does not finish cleanup.

JSONL stdout includes `jobID`, `jobResult` (success/failure), `stepResult`, `stage`, `stepID` array, and optional `executionTime` in nanoseconds. Some completion records use lowercase `stepid`. General logger records are on stderr. Raw step output has `raw_output: true`; never treat workflow-printed success text alone as engine completion. Raw logs are retained including unknown records.

Evidence: [probe results](runtime-probes/results.json), [success](runtime-probes/success.jsonl), [failure](runtime-probes/failure.jsonl), [cancel](runtime-probes/cancel.jsonl), [isolated React](runtime-probes/react-verify.jsonl). These are fresh actual runs, not synthetic fixtures. Product-binary acceptance is recorded separately in final-validation.md.
