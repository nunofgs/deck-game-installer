// Package main provides the entry point for the deck-game-installer v2.
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
	"deck-game-installer/iso"
	"deck-game-installer/proton"
	"deck-game-installer/steam"
	"deck-game-installer/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "install":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		runInstall(os.Args[2])

	case "help", "-h", "--help":
		printUsage()

	default:
		// Treat single argument as install path for convenience
		runInstall(os.Args[1])
	}
}

func printUsage() {
	fmt.Println("Deck Game Installer")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  deck-game-installer install <path-to-iso-or-exe>")
	fmt.Println("  deck-game-installer <path-to-iso-or-exe>")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  deck-game-installer install /path/to/game.iso")
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
	logger := ui.NewGUILogger("Deck Game Installer", filename)

	// Run installer in background goroutine
	go func() {
		steamMgr := steam.NewManager()
		protonMgr := proton.NewManager()

		state := installer.NewState(path, logger, steamMgr, protonMgr)
		runner := installer.NewRunner(state)
		buildPipeline(runner, path)

		if err := runner.Run(ctx); err != nil {
			logger.ShowFailed(err.Error())
		} else {
			logger.ShowComplete()
		}
	}()

	// Run TUI on main thread — blocks until user closes
	logger.Run()
}

// buildPipeline adds the appropriate steps based on the input path type.
func buildPipeline(runner *installer.Runner, path string) {
	// Check if it's an SMB path
	smbInfo := iso.ParseKioPath(path)
	isISO := strings.HasSuffix(strings.ToLower(path), ".iso")
	isEXE := strings.HasSuffix(strings.ToLower(path), ".exe")

	if smbInfo != nil {
		// SMB path - need to mount the share first
		runner.AddStep(installer.NewMountSMB(smbInfo))

		// After SMB mount, the path becomes local
		if strings.HasSuffix(strings.ToLower(smbInfo.RelPath), ".iso") {
			isISO = true
		} else if strings.HasSuffix(strings.ToLower(smbInfo.RelPath), ".exe") {
			isEXE = true
		}
	}

	if isISO {
		// ISO workflow: mount -> find installer -> add to steam -> etc.
		runner.AddSteps(
			installer.NewMountISO(),
			installer.NewFindInstaller(),
			installer.NewAddToSteam(),
			installer.NewConfigureProton(),
			installer.NewRestartSteam(),
			installer.NewRunInstaller(),
			installer.NewWaitForExit(),
			installer.NewFindGame(),
			installer.NewUpdateShortcut(),
			installer.NewUnmount(),
			installer.NewFinalRestart(),
		)
	} else if isEXE {
		// EXE workflow: use the exe directly, skip mount/find steps
		runner.State().InstallerPath = path
		runner.State().GameName = deriveGameNameFromPath(path)

		runner.AddSteps(
			installer.NewAddToSteam(),
			installer.NewConfigureProton(),
			installer.NewRestartSteam(),
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

// deriveGameNameFromPath extracts a game name from the file path.
func deriveGameNameFromPath(path string) string {
	// Try to get meaningful name from directory
	dir := filepath.Dir(path)
	name := filepath.Base(dir)

	// Clean up
	name = strings.TrimSuffix(name, "_files")
	name = strings.TrimSuffix(name, "-files")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// If generic, use filename
	genericNames := map[string]bool{
		"disc1": true, "disc2": true, "disk1": true, "disk2": true,
		"cd1": true, "cd2": true, "dvd1": true, "dvd2": true,
		"setup": true, "install": true, "installer": true,
		".": true, "/": true, "home": true, "downloads": true,
	}

	if genericNames[strings.ToLower(name)] {
		// Use filename without extension
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.ReplaceAll(name, "-", " ")
	}

	// Title case
	return strings.Title(strings.ToLower(name))
}
