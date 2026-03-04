#!/bin/sh
set -e

REPO="nunofgs/deck-game-installer"
BIN_DIR="$HOME/.local/bin"
MENU_DIR="$HOME/.local/share/kio/servicemenus"
RELEASE_URL="https://github.com/$REPO/releases/latest/download/deck-game-installer.zip"
SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd || echo "")"

echo "Installing deck-game-installer..."

mkdir -p "$BIN_DIR" "$MENU_DIR"

# If the required files are present next to this script (e.g. when running from an
# extracted zip), install them directly. Otherwise download the zip from the latest
# GitHub release and extract it (e.g. when piped via curl).
if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/deck-game-installer" ] && [ -f "$SCRIPT_DIR/install-with-steam.desktop" ]; then
  echo "(Installing from local files...)"
  cp "$SCRIPT_DIR/deck-game-installer" "$BIN_DIR/deck-game-installer"
  cp "$SCRIPT_DIR/install-with-steam.desktop" "$MENU_DIR/install-with-steam.desktop"
else
  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT
  curl -fsSL "$RELEASE_URL" -o "$TMP_DIR/deck-game-installer.zip"
  unzip -q "$TMP_DIR/deck-game-installer.zip" -d "$TMP_DIR"
  cp "$TMP_DIR/deck-game-installer" "$BIN_DIR/deck-game-installer"
  cp "$TMP_DIR/install-with-steam.desktop" "$MENU_DIR/install-with-steam.desktop"
fi

chmod +x "$BIN_DIR/deck-game-installer"
chmod +x "$MENU_DIR/install-with-steam.desktop"

echo ""
echo "Done!"
echo "  Binary:       $BIN_DIR/deck-game-installer"
echo "  Service menu: $MENU_DIR/install-with-steam.desktop"
echo ""
echo "Right-click any ISO or EXE in Dolphin and choose 'Install with Steam'."
