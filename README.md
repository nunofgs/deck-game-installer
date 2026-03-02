# deck-game-installer

Installs Windows games on Steam Deck via Proton. Point it at an ISO or installer EXE and it handles the rest — mounting, Steam shortcut creation, Proton setup, and finding the game executable when the installer finishes.

## Installation

```bash
curl -fsSL https://github.com/nunofgs/deck-game-installer/releases/latest/download/install.sh | sh
```

## Usage

### In KDE (recommended)

Right-click any ISO or EXE in Dolphin and choose **Install with Steam**.

<!-- screenshot -->

### From the terminal

```bash
deck-game-installer /path/to/game.iso
deck-game-installer /path/to/setup.exe
deck-game-installer smb://mynas/games/game.iso
```

SMB paths are useful for installing directly from a NAS without copying files to the Deck first.

## How it works

1. Mounts the ISO (or SMB share)
2. Finds and launches the installer through Steam/Proton
3. Watches Steam's logs to detect when the installer exits
4. Scans the Proton prefix for the installed game executable
5. Updates the Steam shortcut to point at the game

## License

MIT
