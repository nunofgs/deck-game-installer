package installer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"deck-game-installer/gui"
	"deck-game-installer/iso"
	"deck-game-installer/proton"
	"deck-game-installer/steam"
)

type Installer struct {
	logWin *gui.LogWindow
	isoMgr *iso.Manager
	steam  *steam.Manager
	proton *proton.Manager
}

func NewInstaller(logWin *gui.LogWindow) *Installer {
	return &Installer{
		logWin: logWin,
		isoMgr: iso.NewManager(),
		steam:  steam.NewManager(),
		proton: proton.NewManager(),
	}
}

func (i *Installer) Install(path string) error {
	i.logWin.SetStep("Initializing")
	i.logWin.Log("--- STEP 1: INITIALIZING ---")
	i.logWin.Log("Starting installation for: " + path)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".iso":
		return i.installFromISO(path)
	case ".exe":
		return i.installFromExe(path)
	default:
		return errors.New("unsupported file type: " + ext)
	}
}

func (i *Installer) installFromISO(path string) error {
	var smbMount *iso.SMBMount

	if smb := iso.ParseKioPath(path); smb != nil {
		i.logWin.Log("\n--- DETECTED NETWORK SHARE ---")
		i.logWin.Log("Server: " + smb.Server + ", Share: " + smb.Share)
		i.logWin.Log("Automatically remounting SMB share as system drive (requires admin password)...")

		m, err := iso.RemountSMB(smb)
		if err != nil {
			i.logWin.Log("Failed to remount SMB share: " + err.Error())
		} else {
			smbMount = m
			path = filepath.Join(m.MountPoint, smb.RelPath)
			i.logWin.Log("SMB Share remounted at: " + m.MountPoint)
			i.logWin.Log("New ISO Path: " + path)
		}
	}

	defer func() {
		i.isoMgr.Unmount()
		if smbMount != nil {
			i.logWin.Log("Unmounting temporary SMB share...")
			_ = smbMount.Unmount()
		}
		i.logWin.Log("\n--- INSTALLATION FINISHED ---")
		i.logWin.Log("You can close this window now.")
		i.logWin.Wait()
	}()

	i.logWin.Log("\n--- STEP 2: MOUNTING ISO ---")
	i.logWin.SetStep("Mounting ISO")
	mountPoint, err := i.isoMgr.Mount(path)
	if err != nil {
		i.logWin.Log("Standard mount failed: " + err.Error())
		i.logWin.Log("Attempting root mount...")
		mountPoint, err = i.isoMgr.MountRoot(path)
		if err != nil {
			return err
		}
	}

	i.logWin.Log("Mounted at: " + mountPoint)
	i.logWin.Log("Scanning for installer executables...")
	installers, err := iso.FindInstallers(mountPoint)
	if err != nil {
		return err
	}
	if len(installers) == 0 {
		return errors.New("no executables found in mounted ISO")
	}

	selected := installers[0]
	if len(installers) > 1 {
		choice, ok := i.logWin.Select("Select Installer", "Select the installer executable:", installers)
		if !ok {
			return errors.New("installer selection cancelled")
		}
		selected = choice
	}

	i.logWin.Log("Selected installer: " + selected)
	return i.runInstallationWorkflow(selected, gameNameFromPath(path, mountPoint))
}

