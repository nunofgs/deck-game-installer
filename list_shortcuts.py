import vdf
import os
import struct
import sys

def get_shortcuts():
    p = os.path.expanduser('~/.local/share/Steam/userdata')
    if not os.path.exists(p): return {}
    sds = [d for d in os.listdir(p) if d.isdigit()]
    if not sds: return {}
    # Sort to get a consistent one, or check all
    for sd in sds:
        sp = os.path.join(p, sd, 'config/shortcuts.vdf')
        if os.path.exists(sp):
            with open(sp, 'rb') as f:
                d = vdf.binary_load(f)
                items = d.get('shortcuts', {})
                for k, v in items.items():
                    appid = v.get('appid')
                    # Get the 64-bit ID for URL
                    u32 = struct.unpack('<I', struct.pack('<i', appid))[0] if appid < 0 else appid
                    u64 = (u32 << 32) | 0x02000000
                    print(f"SD: {sd} | Name: {v.get('AppName')} | Exe: {v.get('Exe')} | AppID: {appid} | URL_ID: {u64}")

get_shortcuts()
