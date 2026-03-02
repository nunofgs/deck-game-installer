// Package installer provides the game installation workflow.
package installer

import (
	"deck-game-installer/iso"
	"deck-game-installer/proton"
	"deck-game-installer/steam"
	"deck-game-installer/ui"
)

// State holds all shared data passed between workflow steps.
// Each step can read from and write to this state as needed.
type State struct {
	// Input configuration
	InputPath string // Original path (ISO file, SMB path, or EXE)
	GameName  string // Name of the game (derived from filename or user input)

	// Mount state
	ISOManager *iso.Manager   // ISO mount manager (nil if not mounted)
	SMBMount   *iso.SMBMount  // SMB mount info (nil if not an SMB path)
	MountPoint string         // Where the ISO is mounted

	// Installer state
	InstallerPath string // Path to the installer executable

	// Steam state
	AppID                int32  // Steam shortcut app ID
	OriginalShortcutExe  string // Original exe path (for rollback after update)
	OriginalShortcutDir  string // Original start dir (for rollback after update)
	ProtonVersion        string // Selected Proton version
	ProtonConfigChanged  bool   // Whether we changed the Proton config
	SteamRestartRequired bool   // Whether Steam needs to be restarted

	// Game state
	GameExePath   string // Path to the installed game executable
	GameStartDir  string // Start directory for the game
	GameCandidates []string // List of candidate game executables found

	// Dependencies (injected)
	UI     ui.Logger
	Steam  *steam.Manager
	Proton *proton.Manager
}

// NewState creates a new installation state with the given input path and dependencies.
func NewState(inputPath string, logger ui.Logger, steamMgr *steam.Manager, protonMgr *proton.Manager) *State {
	return &State{
		InputPath: inputPath,
		UI:        logger,
		Steam:     steamMgr,
		Proton:    protonMgr,
	}
}
