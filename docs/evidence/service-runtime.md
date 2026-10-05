# Measured service runtime — 2026-10-05

The parent agent ran act 0.2.89 in an independent local clone of recruit100-service HEAD `e58f8403f675e49e3e9681ad9492d158622bb0c2`. The original application repository was not edited. Neutral launcher cwd, isolated HOME/XDG, explicit Docker endpoint and empty env/secret/var/input files were used; host token/cloud environment was not inherited.

| Job | Workflow | Actual exit | Duration |
|---|---|---:|---:|
| lint | unchanged ci.yml | 0 | 91.55s |
| test | unchanged ci.yml | 0 | 44.43s |
| integration | temporary copy, removed only integration needs/if | 0 | 37.20s |

Original workflow SHA-256 `fb46b6d89426d9b4d9f1ee46c79020f59a8cfcb0630f1022c59b649619682394`; prepared integration workflow SHA-256 `642ef60675d002037b33b0bff974e586f747fadc80754992f478fd736edc5b36`. The actual original condition was `needs.changes.outputs.persistence == 'true'`, and needs was `changes`. All validation steps stayed unchanged.

Lint/test disabled Docker socket mounting. Integration passed `--container-daemon-socket unix:///Users/thanyatab/.orbstack/run/docker.sock`; inside the Linux ARM64 runner, actual integration-tagged PostgreSQL infrastructure package tests ran and passed. This proves the integration steps/Testcontainers runtime on this host; it does not prove original PR paths-filter semantics. Integration must remain opt-in with shape checks and workflowCoverage partial.

No labeled runner remained after any trial. Scope of this residual check is owned runner resources; Testcontainers resources are checked separately by labels/session ownership rather than broad deletion. Never stop unrelated PostgreSQL/MinIO containers.

Evidence: [results and argv](runtime-probes/application-results.json), [lint](runtime-probes/service-lint.jsonl), [test](runtime-probes/service-test.jsonl), [integration](runtime-probes/service-integration.jsonl), [prepared workflow](runtime-probes/service-integration.yml). Product-binary acceptance is separate from these feasibility probes.
