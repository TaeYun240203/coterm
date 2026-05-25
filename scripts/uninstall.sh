#!/bin/sh
set -eu

BIN_DIR="${COTERM_BIN_DIR:-$HOME/.local/bin}"
BIN="${COTERM_BIN:-$BIN_DIR/coterm}"
SKILL_DIR="${COTERM_SKILL_DIR:-$HOME/.codex/skills/coterm-shared-terminal}"
PURGE=0

usage() {
  cat <<'USAGE'
Usage: uninstall.sh [--purge]

Environment:
  COTERM_BIN        Installed coterm binary path (default: ~/.local/bin/coterm)
  COTERM_BIN_DIR    Binary install directory used when COTERM_BIN is unset
  COTERM_SKILL_DIR  Skill install directory (default: ~/.codex/skills/coterm-shared-terminal)
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --purge)
      PURGE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

if [ -x "$BIN" ]; then
  if [ "$PURGE" -eq 1 ]; then
    "$BIN" uninstall --purge
  else
    "$BIN" uninstall
  fi
  exit $?
fi

rm -f "$BIN"
rm -rf "$SKILL_DIR"

if [ "$PURGE" -eq 1 ]; then
  rm -rf "$HOME/.cache/coterm" "$HOME/.local/state/coterm" "$HOME/.config/coterm"
fi

echo "Removed coterm binary path: $BIN"
echo "Removed coterm skill path: $SKILL_DIR"
