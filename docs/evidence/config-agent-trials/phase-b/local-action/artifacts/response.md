Configured the local composite verification scope in `.pitstop.json` and validated it successfully.

- Selected job/check: `verify`
- Runner: `node:22-bookworm`, `linux/arm64`
- Step: `npm run check`
- Validator: `valid: true`; 1 check, 1 step, 0 issues; exit `0`
- Runtime gap: `check` delegates to `npm test`, but `package.json` has no `test` script.
- Schema-v2 omissions: action metadata and input (`mode: test`), composite shell/using, checkout, local action path, and workflow runner label.

Evidence and raw CLI outputs/exits are in this directory. No project check was executed.
