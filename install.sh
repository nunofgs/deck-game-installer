#!/bin/sh
set -e

REPO="nunofgs/deck-game-installer"
BIN_DIR="$HOME/.local/bin"
MENU_DIR="$HOME/.local/share/kio/servicemenus"
BASE_URL="https://github.com/$REPO/releases/latest/download"

echo "Installing deck-game-installer..."

mkdir -p "$BIN_DIR" "$MENU_DIR"

curl -fsSL "$BASE_URL/deck-game-installer" -o "$BIN_DIR/deck-game-installer"
chmod +x "$BIN_DIR/deck-game-installer"

curl -fsSL "$BASE_URL/install-with-steam.desktop" -o "$MENU_DIR/install-with-steam.desktop"
chmod +x "$MENU_DIR/install-with-steam.desktop"

echo ""
echo "Done!"
echo "  Binary:       $BIN_DIR/deck-game-installer"
echo "  Service menu: $MENU_DIR/install-with-steam.desktop"
echo ""
echo "Right-click any ISO or EXE in Dolphin and choose 'Install with Steam'."
