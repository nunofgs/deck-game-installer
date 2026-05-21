// Package installer provides the game installation workflow.
package installer

import (
	"deck-game-installer/iso"
	"deck-game-installer/proton"
	"deck-game-installer/smb"
	"deck-game-installer/steam"
	"deck-game-installer/ui"
)

// ResumeMode describes what to do when the user re-points at an installer
// that already has an existing Steam shortcut.
type ResumeMode int

const (
	ResumeModeNone     ResumeMode = iota // Fresh installation
	ResumeModeFinished                   // Install done — find game and update shortcut
	ResumeModePending                    // Still installing — re-launch installer
)

// State holds all shared data passed between workflow steps.
type State struct {
	// Input
	InputPath  string // Original path (ISO, SMB, or EXE)
	InputMode  InputMode
	GameName   string     // Derived or user-provided game name
	ResumeMode ResumeMode // Set when resuming an existing shortcut

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
	GameExePath string // Path to the installed game executable

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
