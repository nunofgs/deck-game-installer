package installer

import (
	"context"
)

// Unmount cleans up ISO and SMB mounts.
type Unmount struct{}

// NewUnmount creates a new unmount step.
func NewUnmount() *Unmount {
	return &Unmount{}
}

func (s *Unmount) Name() string {
	return "Cleanup"
}

func (s *Unmount) Execute(ctx context.Context, state *State) error {
	unmountAll(state)
	state.UI.Log("Cleanup complete")
	return nil
}

// unmountAll tears down any ISO and SMB mounts we created.
// Called both from the Unmount step (happy path) and from Runner.cleanup (error path).
func unmountAll(state *State) {
	if state.ISOManager != nil && !state.ISOManager.WasExisting() {
		state.UI.Log("Unmounting ISO...")
		state.ISOManager.Unmount()
		state.ISOManager = nil
	}
	if state.SMBMount != nil && !state.SMBMount.WasExisting() {
		state.UI.Log("Unmounting network share...")
		if err := state.SMBMount.Unmount(); err != nil {
			state.UI.Log("Warning: failed to unmount SMB share: " + err.Error())
		}
		state.SMBMount = nil
	}
}
