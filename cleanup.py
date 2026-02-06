import os
files = ['find_appid.py', 'find_appid_out.txt', 'find_appid_v2.py', 'appid_v2_out.txt', 
         'debug_vdf.py', 'debug_simple.py', 'find_shortcuts.py', 'list_all.py', 'list_out.txt',
         'final_out.txt', 'shortcuts_content.txt', 'appid_calc.txt', 'fix_shortcuts.py', 
         'fix_out.txt', 'find_app_name.py', 'debug_shortcuts_final.py']
for f in files:
    if os.path.exists(f):
        os.remove(f)
