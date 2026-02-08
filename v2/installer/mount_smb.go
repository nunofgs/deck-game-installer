package installer

import (
	"context"
	"fmt"
	"path/filepath"

	"deck-game-installer/v2/iso"
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

func (s *MountSMB) Description(state *State) string {
	if state.SMBMount != nil {
		return fmt.Sprintf("SMB share mounted at %s", state.SMBMount.MountPoint)
	}
	return fmt.Sprintf("SMB share //%s/%s", s.smbInfo.Server, s.smbInfo.Share)
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

func (s *MountSMB) Rollback(ctx context.Context, state *State) error {
	if state.SMBMount != nil && !state.SMBMount.WasExisting() {
		state.UI.Log("Unmounting SMB share...")
		return state.SMBMount.Unmount()
	}
	return nil
}

func (s *MountSMB) CanRollback() bool {
	return true
}
