package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"deck-game-installer/steam"
)

// WaitForExit monitors Steam's console log to detect when the installer exits.
type WaitForExit struct {
	BaseStep
}

// NewWaitForExit creates a new wait for exit step.
func NewWaitForExit() *WaitForExit {
	return &WaitForExit{}
}

func (s *WaitForExit) Name() string {
	return "Run Installer"
}

func (s *WaitForExit) Description(state *State) string {
	return "Monitored installer process"
}

func (s *WaitForExit) Execute(ctx context.Context, state *State) error {
	state.UI.Log(">>> ACTION REQUIRED <<<")
	state.UI.Log("Complete the installation in the game window and quit the installer.")
	state.UI.Log("We'll continue automatically once all installer processes exit.")
	state.UI.Log("If the installer doesn't appear, click the button below to continue manually.")

	// Get the URL app ID format used in Steam logs
	urlAppID := steam.GetURLAppID(state.AppID)

	// Start monitoring in background
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- waitForSteamGameToExit(ctx, urlAppID, state.UI.Log)
	}()

	// Show manual override button (returns a channel)
	manualCh := state.UI.WaitWithManualOverride()

	// Wait for either automatic detection or manual override
	select {
	case err := <-doneCh:
		if err != nil {
			state.UI.Log("Automatic detection issue: " + err.Error())
			state.UI.Log("Proceeding anyway - please ensure the installer completed successfully.")
		} else {
			state.UI.Log("All installer processes have exited.")
		}
	case <-manualCh:
		state.UI.Log("Manual override - continuing...")
	case <-ctx.Done():
		return ctx.Err()
	}

	// Give filesystem time to settle
	time.Sleep(2 * time.Second)

	return nil
}

// CanRollback returns false - nothing to roll back.
func (s *WaitForExit) CanRollback() bool {
	return false
}

// waitForSteamGameToExit monitors Steam's console log for process exit.
// This matches v1's logic exactly: track "Adding process" and "Removing process" entries.
func waitForSteamGameToExit(ctx context.Context, gameID string, logFn func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logPath := filepath.Join(home, ".local", "share", "Steam", "logs", "console-linux.txt")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("Steam log file not found")
	}

	// Get initial file size to start reading from the end
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return err
	}
	offset := fileInfo.Size()

	logFn("Waiting for installer to start...")

	// Track PIDs for this game
	activePIDs := make(map[int]bool)
	hasStarted := false

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute) // 5 minute timeout

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			return fmt.Errorf("timeout waiting for installer")

		case <-ticker.C:
			file, err := os.Open(logPath)
			if err != nil {
				continue
			}

			// Seek to last known position
			file.Seek(offset, 0)

			// Read new content
			buf := make([]byte, 8192)
			n, _ := file.Read(buf)
			if n > 0 {
				offset += int64(n)
				content := string(buf[:n])
				lines := strings.Split(content, "\n")

				for _, line := range lines {
					// Look for "Adding process [PID] for gameID [ID]"
					if strings.Contains(line, "Adding process") && strings.Contains(line, "for gameID "+gameID) {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "process" && i+1 < len(parts) {
								if pid, err := strconv.Atoi(parts[i+1]); err == nil {
									activePIDs[pid] = true
									if !hasStarted {
										logFn("Installer started (tracking " + strconv.Itoa(len(activePIDs)) + " process(es))")
										hasStarted = true
									}
								}
								break
							}
						}
					}

					// Look for "Removing process [PID] for gameID [ID]"
					if strings.Contains(line, "Removing process") && strings.Contains(line, "for gameID "+gameID) {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "process" && i+1 < len(parts) {
								if pid, err := strconv.Atoi(parts[i+1]); err == nil {
									delete(activePIDs, pid)
								}
								break
							}
						}
					}
				}
			}
			file.Close()

			// If we've seen processes start and all have exited, we're done
			if hasStarted && len(activePIDs) == 0 {
				return nil
			}
		}
	}
}
