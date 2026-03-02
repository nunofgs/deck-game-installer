package installer

import (
	"context"
	"fmt"
	"path/filepath"

	"deck-game-installer/iso"
)

// MountSMB mounts an SMB share for local access.
type MountSMB struct {
	BaseStep
	smbInfo *iso.SMBInfo
}

// NewMountSMB creates a new SMB mount step.
func NewMountSMB(smbInfo *iso.SMBInfo) *MountSMB {
	return &MountSMB{smbInfo: smbInfo}
}

func (s *MountSMB) Name() string {
	return "Mount Network Share"
}

func (s *MountSMB) Execute(ctx context.Context, state *State) error {
	state.UI.Log(fmt.Sprintf("Mounting SMB share //%s/%s...", s.smbInfo.Server, s.smbInfo.Share))

	mount, err := iso.RemountSMB(ctx, s.smbInfo)
	if err != nil {
		return fmt.Errorf("failed to mount SMB share: %w", err)
	}

	state.SMBMount = mount
	state.InputPath = filepath.Join(mount.MountPoint, s.smbInfo.RelPath)

	if mount.WasExisting() {
		state.UI.Log("Using existing mount at: " + mount.MountPoint)
	} else {
		state.UI.Log("Mounted SMB share at: " + mount.MountPoint)
	}

	state.UI.Log("ISO path updated to: " + state.InputPath)
	return nil
}


