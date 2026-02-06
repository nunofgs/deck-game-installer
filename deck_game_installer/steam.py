import os
import struct
import vdf
from pathlib import Path
from typing import Optional, Dict, Any, List

class SteamManager:
    def __init__(self):
        self.steam_path = Path.home() / ".local/share/Steam"
        self.userdata_path = self.steam_path / "userdata"
        self.user_ids = self._find_user_ids()
        self.user_id = self.user_ids[0] if self.user_ids else "0"

    def _find_user_ids(self) -> list:
        if not self.userdata_path.exists():
            return []
        # Return all numeric subdirs
        return [d.name for d in self.userdata_path.iterdir() if d.is_dir() and d.name.isdigit()]

    def get_shortcuts_path(self) -> Path:
        return self.userdata_path / self.user_id / "config/shortcuts.vdf"

    def get_config_path(self) -> Path:
        """Returns the path to the global config.vdf where compatibility settings are stored."""
        return self.steam_path / "config/config.vdf"

    @staticmethod
    def generate_app_id(exe_path: str, app_name: str) -> int:
        """Generates the 32-bit AppID used by Steam for non-Steam games."""
        key = f"{exe_path}{app_name}"
        from zlib import crc32
        u32 = (crc32(key.encode('utf-8')) | 0x80000000) & 0xFFFFFFFF
        return struct.unpack('<i', struct.pack('<I', u32))[0]
    
    @staticmethod
    def get_url_app_id_from_u32(u32: int) -> str:
        """Generates the 64-bit AppID string for steam:// URLs from a 32-bit AppID."""
        # Ensure it's treated as unsigned for the shift. 
        # Steam stores it as signed 32-bit in binary VDF.
        u32_unsigned = struct.unpack('<I', struct.pack('<i', u32))[0] if u32 < 0 else u32
        u64 = (u32_unsigned << 32) | 0x02000000
        return str(u64)

    def find_app_id_by_path(self, exe_path: str) -> Optional[int]:
        """Finds the 32-bit AppID for an existing shortcut by searching all userdata directories."""
        target_exe = f'"{exe_path}"'
        # Also try without quotes just in case
        target_exe_alt = exe_path
        
        for uid in self.user_ids:
            sp = self.userdata_path / uid / "config/shortcuts.vdf"
            if not sp.exists(): continue
            try:
                with open(sp, "rb") as f:
                    shortcuts = vdf.binary_load(f)
                items = shortcuts.get("shortcuts", {})
                for item in items.values():
                    if item.get("Exe") == target_exe or item.get("Exe") == target_exe_alt:
                        return item.get("appid")
            except Exception:
                continue
        return None

    def add_shortcut(self, app_name: str, exe_path: str, arguments: str = "", start_dir: str = "") -> int:
        """Adds a non-Steam game shortcut directly to shortcuts.vdf."""
        sp = self.get_shortcuts_path()
        shortcuts = {"shortcuts": {}}
        
        if sp.exists():
            try:
                with open(sp, "rb") as f:
                    shortcuts = vdf.binary_load(f)
            except Exception:
                pass

        if "shortcuts" not in shortcuts:
            shortcuts["shortcuts"] = {}

        # Check if already exists
        target_exe = f'"{exe_path}"'
        for entry in shortcuts["shortcuts"].values():
            if entry.get("Exe") == target_exe or entry.get("Exe") == exe_path:
                return entry.get("appid")

        # Create new entry
        appid = self.generate_app_id(exe_path, app_name)
        new_index = str(len(shortcuts["shortcuts"]))
        
        shortcuts["shortcuts"][new_index] = {
            "appid": appid,
            "AppName": app_name,
            "Exe": target_exe,
            "StartDir": start_dir or str(Path(exe_path).parent),
            "icon": "",
            "ShortcutPath": "",
            "LaunchOptions": arguments,
            "IsHidden": 0,
            "AllowDesktopConfig": 1,
            "AllowOverlay": 1,
            "OpenVR": 0,
            "Devkit": 0,
            "DevkitGameID": "",
            "DevkitOverrideAppID": 0,
            "LastPlayTime": 0,
            "FlatpakAppID": "",
            "sortas": "",
            "tags": {}
        }

        # Backup and Save
        if sp.exists():
            import shutil
            shutil.copy2(sp, sp.with_suffix(".vdf.bak"))

        with open(sp, "wb") as f:
            vdf.binary_dump(shortcuts, f)

        return appid

    def get_proton_version(self, app_id_32: int) -> Optional[str]:
        """Gets the currently configured Proton version for a specific AppID, if any."""
        path = self.get_config_path()
        if not path.exists():
            return None

        try:
            with open(path, "r") as f:
                config = vdf.load(f)
            
            mapping = config.get('InstallConfigStore', {}).get('Software', {}).get('Valve', {}).get('Steam', {}).get('CompatToolMapping', {})
            
            # Convert signed to unsigned AppID
            u32_unsigned = struct.unpack('<I', struct.pack('<i', app_id_32))[0] if app_id_32 < 0 else app_id_32
            
            entry = mapping.get(str(u32_unsigned))
            if entry:
                return entry.get('name')
        except Exception:
            pass
        return None

    def set_proton_version(self, app_id_32: int, proton_version: str):
        """Sets the Proton version for a specific AppID in config.vdf."""
        path = self.get_config_path()
        if not path.exists():
            return

        with open(path, "r") as f:
            config = vdf.load(f)

        # Path: InstallConfigStore/Software/Valve/Steam/CompatToolMapping
        try:
            mapping = config['InstallConfigStore']['Software']['Valve']['Steam']['CompatToolMapping']
        except KeyError:
            # Create the path if it doesn't exist
            if 'InstallConfigStore' not in config: config['InstallConfigStore'] = {}
            if 'Software' not in config['InstallConfigStore']: config['InstallConfigStore']['Software'] = {}
            if 'Valve' not in config['InstallConfigStore']['Software']: config['InstallConfigStore']['Software']['Valve'] = {}
            if 'Steam' not in config['InstallConfigStore']['Software']['Valve']: config['InstallConfigStore']['Software']['Valve']['Steam'] = {}
            if 'CompatToolMapping' not in config['InstallConfigStore']['Software']['Valve']['Steam']: 
                config['InstallConfigStore']['Software']['Valve']['Steam']['CompatToolMapping'] = {}
            mapping = config['InstallConfigStore']['Software']['Valve']['Steam']['CompatToolMapping']

        # Steam expects unsigned 32-bit AppID strings in CompatToolMapping
        u32_unsigned = struct.unpack('<I', struct.pack('<i', app_id_32))[0] if app_id_32 < 0 else app_id_32
        
        mapping[str(u32_unsigned)] = {
            'name': proton_version,
            'config': '',
            'priority': '250'
        }

        with open(path, "w") as f:
            vdf.dump(config, f)

    def restart_steam(self):
        """Attempts to restart Steam to apply config changes."""
        import subprocess
        # Try to kill Steam
        subprocess.run(["pkill", "-x", "steam"], capture_output=True)
        # Give it a moment to die
        import time
        time.sleep(3)
        # Relaunch Steam
        subprocess.Popen(["steam"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


