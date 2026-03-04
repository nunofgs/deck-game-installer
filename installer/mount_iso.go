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

	mountPoint, err := isoMgr.Mount(ctx, state.InputPath)
	if err != nil {
		return fmt.Errorf("failed to mount ISO: %w", err)
	}

	state.MountPoint = mountPoint

	state.GameName = gameNameFromPath(state.InputPath, mountPoint)
	state.UI.Log("Game name: " + state.GameName)

	if isoMgr.WasExisting() {
		state.UI.Log("Using existing mount at: " + mountPoint)
	} else {
		state.UI.Log("ISO mounted at: " + mountPoint)
	}

	return nil
}

// gameNameFromPath derives the game name from the ISO filename.
func gameNameFromPath(isoPath, _ string) string {
	name := strings.TrimSuffix(filepath.Base(isoPath), filepath.Ext(isoPath))
	return cleanGameName(name)
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
