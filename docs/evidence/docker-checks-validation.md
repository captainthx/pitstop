# Docker-only validation — 2026-10-07

Implementation: `feature/docker-checks`, Docker-only backend. Release/merge/push were not performed.

Validated on macOS ARM64 with Docker 29.4.0, Linux ARM64 containers.

- `go test -race -count=1 ./...`: 26 tests passed in 9 packages.
- `go vet ./...`, formatting, trimmed CLI build and diff checks passed.
- Root and command help succeeded; version JSON passed the existing validator.
- Real Docker smoke (`alpine:3.22`) passed success, failure/remaining-step skip, independent fresh containers, same-job file reuse, dirty/new/deleted/ignored snapshot input, paths with spaces, timeout, Ctrl+C, retained report equality and ownership cleanup. The unrelated smoke container survived.
- Existing Pitstop project script `go run ./scripts/validate-json.go` passed in `golang:1.26.5` using a temporary clone/config. Original project files were not modified. Persisted report equaled stdout JSON; snapshot/scripts were removed and no container with the exact run label remained.
- Independent review confirmed prior stream-drain, cancellation and cleanup/result-preservation defects were fixed. A 4 MB output regression checks that logs drain before command wait.

Go project image ID: `sha256:04b188c2db8449ea15c09355dfab9ce9ed4ecf9414e94674104e316bda7d1996`.

Image digest: `sha256:705e964a93a2fd2e75c7d59bb7d781b57e30f12293ffde5175c69229e18fb678`.

Local retained report: `/Users/thanyatab/Library/Caches/pitstop/runs/1791390353467895000/report.json`.

Docker create uses the immutable repository digest when available and the image config ID for locally built images without a digest. The report retains the platform-specific image ID. This avoids Docker multi-platform config IDs that are inspectable but cannot be used directly for create.

GitHub CI includes the smoke command; its remote execution is not verified by these local checks. These checks validate Docker checks, not GitHub Actions workflow interpretation. Historical act evidence remains historical.
