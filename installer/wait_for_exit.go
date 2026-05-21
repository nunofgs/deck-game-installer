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
}

// NewWaitForExit creates a new wait for exit step.
func NewWaitForExit() *WaitForExit {
	return &WaitForExit{}
}

func (s *WaitForExit) Name() string {
	return "Wait for Installer"
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

	// allExitedAt records when activePIDs last dropped to zero. Some installers
	// (e.g. DODI) use a launcher that exits immediately and spawns the real
	// installer as a child process Steam doesn't track. We wait a grace period
	// after the last tracked process exits before declaring done, so any
	// newly-launched child processes have time to appear in the Steam log.
	var allExitedAt time.Time
	const gracePeriod = 5 * time.Second

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			file, err := os.Open(logPath)
			if err != nil {
				continue
			}

			if _, err := file.Seek(offset, 0); err != nil {
				file.Close()
				continue
			}

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
									allExitedAt = time.Time{} // new process appeared, reset grace timer
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

			// If we've seen processes start and all have exited, start the grace
			// period. Only declare done once the grace period elapses without new
			// processes appearing — this handles launchers that exit before
			// spawning the real installer subprocess.
			if hasStarted && len(activePIDs) == 0 {
				if allExitedAt.IsZero() {
					allExitedAt = time.Now()
					logFn("All tracked processes exited, waiting briefly for any child processes...")
				} else if time.Since(allExitedAt) >= gracePeriod {
					return nil
				}
			}
		}
	}
}
