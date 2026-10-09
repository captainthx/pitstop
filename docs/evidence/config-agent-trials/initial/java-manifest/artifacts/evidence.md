# Evidence

## Selected scope

- Job: `verify`
- Step: `test` / `Run Maven wrapper tests`
- Command: `./mvnw -B -ntp test`
- Runner: `maven:3.9.11-eclipse-temurin-21`, `linux/arm64`

## Repository evidence

- `pom.xml` declares a Maven project (`example:fixture-java:1.0.0`).
- `mvnw` is checked in and was inspected as data; it is a placeholder script and has mode `-rw-r--r--`.
- No existing `.pitstop.json` was present.
- The installed skill's Java Maven example and mapping select the wrapper test command above.

## Assumptions and gaps

- The normal test task is Maven's standard `test` lifecycle phase, invoked through the checked-in wrapper.
- The configured image/tag and architecture are explicit starter choices from the installed skill; image availability and runtime compatibility were not tested.
- Runtime readiness requires replacing the placeholder with a real executable Maven wrapper and providing the project test/dependency metadata needed by that wrapper.
- No workflow parity, dependency installation, Docker execution, or test result is claimed.

## Commands and execution policy

- Read-only inspection: `pitstop-phase-a version`, `pitstop-phase-a -h`, `pitstop-phase-a validate -h`.
- Validation: `pitstop-phase-a validate --repo <fixture> --json` returned exit code `0`, `valid: true`, one job, one step, and no issues.
- Project commands run: **No**. The Maven wrapper was never executed; only the Pitstop validator ran.

## Agent metadata

- Codex subagent: `luna_worker`, model `gpt-5.6-luna`, reasoning `high`.
- Client version: unavailable.
- Skill source: installed `.agents/skills/pitstop-config/SKILL.md` and its relevant local references only.
