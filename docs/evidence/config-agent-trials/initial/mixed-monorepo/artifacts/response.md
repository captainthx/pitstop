Created `.pitstop.json` for the requested mixed monorepo checks.

- Selected `packages/frontend` (`npm ci && npm test`) and `services/api` (`go test ./...`) in one `verify` job.
- Chose `pitstop-local/node-go-tools:2026.10` with explicit `linux/arm64`; it combines Node 22 and Go 1.26.5.
- `pitstop validate --repo <fixture> --json` passed: exit 0, valid config, 1 job/check, 2 steps, no issues.
- No project commands, Docker, network, or deployment execution was performed.

Artifacts include the config copy, CLI version/help/validation outputs, and evidence with scope, assumptions, gaps, and the full command list.
