# Go setup and installed bundle 3 trials

Observed 2026-10-28 with fresh generic Go repositories on macOS ARM64 / Linux ARM64 Docker 29.8.2. [Prompts](prompts.md) record the evaluator inputs; [summary](summary.json) identifies the actual CLI/manifest hashes and outcomes. No private example code was copied, modified or executed.

`prepare.py` constructs disposable fixtures from the public generic Testcontainers HTTP smoke, adding a unit test, a tagged environment-gated external test and a nested tools module. Their Makefile/CI commands are evidence, not instructions to the evaluator. The initial config selects only unit tests. `prepare.py` also reconstructs exact bundle 2 and verifies every historical file hash for baseline replay.

| Direct-read trial | Actual result |
| --- | --- |
| Bundle 2, configure only | Found tagged integration and nested tools, preserving the unit check. Omitted `testcontainers: true` despite reading the helper. Validation/list/doctor passed; no tests ran. |
| Installed bundle 3, explicit DinD grant | Preserved unit, added integration with capability and tools; actual unit, mapped HTTP Testcontainers and nested-module tests passed. External test skipped; evaluator reported incomplete external coverage. |
| Installed bundle 3, ordinary test request | Ran unit/external/tools without privilege; unit/tools passed and external skipped. Integration stayed unrun because explicit DinD authorization was absent. |

The baseline did **not** reproduce missing tagged-test discovery; the observed gap was the missing runtime capability. Neither validation nor doctor was called a test pass. First post-change execution attempts correctly reported Docker unavailable; those raw blocks are retained separately. After restarting Docker, the same requests continued and produced the retained successful reports.

The post-change evaluators read the skill actually installed by the fresh CLI into a disposable user HOME; its bytes match the candidate bundle exactly. Parent checks confirmed the original unit check remained identical, only config/evidence changed, Go JSON contained the named real test passes and external skip, and all labelled outer containers/networks/volumes were removed. Shell exit 0 did not erase the external coverage gap.

Authenticated bundle-2 and actual v0.1.0 upgrades pass, with existing unknown/modified-file protections retained. The worker's original RED and a parent replay against pre-change commit `eed3d67` both fail because bundle 2 cannot satisfy the expected bundle 3 upgrade; GREEN, full noncached race/vet, manifest/history hash checks, skill validation and offline smoke passed. The generic Go preset remains unchanged. Full Testcontainers lifecycle evidence is in [the runner acceptance](../testcontainers/README.md).

To prepare another isolated trial, run `python3 docs/evidence/go-integration-setup/prepare.py`, build the CLI, install its skill into a new disposable HOME, and use the recorded prompts. Reports contain original fixture/cache paths; sibling copies preserve the observed evidence. These bounded trials establish the recorded behavior, not universal agent reliability or external-service coverage. No public release was created.
