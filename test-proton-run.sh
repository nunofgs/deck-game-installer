#!/usr/bin/env bash
# One-off test: launch a non-Steam game via the Proton wrapper script.
# This exercises the full Proton stack (pressure-vessel, GDK stubs, DXVK, etc.)
# rather than raw wine, to check whether game-runtime checks pass.
#
# Usage: ./test-proton-run.sh [/path/to/Game.exe]
# Defaults to Mixtape if no argument given.

set -euo pipefail

STEAM_DIR="$HOME/.local/share/Steam"
PROTON_DIR="$STEAM_DIR/steamapps/common/Proton - Experimental"

GAME_EXE="${1:-/home/nunofgs/Games/Mixtape-InsaneRamZes/Mixtape.exe}"

# Read the app ID directly from shortcuts.vdf so it matches exactly what Steam stored.
APP_ID=$(python3 - "$GAME_EXE" <<'EOF'
import sys, os, struct, ctypes

target_exe = sys.argv[1]
shortcuts_path = os.path.expanduser(
    "~/.local/share/Steam/userdata"
)

for uid in os.listdir(shortcuts_path):
    p = os.path.join(shortcuts_path, uid, "config", "shortcuts.vdf")
    if not os.path.exists(p):
        continue
    data = open(p, "rb").read()
    # Scan for the exe string, then walk back to find the appid field
    needle = ('"' + target_exe + '"').encode()
    idx = data.find(needle)
    if idx == -1:
        continue
    # appid field is a 4-byte little-endian int preceded by \x02appid\x00
    tag = b"\x02appid\x00"
    tag_idx = data.rfind(tag, 0, idx)
    if tag_idx == -1:
        continue
    raw = data[tag_idx + len(tag): tag_idx + len(tag) + 4]
    signed = struct.unpack("<i", raw)[0]
    print(ctypes.c_uint32(signed).value)
    sys.exit(0)

print("NOT_FOUND")
EOF
)

if [ "$APP_ID" = "NOT_FOUND" ]; then
    echo "Could not find shortcut for $GAME_EXE in shortcuts.vdf" >&2
    exit 1
fi

COMPAT_DATA="$STEAM_DIR/steamapps/compatdata/$APP_ID"

echo "Game:        $GAME_EXE"
echo "App ID:      $APP_ID"
echo "Compat data: $COMPAT_DATA"
echo "Proton:      $PROTON_DIR"
echo ""

mkdir -p "$COMPAT_DATA"

export WINEDLLOVERRIDES="xgameruntime=n,b"
export STEAM_COMPAT_DATA_PATH="$COMPAT_DATA"
export STEAM_COMPAT_CLIENT_INSTALL_PATH="$STEAM_DIR"
export STEAM_COMPAT_INSTALL_PATH="$(dirname "$GAME_EXE")"
export STEAM_COMPAT_LIBRARY_PATHS="$STEAM_DIR/steamapps"
export SteamAppId="$APP_ID"
export STEAM_COMPAT_APP_ID="$APP_ID"

exec "$PROTON_DIR/proton" run "$GAME_EXE"
