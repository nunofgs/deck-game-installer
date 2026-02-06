Deck Game Installer - Technical Specification
Project Overview
A Python tool for Steam Deck that automates installing Windows games into Steam with Proton support. Users right-click ISO or EXE files in Dolphin file manager and the tool handles mounting, installation, executable detection, and Steam shortcut creation.
System Requirements

Platform: Steam Deck (SteamOS) / Linux with KDE Plasma
Python: 3.10+
Dependencies:

click (CLI framework)
vdf (Steam VDF file parsing)
pyxdg (XDG directory handling)


System utilities: kdialog, udisksctl, steam (pre-installed on Steam Deck), steamos-add-to-steam (SteamOS)

Project Goals

Enable right-click context menu in Dolphin for ISO and EXE files
Automate mounting ISOs and finding installers
Add games to Steam with configurable Proton versions
Scan Proton prefix for game executables after installation
Update Steam shortcuts with final game executable

Workflows
ISO File Workflow

User right-clicks ISO in Dolphin, selects "Install into Steam"
Mount ISO using udisksctl
Scan mounted ISO for executables
Auto-detect common installers (setup.exe, install.exe, installer.exe, autorun.exe)
Present radio list of found executables with suggested installer at top
Include "Browse for another executable..." option
Add selected installer to Steam using steamos-add-to-steam (no Steam restart)
Launch installer via Steam (steam://rungameid/APPID)
Show dialog: "Press OK when installation is complete"
Scan Proton prefix (compatdata/APPID/pfx/drive_c/) for game executables
Filter out uninstallers, redistributables, system files
Present radio list of found executables
Present dropdown of available Proton versions (Experimental at top)
User selects game executable and Proton version
Update Steam shortcut with game executable and working directory
Set Proton version in Steam config
Unmount ISO
Show success dialog

EXE File Workflow

User right-clicks EXE in Dolphin, selects "Install into Steam"
Add EXE to Steam using steamos-add-to-steam (no Steam restart)
Launch installer via Steam
Show dialog: "Press OK when installation is complete"
Scan Proton prefix for game executables
Filter out uninstallers, redistributables, system files
Present radio list of found executables
Present dropdown of available Proton versions
User selects game executable and Proton version
Update Steam shortcut with game executable and working directory
Set Proton version in Steam config
Show success dialog

Component Specifications
1. CLI Module (cli.py)
Purpose: Entry point for the application
Interface:
pythondeck-game-installer install <file_path>
Responsibilities:

Parse command line arguments using Click
Validate file exists and is ISO or EXE
Create GameInstaller instance and call install()
Handle top-level exceptions and display errors

2. KDialog Wrapper (kdialog.py)
Purpose: Abstraction layer for kdialog GUI interactions
Methods:
pythonKDialog.error(title: str, message: str)
# Show error dialog with OK button

KDialog.info(title: str, message: str)  
# Show info dialog with OK button

KDialog.question(title: str, message: str) -> bool
# Show yes/no dialog, return True if Yes

KDialog.select_file(title: str, start_dir: Path, filter: str) -> Optional[Path]
# Show file picker, return selected file or None

KDialog.radio_list(title: str, options: List[Tuple[str, str]], default: str) -> Optional[str]
# Show radio button list
# options: [(tag, display_text), ...]
# Returns selected tag or None

KDialog.combo_box(title: str, message: str, options: List[str], default: str) -> Optional[str]
# Show dropdown selection
# Returns selected option or None
Implementation Details:

Use subprocess.run() to call kdialog
Parse stdout for return values
Check returncode for cancel detection (non-zero = cancelled)

3. ISO Manager (iso.py)
Purpose: Mount/unmount ISOs and find executables
Methods:
pythonISOManager.mount(iso_path: Path) -> Path
# Mount ISO using: udisksctl loop-setup -f <iso_path>
# Then: udisksctl mount -b <loop_device>
# Parse output to extract mount point
# Return mount point Path

ISOManager.unmount()
# Unmount using: udisksctl unmount -p <mount_point>

ISOManager.find_installer(search_path: Path) -> List[Path]
# Search for *.exe recursively
# Prioritize in order: setup.exe, install.exe, installer.exe, autorun.exe
# Then include executables with "setup", "install", "autorun" in name
# Then include all other executables
# Return sorted list with likely installers first
Context Manager:

Implement enter and exit to ensure unmount on cleanup

4. Steam Shortcuts Manager (steam.py)
Purpose: Manage Steam's shortcuts.vdf file
Key Paths:

Shortcuts file: ~/.local/share/Steam/userdata/<USERID>/config/shortcuts.vdf
Find first numeric directory in userdata/

Methods:
pythonSteamShortcuts.add_shortcut(
    app_name: str,
    exe_path: str, 
    start_dir: str = "",
    launch_options: str = ""
) -> int
# Create new shortcut entry
# Generate app_id using Steam's algorithm: hash(exe_path + app_name)
# Return shortcut index

SteamShortcuts.find_shortcut_by_exe(exe_path: str) -> Optional[int]
# Locate the newest shortcut whose Exe matches exe_path
# Used after steamos-add-to-steam creates the entry

SteamShortcuts.update_shortcut(index: int, **kwargs)
# Update existing shortcut
# Accepts: app_name, exe_path, start_dir, launch_options
# Regenerate app_id if exe_path or app_name changed

SteamShortcuts.get_app_id(index: int) -> Optional[int]
# Return app_id for shortcut index
VDF Format (binary):
python{
    'shortcuts': {
        '0': {
            'appid': <generated_int>,
            'AppName': 'Game Name',
            'Exe': '"C:\\path\\to\\game.exe"',
            'StartDir': '"C:\\path\\to"',
            'icon': '',
            'ShortcutPath': '',
            'LaunchOptions': '',
            'IsHidden': 0,
            'AllowDesktopConfig': 1,
            'AllowOverlay': 1,
            'OpenVR': 0,
            'Devkit': 0,
            'DevkitGameID': '',
            'DevkitOverrideAppID': 0,
            'LastPlayTime': 0,
            'FlatpakAppID': '',
            'tags': {}
        }
    }
}
App ID Generation:
pythondef _generate_app_id(exe_path: str, app_name: str) -> int:
    key = f"{exe_path}{app_name}"
    top = struct.unpack('<I', struct.pack('<i', hash(key) & 0xFFFFFFFF))[0] | 0x80000000
    return (top << 32) | 0x02000000
5. Proton Manager (proton.py)
Purpose: Detect Proton versions and scan game prefixes
Key Paths:

Proton: ~/.local/share/Steam/steamapps/common/Proton*
GE-Proton: ~/.local/share/Steam/compatibilitytools.d/*
Prefix: ~/.local/share/Steam/steamapps/compatdata/<APPID>/pfx/drive_c/

Methods:
pythonProtonManager.get_available_proton_versions() -> List[str]
# Scan both common/ and compatibilitytools.d/
# Return sorted list with "Proton Experimental" first
# Then descending version numbers (9.0 before 8.0)

ProtonManager.scan_prefix_for_executables(app_id: int) -> List[Tuple[str, Path]]
# Scan prefix drive_c for executables
# Search in: Program Files, Program Files (x86), Games
# Filter out:
#   - unins*.exe, uninst*.exe
#   - vcredist*.exe, directx*.exe, dxsetup.exe
#   - setup.exe, install.exe, installer.exe
#   - *redist*.exe, *crash*reporter*.exe
# Exclude directories: Windows, Common Files, Internet Explorer, etc.
# Return [(display_name, windows_path), ...]
# display_name format: "game.exe (Program Files/GameName)"
# windows_path format: "C:\\Program Files\\GameName\\game.exe"
# Sort by modification time (newest first)
6. Game Installer Orchestrator (installer.py)
Purpose: Main workflow coordination
Method:
pythonGameInstaller.install(file_path: Path)
# Route to _install_from_iso() or _install_from_exe() based on extension
# Handle exceptions and show error dialogs
# Ensure ISO cleanup in finally block

_install_from_iso(iso_path: Path)
# 1. Mount ISO with ISOManager
# 2. Call _select_installer_from_iso()
# 3. Call _run_installation_workflow()
# 4. Unmount ISO

_install_from_exe(exe_path: Path)
# Call _run_installation_workflow() directly

_select_installer_from_iso(mount_point: Path) -> Optional[Path]
# Find installers with ISOManager.find_installer()
# Build options list: [(str(path), display_name), ...]
# Add browse option: ("__browse__", "Browse for another executable...")
# Show radio_list with first option as default
# If "__browse__" selected, show file picker
# Return selected Path or None

_run_installation_workflow(installer_path: Path, game_name: str)
# 1. Get Proton versions, default to Experimental
# 2. Add installer to Steam using steamos-add-to-steam (steam://addnonsteamgame/<path>)
# 3. Read shortcuts.vdf and locate the new shortcut by exe_path
# 4. Get app_id from shortcut
# 5. Set Proton version via _set_proton_for_app()
# 6. Show info dialog: "Installer will launch, complete installation then click OK"
# 7. Launch via _launch_steam_app(app_id)
# 8. Show question dialog: "Installation complete?"
# 9. Scan prefix with ProtonManager.scan_prefix_for_executables()
# 10. Call _select_game_exe_and_proton()
# 11. Update shortcut with game exe and working directory
# 12. Update Proton version if changed
# 13. Show success dialog

_select_game_exe_and_proton(executables, proton_versions, default_proton) -> Tuple[Path, str]
# Show radio_list for executables
# Show combo_box for Proton versions
# Return (selected_exe_path, selected_proton) or (None, None)

_set_proton_for_app(app_id: int, proton_version: str)
# Modify Steam's config.vdf
# Path: userdata/<USERID>/config/config.vdf
# Navigate to: InstallConfigStore/Software/Valve/Steam/CompatToolMapping
# Set: {str(app_id): {'name': proton_version, 'config': '', 'priority': '250'}}
# Save with vdf.dump()

_launch_steam_app(app_id: int)
# Execute: steam steam://rungameid/<app_id>
# Use subprocess.Popen with DEVNULL
# Sleep 2 seconds for Steam to initialize
7. Dolphin Service Menu (install-to-steam.desktop)
Location: ~/.local/share/kio/servicemenus/install-to-steam.desktop
Content:
ini[Desktop Entry]
Type=Service
ServiceTypes=KonqPopupMenu/Plugin
MimeType=application/x-cd-image;application/x-iso9660-image;application/x-executable;application/x-ms-dos-executable;
Actions=InstallToSteam

[Desktop Action InstallToSteam]
Name=Install into Steam
Icon=steam
Exec=deck-game-installer install %f
```

## Error Handling

**Required error messages**:
- "Failed to mount ISO: <error>"
- "No Proton versions found. Please install Proton from Steam."
- "Could not add game to Steam shortcuts."
- "No game executables found in prefix. Game may not have installed correctly."
- "Unsupported file type: <ext>. Supported: .iso, .img, .exe"

**Cleanup requirements**:
- Always unmount ISO in finally block
- Show error dialog before raising exceptions
- Handle user cancellation (None returns) gracefully

## File Formats

### Steam shortcuts.vdf (binary VDF)
Use `vdf.binary_load()` and `vdf.binary_dump()`

### Steam config.vdf (text VDF)
Use `vdf.load()` and `vdf.dump()`

## Installation

**Package structure**:
```
pyproject.toml - setuptools configuration
deck_game_installer/ - Python package
mise.toml - mise tasks and tooling configuration
install-to-steam.desktop - Service menu file
```

**Install commands**:
- Use mise tasks defined in mise.toml
- `mise run install` should run two subtasks:
    - `mise run app` (updates/installs the Python package in editable mode)
    - `mise run servicemenu` (installs the Dolphin service menu)
Testing Considerations

Mock kdialog calls for automated testing
Test with various ISO structures
Test exe name extraction from paths
Verify VDF file format preservation
Test Proton version sorting algorithm
Verify prefix scanning filters work correctly

Non-Goals (Future Features)

Artwork download (SteamGridDB integration)
SMB share mounting (users handle this themselves)
TUI/terminal interface (GUI-only for now)
Configuration file
Multiple Steam user support
Progress bars during operations
Logging/debug output
