# Config reference

Pitstop configs use `schemaVersion: 2`, one Linux runner image, selected `jobs`, and ordered shell `steps`. The image must be an explicit supported tag or digest; the parser does not prove registry availability. Commands are executable shell text and require human review before `pitstop run`.

Use the starter matching project evidence, then replace its commands with the project's real lockfile and test scripts. `base` is an intentional starter: its configure step exits 2 until a project check is defined. `custom` is supplied through init flags and has no placeholder template.

Prerequisites are part of the evidence:

| Preset | Evidence and prerequisite |
| --- | --- |
| npm | A lockfile and a real `check` script; use `npm ci` before `npm run check`. |
| pnpm | A pinned `packageManager`, Corepack bootstrap, and lockfile; use `pnpm install --frozen-lockfile`. |
| Yarn Classic | A pinned Yarn 1 `packageManager`, Corepack bootstrap, and lockfile; use `--frozen-lockfile`. |
| Yarn Berry | A pinned Yarn 4 `packageManager`, Corepack bootstrap, and lockfile; use `--immutable`. |
| Go | The module's selected Go toolchain and real vet, test, and build targets. |
| Python pip | A requirements or project dependency file declaring `pytest` (and lint dependencies); pin versions or hashes when reproducibility is required. |
| Python uv | A locked `uv.lock`, project test dependencies, and `uv sync --locked` before `uv run`. |
| Maven / Gradle | A checked-in `mvnw` or `gradlew` wrapper and project test task. |
| .NET | A reviewed solution or project file; restore before `dotnet test --no-restore`. |
| Rust | A lockfile and Cargo test/build targets; use `--locked`. |
| Composer | Composer scripts plus required PHP extensions; use the lockfile and `composer run-script test`. |
| Bundler | A lockfile and reviewed Rake task; use `bundle exec rake`. |

Inspect these prerequisites without executing project commands while configuring.

The runner architecture is explicit (`linux/arm64` or `linux/amd64`). A starter is parser-valid, not a claim that its image, dependencies, or checks were executed.
