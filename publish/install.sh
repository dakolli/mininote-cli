#!/usr/bin/env sh
#
# install.sh — install the mininote CLI from GitHub Releases.
#
# Fetches the prebuilt binary for the current OS/arch from the mirror repo
# github.com/dakolli/mininote-cli and installs it to INSTALL_DIR (default
# /usr/local/bin, falling back to ~/bin when not writable).
#
# Supported: linux + darwin, x86_64/amd64 + arm64/aarch64.
# Dependencies: curl (or wget), tar, and sh. That's it.
#
# Usage:
#   curl -sfL https://raw.githubusercontent.com/dakolli/mininote-cli/main/install.sh | sh
#   VERSION=v1.2.3 curl -sfL https://raw.githubusercontent.com/dakolli/mininote-cli/main/install.sh | sh
#   INSTALL_DIR="$HOME/bin" sh ./install.sh
#
# Env:
#   VERSION       release tag to install, e.g. "v1.2.3" (default: latest release)
#   INSTALL_DIR   directory to install the binary into (default: /usr/local/bin,
#                 or ~/bin if /usr/local/bin is not writable)

set -eu

REPO="dakolli/mininote-cli"
PROJECT="mininote"
BIN="mininote"

# --- detect OS/arch (unix only: linux + darwin) -------------------------------
os="$(uname -s)"
case "$os" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "error: unsupported OS '$os' — install.sh supports linux and darwin only" >&2
    exit 1
    ;;
esac

machine="$(uname -m)"
case "$machine" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture '$machine' — install.sh supports amd64 and arm64 only" >&2
    exit 1
    ;;
esac

# --- resolve the release tag ---------------------------------------------------
# GoReleaser names assets with the version WITHOUT the leading 'v'
# (mininote_1.2.3_linux_amd64.tar.gz) but serves them from the tag path WITH it
# (/releases/download/v1.2.3/...). Normalize both forms of VERSION here.
tag="${VERSION:-}"
if [ -z "$tag" ]; then
  echo "Resolving the latest release from GitHub..."
  if command -v curl >/dev/null 2>&1; then
    tag="$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)" \
      || true
  elif command -v wget >/dev/null 2>&1; then
    tag="$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)" \
      || true
  else
    echo "error: need curl or wget to resolve the latest release; set VERSION explicitly" >&2
    exit 1
  fi
  if [ -z "$tag" ]; then
    echo "error: could not resolve the latest release from GitHub; set VERSION explicitly, e.g. VERSION=v1.2.3" >&2
    exit 1
  fi
fi

case "$tag" in
  v*) version="${tag#v}" ;; # v1.2.3 -> 1.2.3 (asset name)
  *) version="$tag" ;;      # already bare: use as-is
esac

# --- download + verify ----------------------------------------------------------
asset="mininote_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$tag/$asset"

echo "Downloading $url"
tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/mininote-install.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

if command -v curl >/dev/null 2>&1; then
  curl -sfL "$url" -o "$tmpdir/$asset"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$url" -O "$tmpdir/$asset"
else
  echo "error: need curl or wget to download the release" >&2
  exit 1
fi

# Best-effort checksum verification when checksums.txt is published.
if command -v sha256sum >/dev/null 2>&1; then
  checksums_url="https://github.com/$REPO/releases/download/$tag/checksums.txt"
  expected="$(curl -sfL "$checksums_url" 2>/dev/null | grep "^[0-9a-f]*  $asset\$" | awk '{print $1}' | head -n1)" || true
  if [ -n "$expected" ]; then
    actual="$(sha256sum "$tmpdir/$asset" | awk '{print $1}')"
    if [ "$actual" != "$expected" ]; then
      echo "error: checksum mismatch for $asset" >&2
      echo "  expected: $expected" >&2
      echo "  actual:   $actual" >&2
      exit 1
    fi
    echo "Checksum OK"
  else
    echo "note: no checksums.txt for $tag — skipping checksum verification"
  fi
fi

# --- extract -------------------------------------------------------------------
tar -xzf "$tmpdir/$asset" -C "$tmpdir"
# The archive may or may not wrap the binary in a directory; locate it anywhere.
bin="$(find "$tmpdir" -type f -name "$BIN" | head -n1)"
if [ -z "$bin" ]; then
  echo "error: '$BIN' binary not found in $asset" >&2
  exit 1
fi

# --- install -------------------------------------------------------------------
if [ -n "${INSTALL_DIR:-}" ]; then
  installdir="$INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  installdir="/usr/local/bin"
else
  installdir="$HOME/bin"
  echo "note: /usr/local/bin is not writable — installing to $installdir"
fi

mkdir -p "$installdir"
install -m 0755 "$bin" "$installdir/$BIN"

echo
echo "Installed mininote ${version} to $installdir/$BIN"
echo "Run 'mininote --help' to get started (add $installdir to your PATH if needed)."
