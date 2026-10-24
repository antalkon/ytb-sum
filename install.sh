#!/bin/sh
# ytb-sum installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.sh | sh
#
# Environment:
#   YTB_SUM_VERSION      version to install, e.g. v0.1.0 (default: latest)
#   YTB_SUM_INSTALL_DIR  target directory (default: /usr/local/bin if writable, else ~/.local/bin)
set -eu

REPO="antalkon/ytb-sum"
BIN="ytb-sum"

say()  { printf '\033[1;31m▶\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "'$1' is required"; }
need curl
need tar
need uname

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin|linux) ;;
  *) die "unsupported OS: $os (on Windows use install.ps1)" ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

version="${YTB_SUM_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's#.*/tag/##')
  [ -n "$version" ] && [ "$version" != "latest" ] || die "could not determine the latest release"
fi
num="${version#v}"
archive="${BIN}_${num}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "Downloading $BIN $version ($os/$arch)"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || die "download failed: $base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "could not download checksums"

expected=$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || die "no checksum for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" "$BIN"

dir="${YTB_SUM_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ]; then dir=/usr/local/bin; else dir="$HOME/.local/bin"; fi
fi
mkdir -p "$dir"
install -m 0755 "$tmp/$BIN" "$dir/$BIN" 2>/dev/null || { cp "$tmp/$BIN" "$dir/$BIN" && chmod 0755 "$dir/$BIN"; }
say "Installed $dir/$BIN"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) warn "$dir is not in PATH — add: export PATH=\"$dir:\$PATH\"" ;;
esac

if ! command -v yt-dlp >/dev/null 2>&1; then
  if [ "$os" = darwin ]; then hint="brew install yt-dlp ffmpeg"; else hint="pipx install yt-dlp  (or your package manager)"; fi
  warn "yt-dlp is required at runtime: $hint"
fi
if [ -z "${OPENROUTER_API_KEY:-}" ]; then
  warn "set your key: export OPENROUTER_API_KEY=sk-or-...  (https://openrouter.ai/keys)"
fi
say "Done. Try: $BIN https://youtu.be/aircAruvnKk"
