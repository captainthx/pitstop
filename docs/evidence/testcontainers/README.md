# Dedicated Testcontainers daemon acceptance

Observed 2026-10-28 on macOS ARM64 with Linux ARM64 Docker 29.8.2. [Summary](summary.json) records the binary hash, cases and exits; sibling JSONs and per-case directories retain raw reports, logs and creation argv. Paths in those records refer to the observed disposable fixture/cache, not a portable checkout.

The new generic Go fixture pins Testcontainers Go v0.44.0 and requests a BusyBox 1.37 HTTP service. Its test connects through the mapped port at `http://pitstop-dind:<port>`, asserts the response, then terminates the service. Ryuk remains enabled. No private example repository, host socket, source bind, credentials or outer published daemon port was used.

| Case | Observed exit | Assertion |
| --- | --- | --- |
| Missing authorization | 2 | Entire run blocked; Docker resources unchanged |
| Service round-trip | 0 | Actual HTTP response and named Go test passed |
| Intentional assertion failure | 1 | HTTP succeeded, then the expected assertion failed |
| Provisioning timeout | 1 | Timeout during daemon setup; owned resources removed |
| Runtime timeout | 1 | Daemon ready and project step marker observed before timeout |
| SIGINT | 130 | This fixture's runner started before cancellation |
| Injected daemon start failure | 2 | Real allocated service/network/data removed |
| Injected source-copy failure | 2 | Daemon identity/readiness/diagnostics retained; resources removed |

Every run asserts that its labelled outer containers, networks and volumes are gone, and that an unrelated labelled network and volume survive. Failed executions retain daemon diagnostics. The smoke compares tracked fixture source with Git and confirms source/tests are unchanged.

Replay from the repo root after building the CLI:

```sh
go build -trimpath -o /tmp/pitstop ./cmd/pitstop
PITSTOP_BIN=/tmp/pitstop PITSTOP_SMOKE_KEEP=1 sh scripts/testcontainers-smoke.sh
```

Full noncached race tests and vet passed; the existing offline config-agent smoke and ordinary Docker smoke also passed. Focused tests cover strict optional boolean parsing, mixed-selection authorization, runner isolation, label recovery after lost create output, changed ownership, foreign volume refusal, and image preparation not consuming the check timeout. The retained RED log demonstrates authorization failure before implementation; parser tests were already GREEN when recorded, so no parser RED claim is made.

An initial fixture attempt failed because Alpine lacked `httpd`; the fixture was corrected to BusyBox and the full acceptance rerun passed. No Pitstop behavior was weakened to make the service test pass. Linux amd64 execution is added to CI and must be evaluated separately from this local ARM64 evidence. No public release was created.

Final source commit `c530cf4` passed [GitHub CI](https://github.com/captainthx/pitstop/actions/runs/40696364818), including real Linux amd64 Testcontainers and Linux/macOS installer/package checks. This completion note changes documentation only.
