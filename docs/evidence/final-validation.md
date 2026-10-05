# Final product validation

Pitstop MVP was implemented by Luna and independently exercised through the CLI on macOS ARM64, act 0.2.89, Docker 29.4.0 / OrbStack and the Linux ARM64 runner. The final binary SHA-256 is `80b73aa7c6e2e8e063befaa34d0d03a121a22040123fc4523ce41103cb7fab6c`; build toolchain Go 1.27.1. The required Go 1.26.5 baseline also passed the complete test suite. A fresh whole-implementation audit found two Important defects. Both were fixed with failing-then-passing regressions, followed by the complete tests/race/vet and actual application checks. See [audit findings and resolutions](final-review.md).

| Check | Final result |
|---|---|
| Go tests | 35 test functions passed; Go 1.27.1 and baseline Go 1.26.5 |
| Race detector / vet / build | passed |
| version / invalid flags / list / blocked selection | expected exits 0 / 2 / 0 / 2, one JSON document |
| intentional failure | exit 1, failure category unknown, executed coverage complete |
| dirty snapshot | exit 0; working content overrides staged, tracked .env edits preserved, deletions absent, untracked files and spaced paths present, executable mode preserved, ignored files/local env excluded |
| timeout / cancellation | TIMEOUT exit 1; SIGINT exit 130; final report persists |
| workflow boundaries | empty YAML, empty steps, conditional-only jobs, secret in local composite and integration command drift all exit 2 |
| final React verify | exit 0, selected/workflow coverage complete; 74.91s total / 73.643s engine job |
| final Go default profile | lint and test passed, integration visibly blocked; exit 2, coverage partial; 42.32s total |
| final forced integration | actual PostgreSQL/Testcontainers integration passed, exit 0; 38.97s total / 37.932s engine job; selected coverage complete, workflow coverage partial, exactly two guard removals |
| report retention | persisted JSON equals stdout; original/prepared hashes match retained files; real argv retained |
| cleanup / source integrity | final run snapshots and HOME removed, zero Pitstop runners left; original app Git statuses clean; existing PostgreSQL/MinIO containers preserved |

Final React and service checks used independent clones with their original workflows and example configs. They overlapped in time and shared existing caches; these timings are observations, not cold-cache or comparative benchmarks. Earlier React runs took 50.12s and 48.57s and remain in the historical evidence. Initial boundary failures are also preserved, with separate final passing results.

Supported runtime evidence covers macOS ARM64 hosting Linux ARM64 jobs with a local Unix Docker endpoint. Linux AMD64 is an accepted explicit configuration but has not been exercised here. Absolute source symlinks, submodules, unsupported workflow semantics and credential-dependent jobs are blocked. Paths-filter native conditional support remains deferred because the real original changes job fails without a token/checkout; forced integration clearly reports reduced workflow fidelity. Step details remain empty when no dependable typed evidence exists.

Evidence: [final core checks](product-acceptance/final-core-results.json), [final workflow boundaries](product-acceptance/final-workflow-results.json), [final React](product-acceptance/react-final-results.json), [final service](product-acceptance/service-final-results.json), [environment](product-acceptance/validation-environment.json), [source integrity](product-acceptance/source-integrity.json). Run directories under product-acceptance retain envelopes, reports, hashes, manifests, prepared workflows, argv and raw logs. Runtime feasibility probes are separate in runtime-probes/.
