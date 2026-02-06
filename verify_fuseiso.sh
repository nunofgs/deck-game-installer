#!/bin/bash
LOG="verify_fuseiso.log"
exec > "$LOG" 2>&1

echo "Finding ISO..."
ISO=$(find /run/user/1000 -name "*.iso" -print -quit)
if [ -z "$ISO" ]; then
  echo "ISO not found in /run/user/1000/"
  # Debug fallback path if find fails
  exit 1
fi
echo "Found ISO: $ISO"

MNT=$(mktemp -d)
echo "Mounting to $MNT using fuseiso..."

# Explicitly call the binary
/home/nunofgs/.local/bin/fuseiso -p "$ISO" "$MNT"
RES=$?

if [ $RES -eq 0 ]; then
  echo "Mount Command Success. Waiting 2s for FUSE..."
  sleep 2
  echo "Listing contents:"
  ls -la "$MNT"
  fusermount -u "$MNT"
else
  echo "Mount Failed with code $RES"
fi

rmdir "$MNT" 2>/dev/null