func (i *Installer) installFromExe(path string) error {
	i.logWin.Log("Selected installer: " + path)
	return i.runInstallationWorkflow(path, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func (i *Installer) runInstallationWorkflow(installerPath, gameName string) error {
	i.logWin.SetStep("Adding to Steam")
	i.logWin.Log("\n--- STEP 3: ADDING TO STEAM ---")

	cleanName := cleanGameName(gameName)
	installerName := cleanName + " (Installer)"

	appID, err := i.steam.FindAppIDByPath(installerPath)
	if err == nil {
		i.logWin.Log("Shortcut for installer already exists.")
	} else {
		i.logWin.Log("Adding installer to Steam library...")
		appID, err = i.steam.AddShortcut(installerName, installerPath, "", "")
		if err != nil {
			i.logWin.Log("Failed to add shortcut: " + err.Error())
			appID = steam.GenerateAppID(installerPath, installerName)
		}
	}

	versions := i.proton.GetAvailableProtonVersions()
	defaultProton := pickDefaultProton(versions)

	needsRestart := false
	if defaultProton != "" {
		if existing, err := i.steam.GetProtonVersion(appID); err == nil && existing != "" {
			i.logWin.Log("Proton already configured: " + existing)
		} else {
			i.logWin.Log("Assigning " + defaultProton + " to installer...")
			if err := i.steam.SetProtonVersion(appID, defaultProton); err == nil {
				needsRestart = true
			}
		}
	}

	if needsRestart {
		i.logWin.SetButtons("Restart Now", "Later")
		i.logWin.Log("\n>>> STEAM RESTART REQUIRED <<<")
		i.logWin.Log("Steam must be restarted to recognize the new shortcut and Proton settings.")
		if i.logWin.Wait() {
			i.logWin.Log("Restarting Steam...")
			i.steam.RestartSteam()
			i.logWin.Log("Waiting for Steam to restart...")
			time.Sleep(10 * time.Second)
		} else {
			i.logWin.Log("User chose to restart later.")
		}
		i.logWin.SetButtons("OK", "Cancel")
	}

	urlID := steam.GetURLAppIDFromU32(appID)
	i.logWin.Log("\n--- STEP 4: RUNNING INSTALLER ---")
	i.logWin.SetStep("Running Installer")
	i.logWin.Log("Launching installer...")
	_ = runCommand("steam", "steam://rungameid/"+urlID)

	i.logWin.Log("\n>>> ACTION REQUIRED <<<")
	i.logWin.Log("Complete the installation in the window that opened.")
	i.logWin.Log("Monitoring Steam logs for installer process...")
	
	// Monitor Steam's logs to detect when installer processes exit
	if err := waitForSteamGameToExit(urlID, func(status string) {
		i.logWin.Log(status)
	}); err != nil {
		i.logWin.Log("Could not monitor installer automatically: " + err.Error())
		i.logWin.Log("Please click 'OK' when installation is complete.")
		if !i.logWin.Wait() {
			i.logWin.Log("User cancelled.")
			return nil
		}
	} else {
		i.logWin.Log("All installer processes have exited.")
		time.Sleep(2 * time.Second) // Give filesystem time to settle
	}

	i.logWin.Log("\n--- STEP 5: FINDING GAME ---")
	i.logWin.SetStep("Finding Game")
	i.logWin.Log("Scanning Proton prefix for game executables...")
	executables := i.proton.ScanPrefixForExecutables(appID)
	if len(executables) == 0 {
		i.logWin.Error("Error", "No game executables found. Installation may have failed.")
		return errors.New("no executables found")
	}

	selectedExe := executables[0]
	if len(executables) > 1 {
		choice, ok := i.logWin.Select("Select Game", "Select the game executable:", executables)
		if !ok {
			return errors.New("game selection cancelled")
		}
		selectedExe = choice
	}

	selectedProton := defaultProton
	if len(versions) > 0 {
		choice, ok := i.logWin.Select("Select Proton", "Select Proton version for the game:", versions)
		if ok {
			selectedProton = choice
		}
	}

	i.logWin.Log("\n--- STEP 6: FINALIZING ---")
	i.logWin.SetStep("Finalizing")
	finalAppID, err := i.steam.FindAppIDByPath(selectedExe)
	if err != nil {
		i.logWin.Log("Adding game to Steam library...")
		finalAppID, err = i.steam.AddShortcut(cleanName, selectedExe, "", "")
		if err != nil {
			i.logWin.Error("Error", "Failed to finalize: "+err.Error())
			return err
		}
	}

	if selectedProton != "" {
		_ = i.steam.SetProtonVersion(finalAppID, selectedProton)
	}

	i.logWin.Log("\nSuccessfully completed installation!")
	if i.logWin.Confirm("Restart Steam?", "Proton settings require a Steam restart to take effect. Restart now?") {
		i.steam.RestartSteam()
	} else {
		i.logWin.Log("Please restart Steam manually to apply Proton settings.")
	}

	return nil
}

func pickDefaultProton(versions []string) string {
	for _, v := range versions {
		if strings.Contains(strings.ToLower(v), "experimental") {
			return v
		}
	}
	if len(versions) > 0 {
		return versions[0]
	}
	return ""
}

func gameNameFromPath(isoPath, mountPoint string) string {
	base := filepath.Base(mountPoint)
	if strings.HasPrefix(base, "steamer_mnt_") || len(base) <= 3 {
		return strings.TrimSuffix(filepath.Base(isoPath), filepath.Ext(isoPath))
	}
	return base
}

func cleanGameName(name string) string {
	replacer := strings.NewReplacer("_", " ", ".", " ")
	clean := replacer.Replace(name)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return name
	}
	return clean
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func waitForSteamGameToExit(gameID string, logger func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	
	// Path to Steam's console log
	logPath := filepath.Join(home, ".local", "share", "Steam", "logs", "console-linux.txt")
	
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return errors.New("Steam log file not found")
	}
	
	// Get initial file size to start reading from the end
	fileInfo, err := os.Stat(logPath)
	if err != nil {
		return err
	}
	offset := fileInfo.Size()
	
	logger("Waiting for installer to start...")
	
	// Track PIDs for this game
	activePIDs := make(map[int]bool)
	hasStarted := false
	
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	timeout := time.After(5 * time.Minute) // 5 minute timeout
	
	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for installer")
			
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
										logger("Installer started (tracking " + strconv.Itoa(len(activePIDs)) + " process(es))")
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
			
			// Update status periodically (every 4 seconds = 8 ticks)
			if hasStarted && len(activePIDs) > 0 {
				// Only log occasionally to avoid spam
				static := struct {
					counter int
				}{}
				static.counter++
				if static.counter%8 == 0 {
					logger("Installer running (" + strconv.Itoa(len(activePIDs)) + " process(es) active)...")
				}
			}
		}
	}
}
