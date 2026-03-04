// Package main provides the entry point for the deck-game-installer.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"deck-game-installer/installer"
	"deck-game-installer/proton"
	"deck-game-installer/smb"
	"deck-game-installer/steam"
	"deck-game-installer/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printUsage()
	default:
		runInstall(os.Args[1])
	}
}

func printUsage() {
	fmt.Println("Deck Game Installer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  deck-game-installer <path-to-iso-or-exe>")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  deck-game-installer /path/to/game.iso")
	fmt.Println("  deck-game-installer /path/to/setup.exe")
	fmt.Println("  deck-game-installer smb://server/share/game.iso")
}

func runInstall(path string) {
	// Set up context with cancellation on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	filename := filepath.Base(path)
	guiLogger := ui.NewGUILogger("Deck Game Installer", filename)
	logger := ui.NewTeeLogger(guiLogger)
	fmt.Printf("Logging to: %s\n", ui.LogPath())

	// Run installer in background goroutine
	go func() {
		steamMgr := steam.NewManager()
		protonMgr := proton.NewManager()

		state := installer.NewState(path, logger, steamMgr, protonMgr)
		runner := installer.NewRunner(state)
		buildPipeline(runner, path)

		if err := runner.Run(ctx); err != nil {
			logger.Log(fmt.Sprintf("[FATAL] %v", err))
			guiLogger.ShowFailed(err.Error())
		} else {
			logger.Log("[DONE] Installation completed successfully")
			guiLogger.ShowComplete()
		}
	}()

	// Run GUI on main thread — blocks until user closes
	guiLogger.Run()
}

// buildPipeline adds the appropriate steps based on the input path type.
func buildPipeline(runner *installer.Runner, path string) {
	// Check if it's an SMB path
	smbInfo := smb.ParseSMBPath(path)
	isISO := strings.HasSuffix(strings.ToLower(path), ".iso")
	isEXE := strings.HasSuffix(strings.ToLower(path), ".exe")

	if smbInfo != nil {
		// SMB path - need to mount the share first
		runner.AddSteps(installer.NewMountSMB(smbInfo))

		// After SMB mount, the path becomes local
		if strings.HasSuffix(strings.ToLower(smbInfo.RelPath), ".iso") {
			isISO = true
		} else if strings.HasSuffix(strings.ToLower(smbInfo.RelPath), ".exe") {
			isEXE = true
		}
	}

	if isISO {
		// ISO workflow: shutdown steam first so it can't overwrite our shortcut on exit,
		// then write the shortcut + proton config, then launch.
		runner.AddSteps(
			installer.NewMountISO(),
			installer.NewFindInstaller(),
			installer.NewShutdownSteam(),
			installer.NewAddToSteam(),
			installer.NewConfigureProton(),
			installer.NewRunInstaller(),
			installer.NewWaitForExit(),
			installer.NewFindGame(),
			installer.NewUpdateShortcut(),
			installer.NewUnmount(),
			installer.NewFinalRestart(),
		)
	} else if isEXE {
		// EXE workflow: shutdown steam first so it can't overwrite our shortcut on exit,
		// then write the shortcut + proton config, then launch.
		runner.State().InstallerPath = path
		runner.State().GameName = installer.DeriveGameName(path)

		runner.AddSteps(
			installer.NewShutdownSteam(),
			installer.NewAddToSteam(),
			installer.NewConfigureProton(),
			installer.NewRunInstaller(),
			installer.NewWaitForExit(),
			installer.NewFindGame(),
			installer.NewUpdateShortcut(),
			installer.NewFinalRestart(),
		)
	} else {
		fmt.Printf("Error: Unsupported file type. Expected .iso or .exe, got: %s\n", path)
		os.Exit(1)
	}
}
