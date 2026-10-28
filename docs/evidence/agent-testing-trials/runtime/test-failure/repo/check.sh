#!/bin/sh
set -eu
test "$(cat answer.txt)" = 99 || { printf "fixture assertion failed: expected 99\n" >&2; exit 7; }
