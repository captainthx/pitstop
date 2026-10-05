#!/bin/sh
set -eu
bin=${PITSTOP_BIN:-.pitstop-work/bin/pitstop}
"$bin" version
"$bin" version --json | go run ./scripts/validate-json.go
"$bin" doctor --json || true
"$bin" list --json || true
