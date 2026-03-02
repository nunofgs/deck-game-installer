#!/bin/sh
set -e

REPO="nunofgs/deck-game-installer"
BIN_DIR="$HOME/.local/bin"
MENU_DIR="$HOME/.local/share/kio/servicemenus"
BASE_URL="https://github.com/$REPO/releases/latest/download"
SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd || echo "")"

echo "Installing deck-game-installer..."

mkdir -p "$BIN_DIR" "$MENU_DIR"

if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/deck-game-installer" ] && [ -f "$SCRIPT_DIR/install-with-steam.desktop" ]; then
  echo "(Installing from local files...)"
  cp "$SCRIPT_DIR/deck-game-installer" "$BIN_DIR/deck-game-installer"
  cp "$SCRIPT_DIR/install-with-steam.desktop" "$MENU_DIR/install-with-steam.desktop"
else
  curl -fsSL "$BASE_URL/deck-game-installer" -o "$BIN_DIR/deck-game-installer"
  curl -fsSL "$BASE_URL/install-with-steam.desktop" -o "$MENU_DIR/install-with-steam.desktop"
fi

chmod +x "$BIN_DIR/deck-game-installer"
chmod +x "$MENU_DIR/install-with-steam.desktop"

echo ""
echo "Done!"
echo "  Binary:       $BIN_DIR/deck-game-installer"
echo "  Service menu: $MENU_DIR/install-with-steam.desktop"
echo ""
echo "Right-click any ISO or EXE in Dolphin and choose 'Install with Steam'."
