package installer

import (
	"context"
	"fmt"

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

	state.GameName = gameNameFromPath(state.InputPath)
	state.UI.Log("Game name: " + state.GameName)

	if isoMgr.WasExisting() {
		state.UI.Log("Using existing mount at: " + mountPoint)
	} else {
		state.UI.Log("ISO mounted at: " + mountPoint)
	}

	return nil
}
