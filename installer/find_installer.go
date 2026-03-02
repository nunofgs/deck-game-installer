package installer

import (
	"context"
	"fmt"
	"path/filepath"

	"deck-game-installer/iso"
)

// FindInstaller scans the mounted ISO for installer executables.
type FindInstaller struct {
	BaseStep
}

// NewFindInstaller creates a new find installer step.
func NewFindInstaller() *FindInstaller {
	return &FindInstaller{}
}

func (s *FindInstaller) Name() string {
	return "Find Installer"
}


func (s *FindInstaller) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Scanning for installers in: " + state.MountPoint)

	installers, err := iso.FindInstallers(state.MountPoint)
	if err != nil {
		return fmt.Errorf("failed to scan for installers: %w", err)
	}

	if len(installers) == 0 {
		return fmt.Errorf("no installer executables found in ISO")
	}

	state.UI.Log(fmt.Sprintf("Found %d executable(s)", len(installers)))

	// If only one installer found, use it directly
	if len(installers) == 1 {
		state.InstallerPath = installers[0]
		state.UI.Log("Selected: " + filepath.Base(state.InstallerPath))
		return nil
	}

	// Show selection dialog for multiple options
	options := make([]string, len(installers))
	for i, path := range installers {
		// Show relative path from mount point for readability
		rel, _ := filepath.Rel(state.MountPoint, path)
		if rel == "" {
			rel = filepath.Base(path)
		}
		options[i] = rel
	}

	selected, ok := state.UI.Select(
		"Select Installer",
		"Multiple executables found. Please select the installer:",
		options,
	)

	if !ok {
		return fmt.Errorf("installation cancelled by user")
	}

	// Find the selected path
	for i, opt := range options {
		if opt == selected {
			state.InstallerPath = installers[i]
			break
		}
	}

	state.UI.Log("Selected: " + filepath.Base(state.InstallerPath))
	return nil
}


