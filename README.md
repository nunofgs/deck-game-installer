# deck-game-installer

A tool for installing Windows games on Steam Deck and Linux systems. Automates the entire workflow of installing games from ISO files or standalone EXE installers using Steam's Proton compatibility layer.

## Installation

Run this in a terminal on your Steam Deck (or any Linux system):

```bash
curl -fsSL https://github.com/nunofgs/deck-game-installer/releases/latest/download/install.sh | sh
```

This installs the binary to `~/.local/bin/` and sets up the Dolphin right-click menu entry.

## Usage

You can install games in two ways:

1. **From an installer executable** - Point directly to a Windows setup file (e.g., `setup.exe`)
   ```bash
   deck-game-installer /path/to/setup.exe
   ```

2. **From an ISO file** - Point to a game disc image, which can be stored:
   - **On your local drive**: `/path/to/game.iso`
   - **On a network share**: `smb://server/share/game.iso` (great for accessing a NAS from Steam Deck)
   
   The tool will automatically mount the ISO, find the installer inside, and guide you through installation.

## What It Does

1. **Mounts ISOs** - Automatically mounts game ISO files (supports local files and SMB/network shares)
2. **Detects Installers** - Scans mounted ISOs for installer executables
3. **Steam Integration** - Adds installers as temporary Steam shortcuts with Proton configured
4. **Automated Installation** - Launches installers through Steam/Proton and monitors completion by tracking Steam's process logs
5. **Game Detection** - Scans the Proton prefix to find installed game executables
6. **Finalizes Setup** - Updates Steam shortcuts to point to the actual game executable

## Key Features

- **Network Share Support**: Can mount and install from SMB shares (useful for Steam Deck accessing games from a NAS)
- **Automatic Process Monitoring**: Watches Steam's console logs to detect when installers finish, eliminating manual confirmation
- **Proton Management**: Automatically selects and configures appropriate Proton versions
- **Interactive GUI**: Provides a log window with progress updates and user prompts when needed

## License

MIT