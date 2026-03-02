package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"deck-game-installer/iso"
)

// MountISO mounts an ISO file for access.
type MountISO struct {
}

// NewMountISO creates a new ISO mount step.
func NewMountISO() *MountISO {
	return &MountISO{}
}

func (s *MountISO) Name() string {
	return "Mount ISO"
}


func (s *MountISO) Execute(ctx context.Context, state *State) error {
	state.UI.Log("Mounting ISO: " + state.InputPath)

	// Create ISO manager
	isoMgr := iso.NewManager()
	isoMgr.SetLogger(state.UI.Log)
	state.ISOManager = isoMgr

	// Try regular mount first
	mountPoint, err := isoMgr.Mount(ctx, state.InputPath)
	if err != nil {
		state.UI.Log("Regular mount failed, trying with elevated permissions...")

		// Try root mount as fallback
		mountPoint, err = isoMgr.MountRoot(ctx, state.InputPath)
		if err != nil {
			return fmt.Errorf("failed to mount ISO: %w", err)
		}
	}

	state.MountPoint = mountPoint

	// Derive game name from ISO path and mount point (same logic as v1)
	state.GameName = gameNameFromPath(state.InputPath, mountPoint)
	state.UI.Log("Game name: " + state.GameName)

	if isoMgr.WasExisting() {
		state.UI.Log("Using existing mount at: " + mountPoint)
	} else {
		state.UI.Log("ISO mounted at: " + mountPoint)
	}

	return nil
}

// gameNameFromPath derives the game name from the ISO path and mount point.
// If the mount point is a temp directory, uses the ISO filename instead.
func gameNameFromPath(isoPath, mountPoint string) string {
	base := filepath.Base(mountPoint)
	// If it's our temp mount directory, use the ISO filename
	if strings.HasPrefix(base, "deck-game-installer_mnt_") || len(base) <= 3 {
		name := strings.TrimSuffix(filepath.Base(isoPath), filepath.Ext(isoPath))
		return cleanGameName(name)
	}
	return cleanGameName(base)
}

// cleanGameName cleans up a game name by replacing underscores/dots with spaces.
func cleanGameName(name string) string {
	replacer := strings.NewReplacer("_", " ", ".", " ")
	clean := replacer.Replace(name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return name
	}
	return clean
}


