import vdf
import os
import sys

def debug(msg):
    print(f"[DEBUG] {msg}", file=sys.stderr)

try:
    p = os.path.expanduser('~/.local/share/Steam/userdata')
    debug(f"Userdata path: {p}")
    if not os.path.exists(p):
        debug("Userdata path does not exist")
        exit(1)
        
    subdirs = [d for d in os.listdir(p) if d.isdigit()]
    debug(f"Found subdirs: {subdirs}")
    if not subdirs:
        debug("No numeric subdirs found in userdata")
        exit(1)
        
    for subdir in subdirs:
        s_path = os.path.join(p, subdir, 'config/shortcuts.vdf')
        debug(f"Checking: {s_path}")
        if os.path.exists(s_path):
            debug(f"File exists: {s_path}")
            with open(s_path, 'rb') as f:
                d = vdf.binary_load(f)
            shortcuts = d.get('shortcuts', {})
            debug(f"Found {len(shortcuts)} shortcuts in {subdir}")
            for k, v in shortcuts.items():
                print(f"Subdir {subdir} | Idx {k}: AppName='{v.get('AppName')}', Exe='{v.get('Exe')}', AppID={v.get('appid')}")
        else:
            debug(f"File does not exist: {s_path}")
except Exception as e:
    debug(f"Exception: {e}")
