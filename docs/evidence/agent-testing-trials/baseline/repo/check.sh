#!/bin/sh
set -eu
test "$(cat answer.txt)" = 42
printf "fixture assertion passed\n"
