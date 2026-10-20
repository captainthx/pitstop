#!/bin/sh
# Install only into a directory owned and controlled by the current user.
set -eu
umask 077

die() { printf 'pitstop installer: %s\n' "$*" >&2; exit 1; }
usage() {
  cat <<'EOF'
Usage: sh install.sh [--version vMAJOR.MINOR.PATCH] [--bin-dir DIRECTORY] [--force]
Defaults: latest published stable release; $HOME/.local/bin.
--force permits replacing an existing regular file. Symlinks are never replaced.
Requires curl, SHA-256 (sha256sum or shasum), macOS/Linux on arm64/amd64.
EOF
}
version=latest
bin_dir=${HOME:+$HOME/.local/bin}
force=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version|--bin-dir)
      option=$1
      [ "$#" -ge 2 ] && [ -n "$2" ] || die "$option requires a value"
      case "$option" in --version) version=$2;; --bin-dir) bin_dir=$2;; esac
      shift 2;;
    --force) force=true; shift;;
    --help|-h) usage; exit 0;;
    *) die "Unknown argument: $1";;
  esac
done

valid_version() {
  case "$1" in ''|*[!v0-9.]*) return 1;; esac
  printf '%s\n' "$1" | LC_ALL=C grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
}
[ -n "$bin_dir" ] || die 'HOME is unset; supply --bin-dir'
[ "$version" = latest ] || valid_version "$version" || die 'Invalid version; use vMAJOR.MINOR.PATCH'
case "$(uname -s)" in Darwin) platform=darwin;; Linux) platform=linux;; *) die 'Unsupported operating system';; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64;; x86_64|amd64) arch=amd64;; *) die 'Unsupported architecture';; esac
command -v curl >/dev/null 2>&1 || die 'curl is required'
if command -v sha256sum >/dev/null 2>&1; then
  hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  hash_tool=shasum
else
  die 'SHA-256 requires sha256sum or shasum'
fi

# Override user curl config: never silently weaken TLS or redirect policy.
fetch() {
  curl --disable --fail --silent --show-error --location \
    --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 180 "$@"
}
repo=https://github.com/captainthx/pitstop
if [ "$version" = latest ]; then
  resolved=$(fetch --head --output /dev/null --write-out '%{url_effective}' "$repo/releases/latest") || die 'Cannot resolve latest release'
  case "$resolved" in "$repo/releases/tag/"*) version=${resolved#"$repo/releases/tag/"};; *) die 'Unexpected latest release URL';; esac
  valid_version "$version" || die 'Invalid latest release version'
fi

case "$bin_dir" in /*) ;; *) bin_dir=$PWD/$bin_dir;; esac
target=$bin_dir/pitstop
check_target() {
  if [ -L "$target" ] || { [ -e "$target" ] && [ ! -f "$target" ]; }; then
    die 'Destination must be a regular file, never a symlink or directory'
  fi
  if [ -e "$target" ] && [ "$force" != true ]; then
    die 'Destination exists; review it and use --force to replace it'
  fi
}
check_target
tmp=
stage=
cleanup() {
  [ -z "$stage" ] || rm -f "$stage"
  [ -z "$tmp" ] || rm -rf "$tmp"
  :
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
tmp=$(mktemp -d "${TMPDIR:-/tmp}/pitstop-install.XXXXXXXX") || die 'Cannot create private download directory'
asset=pitstop_${platform}_${arch}
base=$repo/releases/download/$version
fetch --output "$tmp/checksums.txt" "$base/checksums.txt" || die 'checksum download failed'
fetch --output "$tmp/binary" "$base/$asset" || die 'binary download failed'
expected=$(LC_ALL=C awk -v asset="$asset" '
  $2 == asset {
    count++
    if (NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-fA-F]/) bad=1
    digest=tolower($1)
  }
  END { if (count != 1 || bad) exit 1; print digest }
' "$tmp/checksums.txt") || die 'Invalid or missing unique checksum entry'
if [ "$hash_tool" = sha256sum ]; then
  actual=$(sha256sum "$tmp/binary") || die 'Cannot calculate checksum'
else
  actual=$(shasum -a 256 "$tmp/binary") || die 'Cannot calculate checksum'
fi
[ "${actual%% *}" = "$expected" ] || die 'binary checksum mismatch'
mkdir -p "$bin_dir" || die 'Cannot create install directory'
stage=$(mktemp "$bin_dir/.pitstop-install.XXXXXXXX") || die 'Cannot stage executable in install directory'
cat "$tmp/binary" > "$stage" || die 'Cannot write staged executable'
chmod 755 "$stage" || die 'Cannot mark staged executable executable'
# The download directory may be mounted noexec; validate on the destination filesystem.
reported=$("$stage" version) || die 'Downloaded binary version check failed; check OS compatibility'
[ "$reported" = "pitstop ${version#v}" ] || die 'Downloaded binary version does not match release'
check_target
mv -f "$stage" "$target" || die 'Cannot replace executable'
stage=
printf 'Installed Pitstop %s to %s\n' "$version" "$target"
case ":${PATH:-}:" in
  *":$bin_dir:"*) ;;
  *) printf 'Add the install directory to PATH in your shell configuration: %s\n' "$bin_dir";;
esac
