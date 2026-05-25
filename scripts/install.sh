#!/bin/sh
set -eu

OWNER="${COTERM_GITHUB_OWNER:-coterm}"
REPO="${COTERM_GITHUB_REPO:-coterm}"
BIN_DIR="${COTERM_BIN_DIR:-$HOME/.local/bin}"
SKILL_DIR="${COTERM_SKILL_DIR:-$HOME/.codex/skills/coterm-shared-terminal}"

usage() {
  cat <<'USAGE'
Usage: install.sh

Environment:
  COTERM_GITHUB_OWNER  GitHub owner for release downloads (default: coterm)
  COTERM_GITHUB_REPO   GitHub repo for release downloads (default: coterm)
  COTERM_VERSION       Release tag to install (default: latest release)
  COTERM_BIN_DIR       Binary install directory (default: ~/.local/bin)
  COTERM_SKILL_DIR     Skill install directory (default: ~/.codex/skills/coterm-shared-terminal)
USAGE
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ "$#" -ne 0 ]; then
  usage >&2
  exit 2
fi

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "coterm install: missing required command: $1" >&2
    exit 1
  fi
}

need curl
need install
need tar
need sed
need mktemp

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "coterm install: unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "coterm install: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

version="${COTERM_VERSION:-}"
if [ -z "$version" ]; then
  version="$(
    curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
      | sed -n '1p'
  )"
fi

if [ -z "$version" ]; then
  echo "coterm install: could not determine latest release version" >&2
  exit 1
fi

asset="coterm_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$OWNER/$REPO/releases/latest/download/$asset"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/coterm-install.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT INT HUP TERM

archive="$tmp/$asset"
echo "Downloading $url"
curl -fsSL "$url" -o "$archive"
tar -xzf "$archive" -C "$tmp"

if [ ! -f "$tmp/coterm" ]; then
  echo "coterm install: archive did not contain coterm binary" >&2
  exit 1
fi

if [ ! -f "$tmp/skills/coterm-shared-terminal/SKILL.md" ]; then
  echo "coterm install: archive did not contain coterm skill" >&2
  exit 1
fi

mkdir -p "$BIN_DIR" "$SKILL_DIR"
install -m 0755 "$tmp/coterm" "$BIN_DIR/coterm"
install -m 0644 "$tmp/skills/coterm-shared-terminal/SKILL.md" "$SKILL_DIR/SKILL.md"

echo "Installed coterm to $BIN_DIR/coterm"
echo "Installed coterm skill to $SKILL_DIR/SKILL.md"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo
    echo "Add $BIN_DIR to PATH to run coterm from any shell."
    echo "For zsh, add this to ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    ;;
esac
