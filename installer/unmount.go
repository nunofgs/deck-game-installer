package installer

import (
	"context"

)

// Unmount cleans up ISO and SMB mounts.
type Unmount struct {
	BaseStep
}

// NewUnmount creates a new unmount step.
func NewUnmount() *Unmount {
	return &Unmount{}
}

func (s *Unmount) Name() string {
	return "Cleanup"
}

func (s *Unmount) Description(state *State) string {
	return "Unmounted ISO and network shares"
}

func (s *Unmount) Execute(ctx context.Context, state *State) error {
	// Unmount ISO if we mounted it
	if state.ISOManager != nil && !state.ISOManager.WasExisting() {
		state.UI.Log("Unmounting ISO...")
		state.ISOManager.Unmount()
		state.ISOManager = nil
	}

	// Unmount SMB if we mounted it
	if state.SMBMount != nil && !state.SMBMount.WasExisting() {
		state.UI.Log("Unmounting network share...")
		if err := state.SMBMount.Unmount(); err != nil {
			state.UI.Log("Warning: Failed to unmount SMB share: " + err.Error())
		}
		state.SMBMount = nil
	}

	state.UI.Log("Cleanup complete")
	return nil
}

// CanRollback returns false - cleanup is a final step.
func (s *Unmount) CanRollback() bool {
	return false
}
