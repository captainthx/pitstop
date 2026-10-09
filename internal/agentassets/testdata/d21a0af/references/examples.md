# Canonical examples and evidence

The embedded starters are parser-valid examples. Their commands and images are chosen from project evidence and documented vendor references; this bundle does not claim that every image or command was executed.

## Presets

- [base](../assets/examples/base.json): `alpine:3.22`; deliberate exit-2 configure placeholder.
- [node-npm](../assets/examples/node-npm.json): `node:22-bookworm`; `npm ci`, then `npm run check`.
- [node-pnpm](../assets/examples/node-pnpm.json): `node:22-bookworm`; pinned Corepack pnpm bootstrap and frozen install.
- [node-yarn-classic](../assets/examples/node-yarn-classic.json): `node:22-bookworm`; pinned Yarn 1 and frozen lockfile.
- [node-yarn-berry](../assets/examples/node-yarn-berry.json): `node:22-bookworm`; pinned Yarn 4 and immutable install.
- [go](../assets/examples/go.json): `golang:1.26.5`; vet, race tests, and build.
- [python-pip](../assets/examples/python-pip.json): `python:3.13-slim-bookworm`; requirements install and pytest.
- [python-uv](../assets/examples/python-uv.json): `ghcr.io/astral-sh/uv:0.12.23-python3.13-trixie-slim`; locked uv sync and uv-run pytest.
- [java-maven](../assets/examples/java-maven.json): `maven:3.9.11-eclipse-temurin-21`; checked-in Maven wrapper test.
- [java-gradle](../assets/examples/java-gradle.json): `gradle:8.14-jdk21`; checked-in Gradle wrapper test.
- [dotnet](../assets/examples/dotnet.json): `mcr.microsoft.com/dotnet/sdk:9.0.318`; restore and test a project or solution.
- [rust](../assets/examples/rust.json): `rust:1.99.0-bookworm`; locked test and build.
- [php-composer](../assets/examples/php-composer.json): `composer:2.10.3`; Composer install and project test script.
- [ruby-bundler](../assets/examples/ruby-bundler.json): `ruby:3.4.10-bookworm`; Bundler install and Rake.

## Additional examples

- [custom-image](../assets/examples/custom-image.json): a complete config using the same user-owned local image as the mixed-language example. Build it from the [Dockerfile](../assets/images/node-go-tools/Dockerfile); registry availability is not claimed.
- [mixed-language](../assets/examples/mixed-language.json): one local image for a Node and Go monorepo.

## Registry metadata checked 2026-10-08

The following read-only registry metadata checks found both `linux/amd64` and `linux/arm64` manifests for each exact published tag. Digests identify the observed index and do not turn a mutable tag into a content pin. The check records availability only; no image was pulled or run.

| Preset/image | Observed index digest | Platforms |
| --- | --- | --- |
| base / `alpine:3.22` | `sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8` | amd64, arm64 |
| node-npm, node-pnpm, node-yarn-classic, node-yarn-berry / `node:22-bookworm` | `sha256:0e5f906573693feaa1e21057ebdcfdb5bd5021f050b2dc7c9deceb629c7da2a8` | amd64, arm64 |
| go / `golang:1.26.5` | `sha256:705e964a93a2fd2e75c7d59bb7d781b57e30f12293ffde5175c69229e18fb678` | amd64, arm64 |
| python-pip / `python:3.13-slim-bookworm` | `sha256:a1165e272e578941b84abc79e4ab38a0305cd12803a5c4247979ac7655f4d641` | amd64, arm64 |
| python-uv / `ghcr.io/astral-sh/uv:0.12.23-python3.13-trixie-slim` | `sha256:a6aeb5c166af9f765f9c68e585b5a5148c28f3b8a362f90151cecea88b21a3e2` | amd64, arm64 |
| java-maven / `maven:3.9.11-eclipse-temurin-21` | `sha256:6fdc855a6ed81d288ca7ca37ac6ff5e9308b612485c0801d70b25a858c83d237` | amd64, arm64 |
| java-gradle / `gradle:8.14-jdk21` | `sha256:c3dbf585165204a35635e0d6c4c15bc8f9b1effe787f18ded27805e1766992b5` | amd64, arm64 |
| dotnet / `mcr.microsoft.com/dotnet/sdk:9.0.318` | `sha256:1330d3c4144aec2695036a125d9a3b49a1fc555c9024c981248fa83d40fa8b23` | amd64, arm64 |
| rust / `rust:1.99.0-bookworm` | `sha256:114c7a4425406451c2866b6aafe69fe29b1b298832db1277d411ac73c82d04d6` | amd64, arm64 |
| php-composer / `composer:2.10.3` | `sha256:af98f42dfff7c68ba8d53c2164fd9fde1087b7d449514baa38c418b1f6bc4bac` | amd64, arm64 |
| ruby-bundler / `ruby:3.4.10-bookworm` | `sha256:cc183c94af21fe763dcf1a89f731da75797b80b399e1c4d0a9408bcd3f87c63c` | amd64, arm64 |

The exact tag pages used were [Node tags](https://hub.docker.com/_/node/tags), [Go tags](https://hub.docker.com/_/golang/tags), [Python tags](https://hub.docker.com/_/python/tags), [Maven tags](https://hub.docker.com/_/maven), [Gradle tags](https://hub.docker.com/_/gradle), [Rust tags](https://hub.docker.com/_/rust), [Composer image](https://hub.docker.com/_/composer), [Ruby tags](https://hub.docker.com/_/ruby), the [uv Docker guide](https://docs.astral.sh/uv/guides/integration/docker/), and [.NET 9 downloads](https://dotnet.microsoft.com/en-us/download/dotnet/9.0). The .NET SDK 9.0.318 servicing tag and Rust 1.99.0 supported tag were current explicit references at review time; servicing can move, so re-check before execution. These are availability records, not security audits or runtime compatibility claims.

For both custom-image and mixed-language, build the same local image before use:

```Dockerfile
FROM golang:1.26.5 AS go
FROM node:22-bookworm
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/go/bin:$PATH GOPATH=/go
```

```sh
docker build -t pitstop-local/node-go-tools:2026.10 -f assets/images/node-go-tools/Dockerfile .
```

Run that command from the installed bundle root, the directory containing `assets/images/node-go-tools/Dockerfile`. This local build is a prerequisite and has not been executed by this bundle task.
