# deck-game-installer

Installs Windows games on Steam Deck via Proton. Point it at an ISO or installer EXE and it handles the rest — mounting, Steam shortcut creation, Proton setup, and finding the game executable when the installer finishes.

---

😩 You know the drill. Add shortcut to Steam, set compatibility to Proton, boot into desktop mode, run the installer, figure out where it installed to, update the shortcut path, set the launch options... it's a lot for what should be a simple thing.

🎮 This tool does all of that for you. Right-click an ISO or EXE, hit install, and by the end you've got a working game in your Steam library.

---

## Installation

```bash
curl -fsSL https://github.com/nunofgs/deck-game-installer/releases/latest/download/install.sh | sh
```

## Usage

### In Desktop

Right-click any ISO or EXE in Dolphin and choose **Install with Steam**.

![Right-click context menu in Dolphin](screenshots/right_click.png)

The installer runs in a guided window that tracks each step and prompts you when input is needed.

![Installation wizard](screenshots/wizard.png)

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
