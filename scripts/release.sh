#!/bin/sh
set -eu

die() { printf 'pitstop release: %s\n' "$*" >&2; exit 1; }
[ "$#" -eq 2 ] || die 'Usage: sh scripts/release.sh vMAJOR.MINOR.PATCH NEW_OUTPUT_DIRECTORY'
tag=$1
case "$tag" in ''|*[!v0-9.]*) die 'Invalid release tag';; esac
printf '%s\n' "$tag" | LC_ALL=C grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || die 'Invalid release tag'
case "$2" in '') die 'Output directory is required';; /*) dist=$2;; *) dist=$PWD/$2;; esac
[ ! -e "$dist" ] && [ ! -L "$dist" ] || die 'Output directory already exists; use a new directory'
source_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$source_dir"
commit=$(git rev-parse --verify HEAD)
mkdir "$dist" || die 'Cannot create output directory'
for platform in darwin linux; do
  for arch in arm64 amd64; do
    CGO_ENABLED=0 GOOS=$platform GOARCH=$arch go build -trimpath \
      -ldflags "-s -w -X pitstop/internal/cli.Version=${tag#v} -X pitstop/internal/cli.Commit=$commit" \
      -o "$dist/pitstop_${platform}_${arch}" ./cmd/pitstop
  done
done
cp scripts/install.sh "$dist/install.sh"
cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum install.sh pitstop_darwin_arm64 pitstop_darwin_amd64 pitstop_linux_arm64 pitstop_linux_amd64 > checksums.txt
else
  shasum -a 256 install.sh pitstop_darwin_arm64 pitstop_darwin_amd64 pitstop_linux_arm64 pitstop_linux_amd64 > checksums.txt
fi
printf 'Release %s (%s) built in %s\n' "$tag" "$commit" "$dist"
