#!/bin/sh
set -eu
command -v pitstop_fixture_tool_not_installed || { printf "missing fixture tool\n" >&2; exit 127; }
