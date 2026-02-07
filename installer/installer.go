package installer

import (
	"errors"
	"os/exec"
	"path/filepath"
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
	isoMgr := iso.NewManager()
	isoMgr.SetLogger(func(msg string) {
		logWin.Log(msg)
	})
	
	return &Installer{
		logWin: logWin,
		isoMgr: isoMgr,
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

	// Set game name early
	gameName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	i.logWin.SetGameName(gameName)

	if smb := iso.ParseKioPath(path); smb != nil {
		i.logWin.Log("\n--- DETECTED NETWORK SHARE ---")
		i.logWin.Log("Server: " + smb.Server + ", Share: " + smb.Share)
		i.logWin.Log("Looking for share: //" + smb.Server + "/" + smb.Share)
		i.logWin.Log("Checking for existing SMB mount...")

		m, err := iso.RemountSMB(smb)
		if err != nil {
			i.logWin.Log("Failed to remount SMB share: " + err.Error())
		} else {
			smbMount = m
			path = filepath.Join(m.MountPoint, smb.RelPath)
			i.logWin.Log("Using SMB mount at: " + m.MountPoint)
			i.logWin.Log("ISO will be accessed at: " + path)
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
	i.logWin.Log("Attempting to mount ISO file...")
	i.logWin.Log("ISO Path: " + path)
	mountPoint, err := i.isoMgr.Mount(path)
	if err != nil {
		i.logWin.Log("Standard mount failed: " + err.Error())
		i.logWin.Log("Attempting root mount (may require password)...")
		mountPoint, err = i.isoMgr.MountRoot(path)
		if err != nil {
			i.logWin.Log("Root mount also failed: " + err.Error())
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
	gameName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	i.logWin.SetGameName(gameName)
	i.logWin.Log("Selected installer: " + path)
	return i.runInstallationWorkflow(path, gameName)
}

func (i *Installer) runInstallationWorkflow(installerPath, gameName string) error {
	i.logWin.SetStep("Adding to Steam")
	i.logWin.Log("\n--- STEP 3: ADDING TO STEAM ---")

	cleanName := cleanGameName(gameName)

	appID, err := i.steam.FindAppIDByPath(installerPath)
	if err == nil {
		i.logWin.Log("Shortcut for installer already exists.")
	} else {
		i.logWin.Log("Adding installer to Steam library...")
		appID, err = i.steam.AddShortcut(cleanName, installerPath, "", "")
		if err != nil {
			i.logWin.Log("Failed to add shortcut: " + err.Error())
			appID = steam.GenerateAppID(installerPath, cleanName)
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
			i.logWin.Log("Shutting down Steam...")
			i.steam.RestartSteam()
			i.logWin.Log("Steam restarted. Waiting for it to fully initialize...")
			time.Sleep(10 * time.Second)
			i.logWin.Log("Steam should now be ready.")
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
	i.logWin.Log("1. Complete the installation in the window that opened.")
	i.logWin.Log("2. Once the installation is finished, CLICK 'OK' BELOW to continue.")
	if !i.logWin.Wait() {
		i.logWin.Log("User cancelled.")
		return nil
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

	i.logWin.Log("\n--- STEP 6: FINALIZING ---")
	i.logWin.SetStep("Finalizing")
	i.logWin.Log("Updating Steam shortcut to point to game executable...")
	i.logWin.Log("Game exe: " + selectedExe)
	
	// Update the existing installer shortcut to point to the game exe
	if err := i.steam.UpdateShortcut(appID, selectedExe, ""); err != nil {
		i.logWin.Error("Error", "Failed to update shortcut: "+err.Error())
		return err
	}

	i.logWin.Log("\nSuccessfully completed installation!")
	i.logWin.Log("The game will keep using Proton Experimental.")
	
	// Ask about restarting Steam
	if i.logWin.Confirm("Restart Steam?", "A Steam restart is recommended to refresh the library. Restart now?") {
		i.logWin.Log("Shutting down Steam...")
		i.steam.RestartSteam()
		i.logWin.Log("Steam has been restarted.")
	}
	
	// Show completion screen with quit button
	i.logWin.ShowComplete()
	i.logWin.Wait()

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
