#!/bin/sh
# ytb-sum installer for macOS and Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.sh | sh
#
# It installs the binary into a directory that is already on PATH when it can
# (e.g. /opt/homebrew/bin), otherwise into ~/.local/bin and adds that to your
# shell profile. Missing runtime dependencies (yt-dlp, ffmpeg) are installed
# with Homebrew, apt, dnf, pacman, apk or pipx when available.
#
# Environment:
#   YTB_SUM_VERSION      version to install, e.g. v0.1.0 (default: latest)
#   YTB_SUM_INSTALL_DIR  target directory (default: auto)
#   YTB_SUM_NO_DEPS=1    do not install yt-dlp / ffmpeg
#   YTB_SUM_NO_MODIFY_PATH=1  do not edit shell profiles
set -eu

REPO="antalkon/ytb-sum"
BIN="ytb-sum"

say()  { printf '\033[1;31m▶\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m✔\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }
has()  { command -v "$1" >/dev/null 2>&1; }

has curl || die "'curl' is required"
has tar || die "'tar' is required"

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

# --- download & verify -------------------------------------------------------

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
if has sha256sum; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for $archive"
tar -xzf "$tmp/$archive" -C "$tmp" "$BIN"

# --- pick install dir ----------------------------------------------------------

in_path() { case ":$PATH:" in *":$1:"*) return 0 ;; esac; return 1; }

dir="${YTB_SUM_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  # Prefer a writable directory that is already on PATH: works immediately.
  for d in /opt/homebrew/bin /usr/local/bin "$HOME/.local/bin" "$HOME/bin"; do
    if [ -d "$d" ] && [ -w "$d" ] && in_path "$d"; then dir="$d"; break; fi
  done
fi
[ -n "$dir" ] || dir="$HOME/.local/bin"
mkdir -p "$dir"
install -m 0755 "$tmp/$BIN" "$dir/$BIN" 2>/dev/null || { cp "$tmp/$BIN" "$dir/$BIN" && chmod 0755 "$dir/$BIN"; }
ok "Installed $dir/$BIN"

# Remove a copy from an earlier install elsewhere so the new one isn't shadowed.
for old in "$HOME/.local/bin/$BIN" /usr/local/bin/$BIN /opt/homebrew/bin/$BIN; do
  if [ "$old" != "$dir/$BIN" ] && [ -f "$old" ] && [ -w "$(dirname "$old")" ]; then
    rm -f "$old" && say "Removed old copy $old"
  fi
done

# --- PATH ----------------------------------------------------------------------

path_note=""
if ! in_path "$dir" && [ -z "${YTB_SUM_NO_MODIFY_PATH:-}" ]; then
  line="export PATH=\"$dir:\$PATH\""
  marker="# added by ytb-sum installer"
  shell_name=$(basename "${SHELL:-sh}")
  case "$shell_name" in
    zsh)  profiles="${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash) if [ "$os" = darwin ]; then profiles="$HOME/.bash_profile"; else profiles="$HOME/.bashrc"; fi ;;
    fish) profiles="" ;;
    *)    profiles="$HOME/.profile" ;;
  esac
  if [ "$shell_name" = fish ]; then
    fish -c "fish_add_path -U '$dir'" 2>/dev/null && ok "Added $dir to fish PATH"
  else
    for p in $profiles; do
      if ! grep -qs "$marker" "$p"; then
        printf '\n%s\n%s\n' "$marker" "$line" >>"$p"
        ok "Added $dir to PATH in $p"
      fi
    done
  fi
  path_note="$dir"
fi

# --- runtime dependencies ------------------------------------------------------

sudo_cmd=""
if [ "$(id -u)" -ne 0 ] && has sudo; then sudo_cmd="sudo"; fi

install_pkgs() { # $@: packages
  if has brew; then brew install "$@"
  elif has apt-get; then $sudo_cmd apt-get update -qq && $sudo_cmd apt-get install -y "$@"
  elif has dnf; then $sudo_cmd dnf install -y "$@"
  elif has pacman; then $sudo_cmd pacman -S --noconfirm "$@"
  elif has apk; then $sudo_cmd apk add "$@"
  else return 1
  fi
}

if [ -z "${YTB_SUM_NO_DEPS:-}" ]; then
  missing=""
  has yt-dlp || missing="yt-dlp"
  has ffmpeg || missing="$missing ffmpeg"
  if [ -n "$missing" ]; then
    say "Installing dependencies:$( [ -n "${missing%% *}" ] && printf ' %s' $missing )"
    # shellcheck disable=SC2086
    if ! install_pkgs $missing; then
      if ! has yt-dlp && has pipx; then pipx install yt-dlp || true; fi
    fi
  fi
  if has yt-dlp; then ok "yt-dlp $(yt-dlp --version 2>/dev/null)"; else warn "yt-dlp is required — install it: https://github.com/yt-dlp/yt-dlp#installation"; fi
  has ffmpeg || warn "ffmpeg not found — only needed for --audio"
fi

# --- done ----------------------------------------------------------------------

key_found() {
  [ -n "${OPENROUTER_API_KEY:-}" ] && return 0
  grep -qs OPENROUTER_API_KEY "$HOME/.zshrc" "$HOME/.zprofile" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile" && return 0
  [ "$os" = darwin ] && security find-generic-password -s ytb-sum >/dev/null 2>&1 && return 0
  return 1
}
if ! key_found; then
  warn "Set your API key (https://openrouter.ai/keys):  export OPENROUTER_API_KEY=sk-or-..."
fi
if [ -n "$path_note" ]; then
  say "Open a new terminal (or run: exec \$SHELL) and try: $BIN https://youtu.be/aircAruvnKk"
else
  say "Done. Try: $BIN https://youtu.be/aircAruvnKk"
fi
