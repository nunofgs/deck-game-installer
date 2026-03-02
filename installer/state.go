// Package installer provides the game installation workflow.
package installer

import (
	"deck-game-installer/iso"
	"deck-game-installer/proton"
	"deck-game-installer/smb"
	"deck-game-installer/steam"
	"deck-game-installer/ui"
)

// State holds all shared data passed between workflow steps.
type State struct {
	// Input
	InputPath string // Original path (ISO, SMB, or EXE)
	GameName  string // Derived or user-provided game name

	// Mount state
	ISOManager *iso.Manager  // Non-nil if we mounted an ISO
	SMBMount   *smb.SMBMount // Non-nil if we mounted an SMB share
	MountPoint string        // Where the ISO is mounted

	// Installer
	InstallerPath string // Path to the installer executable

	// Steam
	AppID         int32  // Steam shortcut app ID
	ProtonVersion string // Proton version selected for this shortcut

	// Installed game
	GameExePath  string // Path to the installed game executable
	GameStartDir string // Start directory for the game

	// Dependencies
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
