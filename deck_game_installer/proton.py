import os
import re
import struct
from pathlib import Path
from typing import List, Tuple, Optional

class ProtonManager:
    def __init__(self):
        self.steam_path = Path.home() / ".local/share/Steam"
        self.common_path = self.steam_path / "steamapps/common"
        self.compat_tools_path = self.steam_path / "compatibilitytools.d"
        self.compat_data_path = self.steam_path / "steamapps/compatdata"

    def get_available_proton_versions(self) -> List[str]:
        """Returns a list of available Proton version internal names."""
        versions = []
        
        # Scan common/ for Steam-installed Proton
        if self.common_path.exists():
            for d in self.common_path.iterdir():
                if d.is_dir() and d.name.lower().startswith("proton"):
                    # Convert folder name to internal name: "Proton - Experimental" -> "proton-experimental"
                    internal_name = self._folder_to_internal_name(d.name)
                    versions.append(internal_name)
        
        # Scan compatibilitytools.d/ for custom Proton (e.g., Proton-GE)
        if self.compat_tools_path.exists():
            for d in self.compat_tools_path.iterdir():
                if d.is_dir():
                    # For custom tools, check for compatibilitytool.vdf to get internal name
                    vdf_path = d / "compatibilitytool.vdf"
                    if vdf_path.exists():
                        internal_name = self._get_internal_name_from_vdf(vdf_path)
                        if internal_name:
                            versions.append(internal_name)
                    else:
                        # Fallback to folder name conversion
                        versions.append(self._folder_to_internal_name(d.name))

        # Sort: Experimental first, then others descending
        def sort_key(v):
            if "experimental" in v.lower():
                return (0, "")
            # Extract version numbers if possible
            version_match = re.search(r"(\d+\.\d+)", v)
            if version_match:
                return (1, version_match.group(1))
            return (2, v)

        return sorted(list(set(versions)), key=sort_key)

    @staticmethod
    def _folder_to_internal_name(folder_name: str) -> str:
        """Converts a Proton folder name to its Steam internal name.
        
        Examples:
            'Proton - Experimental' -> 'proton-experimental'
            'Proton 10.0' -> 'proton_10'
            'Proton 9.0' -> 'proton_9'
        """
        name_lower = folder_name.lower()
        if "experimental" in name_lower:
            return "proton-experimental"
        # Extract version number for stable releases
        match = re.search(r"proton\s*(\d+)(?:\.\d+)?", name_lower)
        if match:
            return f"proton_{match.group(1)}"
        # Fallback: convert spaces/dashes to underscores
        return name_lower.replace(" - ", "-").replace(" ", "_")

    def _get_internal_name_from_vdf(self, vdf_path: Path) -> Optional[str]:
        """Extracts the internal tool name from a compatibilitytool.vdf file."""
        try:
            import vdf
            with open(vdf_path, "r") as f:
                data = vdf.load(f)
            compat_tools = data.get("compatibilitytools", {}).get("compat_tools", {})
            # Return the first key (the internal name)
            if compat_tools:
                return next(iter(compat_tools.keys()))
        except Exception:
            pass
        return None

    def scan_prefix_for_executables(self, app_id: int) -> List[Tuple[str, Path]]:
        # Steam uses unsigned 32-bit AppID strings for the prefix directory name
        u32_unsigned = struct.unpack('<I', struct.pack('<i', app_id))[0] if app_id < 0 else app_id
        prefix_path = self.compat_data_path / str(u32_unsigned) / "pfx/drive_c"
        if not prefix_path.exists():
            return []

        executables = []
        exclude_dirs = {"windows", "common files", "internet explorer", "steam", "users"}
        exclude_patterns = [
            r"unins.*\.exe", r"uninst.*\.exe", 
            r"vcredist.*\.exe", r"directx.*\.exe", r"dxsetup\.exe",
            r"setup\.exe", r"install\.exe", r"installer\.exe",
            r".*redist.*\.exe", r".*crash.*reporter.*\.exe"
        ]

        for root, dirs, files in os.walk(prefix_path):
            # Prune excluded directories
            rel_root = Path(root).relative_to(prefix_path)
            if any(part.lower() in exclude_dirs for part in rel_root.parts):
                continue

            for file in files:
                if file.lower().endswith(".exe"):
                    # Check against exclude patterns
                    if any(re.match(pattern, file, re.IGNORECASE) for pattern in exclude_patterns):
                        continue
                    
                    full_path = Path(root) / file
                    # Generate a display name like "game.exe (Program Files/GameName)"
                    try:
                        display_rel = full_path.relative_to(prefix_path)
                        display_name = f"{file} ({display_rel.parent})"
                    except ValueError:
                        display_name = file
                        
                    executables.append((display_name, full_path))

        # Sort by modification time (newest first)
        executables.sort(key=lambda x: x[1].stat().st_mtime, reverse=True)
        return executables

    @staticmethod
    def to_windows_path(pfx_path: Path, full_path: Path) -> str:
        """Converts a Linux path within a prefix to a Windows-style path (C:\\...)"""
        try:
            rel = full_path.relative_to(pfx_path)
            return "C:\\" + str(rel).replace("/", "\\")
        except ValueError:
            return str(full_path)
