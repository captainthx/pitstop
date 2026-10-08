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
- [python-uv](../assets/examples/python-uv.json): `ghcr.io/astral-sh/uv:python3.13-trixie-slim`; locked uv sync and uv-run pytest.
- [java-maven](../assets/examples/java-maven.json): `maven:3.9.11-eclipse-temurin-21`; checked-in Maven wrapper test.
- [java-gradle](../assets/examples/java-gradle.json): `gradle:8.14-jdk21`; checked-in Gradle wrapper test.
- [dotnet](../assets/examples/dotnet.json): `mcr.microsoft.com/dotnet/sdk:9.0.305`; restore and test a project or solution.
- [rust](../assets/examples/rust.json): `rust:1.88.0-bookworm`; locked test and build.
- [php-composer](../assets/examples/php-composer.json): `composer:2.10.3`; Composer install and project test script.
- [ruby-bundler](../assets/examples/ruby-bundler.json): `ruby:3.4.10-bookworm`; Bundler install and Rake.

## Additional examples

- [custom-image](../assets/examples/custom-image.json): a complete config showing a user-owned local image prerequisite. Build it from the recipe below; registry availability is not claimed.
- [mixed-language](../assets/examples/mixed-language.json): one local image for a Node and Go monorepo.

The selected references were checked against primary vendor pages: [Node tags](https://hub.docker.com/_/node/tags), [Node Corepack](https://nodejs.org/docs/latest-v22.x/api/corepack.html), [uv Docker integration](https://docs.astral.sh/uv/guides/integration/docker/), [Python tags](https://hub.docker.com/_/python/tags), [Rust tags](https://hub.docker.com/_/rust), [Maven tags](https://hub.docker.com/_/maven), [Gradle tags](https://hub.docker.com/_/gradle), [Composer image](https://hub.docker.com/_/composer), [Ruby tags](https://hub.docker.com/_/ruby), and [.NET container images](https://learn.microsoft.com/en-us/dotnet/core/docker/container-images). These pages expose platform matrices that vary by tag; re-check the vendor registry and available `linux/amd64`/`linux/arm64` platforms before execution.

For the mixed-language example, build a local image before use:

```Dockerfile
FROM golang:1.26.5 AS go
FROM node:22-bookworm
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/usr/local/go/bin:/go/bin:$PATH GOPATH=/go
```

This local build is a prerequisite and has not been executed by this bundle task.
