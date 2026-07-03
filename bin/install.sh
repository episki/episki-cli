#!/usr/bin/env sh
# episki-cli installer.
#
# Usage:
#   curl -sSf https://cli.episki.com/install.sh | sh
#   curl -sSf https://cli.episki.com/install.sh | sh -s -- --version 0.3.1
#   curl -sSf https://cli.episki.com/install.sh | sh -s -- --force
#
# Honors:
#   EPISKI_INSTALL_DIR  Where to drop the binary. Default: $HOME/.local/bin.
set -eu

REPO="episki/episki-cli"
BIN="episki"

VERSION=""
FORCE="0"
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --force) FORCE="1"; shift ;;
    -h|--help)
      sed -n '2,12p' "$0"
      exit 0
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

INSTALL_DIR="${EPISKI_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$INSTALL_DIR"

uname_s="$(uname -s)"
uname_m="$(uname -m)"
# os/arch must match goreleaser's default {{ .Os }}/{{ .Arch }} archive
# naming exactly: lowercase GOOS and amd64/arm64.
case "$uname_s" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) echo "unsupported OS: $uname_s" >&2; exit 1 ;;
esac
case "$uname_m" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $uname_m" >&2; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
  VERSION="$(curl -sSfL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\(v[^"]*\)".*/\1/p' | head -n1)"
  if [ -z "$VERSION" ]; then
    echo "could not determine latest version; pass --version X.Y.Z" >&2
    exit 1
  fi
fi
case "$VERSION" in v*) ;; *) VERSION="v$VERSION" ;; esac

if [ "$FORCE" != "1" ] && command -v "$BIN" >/dev/null 2>&1; then
  current="$("$BIN" --version 2>/dev/null | awk '{print $NF}')" || true
  if [ -n "${current:-}" ] && [ "v${current#v}" = "$VERSION" ]; then
    echo "episki $VERSION already installed at $(command -v "$BIN"). Use --force to reinstall."
    exit 0
  fi
fi

archive="${BIN}_${VERSION#v}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$VERSION/$archive"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $url"
curl -sSfL "$url" -o "$tmp/$archive"
tar -xzf "$tmp/$archive" -C "$tmp"
mv "$tmp/$BIN" "$INSTALL_DIR/$BIN"
chmod +x "$INSTALL_DIR/$BIN"

echo "Installed $BIN $VERSION to $INSTALL_DIR/$BIN"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Note: $INSTALL_DIR is not on your PATH. Add it to your shell profile."
     echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
     ;;
esac
