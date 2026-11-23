#!/bin/sh
# Install dashcrew from a GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/fahnaladitia/dashcrew/main/install.sh | sh
#
# DASHCREW_VERSION      the release to install (0.6.0 or v0.6.0); default: the latest final release.
# DASHCREW_INSTALL_DIR  where the binary goes; default: $HOME/.local/bin. No sudo is used.
# DASHCREW_RELEASES_URL the releases page to download from; default: the GitHub releases of dashcrew.
#
# The archive is installed only when checksums.txt lists it with the same SHA-256 and the binary reports the version
# asked for. There is no flag to skip either check.
set -eu

releases=${DASHCREW_RELEASES_URL:-https://github.com/fahnaladitia/dashcrew/releases}
install_dir=${DASHCREW_INSTALL_DIR:-${HOME:?HOME is not set; set DASHCREW_INSTALL_DIR}/.local/bin}
go_install="go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@latest"

fail() {
  printf 'dashcrew install: %s\n' "$1" >&2
  exit 1
}

# fetch URL FILE downloads URL into FILE; a missing page fails.
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    fail "needs curl or wget"
  fi
}

# latest_tag prints the tag the releases page's latest link redirects to, without the GitHub API.
latest_tag() {
  if command -v curl >/dev/null 2>&1; then
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$releases/latest")
  elif command -v wget >/dev/null 2>&1; then
    # -S prints each response's Location header, and wget adds its own "Location: <url> [following]" line: the
    # first word after the last one is where the redirects ended.
    url=$(wget -S -O /dev/null "$releases/latest" 2>&1 | tr -d '\r' | sed -n 's/^ *[Ll]ocation: *\([^ ]*\).*/\1/p' | tail -n 1)
  else
    fail "needs curl or wget"
  fi
  printf '%s\n' "${url##*/}"
}

# sha256 prints the SHA-256 of FILE.
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    fail "needs sha256sum or shasum to check the download"
  fi
}

case $(uname -s) in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "no install script for $(uname -s) yet; install with: $go_install" ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no release build for $(uname -m); install with: $go_install" ;;
esac

if [ -n "${DASHCREW_VERSION:-}" ]; then
  version=${DASHCREW_VERSION#v}
else
  tag=$(latest_tag) || fail "could not find the latest release at $releases/latest"
  version=${tag#v}
fi
case $version in
  *[!0-9A-Za-z.-]* | "") fail "\"$version\" is not a version such as 0.6.0" ;;
esac
if ! printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  fail "\"$version\" is not a version such as 0.6.0"
fi

archive="dashcrew_${version}_${os}_${arch}.tar.gz"
base="$releases/download/v$version"

tmp=$(mktemp -d)
staged="$install_dir/.dashcrew.install.$$"
trap 'rm -rf "$tmp"; rm -f "$staged"' EXIT
trap 'exit 1' HUP INT TERM

fetch "$base/$archive" "$tmp/$archive" || fail "could not download $archive from $base (is $version a release?)"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt from $base; nothing installed"

expected=$(awk -v name="$archive" '$2 == name {print $1; exit}' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "checksums.txt does not list $archive; nothing installed"
actual=$(sha256 "$tmp/$archive")
[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive (expected $expected, got $actual); nothing installed"

mkdir -p "$tmp/x"
tar -xzf "$tmp/$archive" -C "$tmp/x" dashcrew || fail "$archive holds no dashcrew binary"
reported=$("$tmp/x/dashcrew" --version 2>/dev/null) || reported=
[ "${reported##* }" = "$version" ] || fail "the binary in $archive reports \"$reported\", not $version; nothing installed"

# Copied next to its target, then renamed over it: a guard call never runs half a file.
mkdir -p "$install_dir"
cp "$tmp/x/dashcrew" "$staged"
chmod 0755 "$staged"
mv -f "$staged" "$install_dir/dashcrew"

printf 'Installed dashcrew %s to %s\n' "$version" "$install_dir/dashcrew"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *)
    # fish cannot read an export line; fish_add_path keeps the folder for every later session.
    case "${SHELL:-}" in
      */fish) printf '%s is not on PATH. Add it, for example by running once:\n  fish_add_path %s\n' "$install_dir" "$install_dir" ;;
      *) printf '%s is not on PATH. Add it, for example in your shell profile:\n  export PATH="%s:$PATH"\n' "$install_dir" "$install_dir" ;;
    esac
    ;;
esac
first=$(command -v dashcrew 2>/dev/null || true)
if [ -n "$first" ] && [ "$first" != "$install_dir/dashcrew" ]; then
  printf 'The dashcrew first on PATH is %s, not this one; the guard shim runs that one.\n' "$first"
fi
