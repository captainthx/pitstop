Created and validated the requested Pitstop configuration.

- Config: `.pitstop.json` (copied to this artifacts directory)
- Scope: `verify` only, one step: `npm run check`
- Runner: `node:22-bookworm`, `linux/arm64`
- Validation: exit code `0`; `valid: true`; one selected job and one step; no issues
- Action metadata gap: composite input `mode: test` has no schema-v2 field and is documented as omitted
- Project execution: none; the action and workflow were inspected as data only

Artifacts: `.pitstop.json`, `cli-output.md`, `evidence.md`, and this response.
