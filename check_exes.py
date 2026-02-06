import vdf
import os
try:
    p = os.path.expanduser('~/.local/share/Steam/userdata')
    subdirs = [d for d in os.listdir(p) if d.isdigit()]
    if not subdirs:
        print("No userdata found")
        exit(0)
    s_path = os.path.join(p, subdirs[0], 'config/shortcuts.vdf')
    if not os.path.exists(s_path):
        print(f"Path not found: {s_path}")
        exit(0)
    with open(s_path, 'rb') as f:
        d = vdf.binary_load(f)
    shortcuts = d.get('shortcuts', {})
    for k, v in shortcuts.items():
        print(f"Idx {k}: AppName='{v.get('AppName')}', Exe='{v.get('Exe')}', AppID={v.get('appid')}")
except Exception as e:
    print(f"Error: {e}")
