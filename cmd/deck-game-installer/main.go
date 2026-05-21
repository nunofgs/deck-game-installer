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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printUsage()
	default:
		runInstall(ctx, os.Args[1])
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
	fmt.Println("  deck-game-installer /path/to/portable-game/Game.exe")
	fmt.Println("  deck-game-installer smb://server/share/game.iso")
}

func runInstall(ctx context.Context, path string) {
	filename := filepath.Base(path)
	guiLogger := ui.NewGUILogger("Deck Game Installer", filename)
	logger := ui.NewTeeLogger(guiLogger)
	fmt.Printf("Logging to: %s\n", ui.LogPath())

	// Run installer in background goroutine
	go func() {
		steamMgr := steam.NewManager()
		protonMgr := proton.NewManager()

		state := installer.NewState(path, logger, steamMgr, protonMgr)

		if err := runWorkflow(ctx, state, path); err != nil {
			logger.Log(fmt.Sprintf("[FATAL] %v", err))
			guiLogger.ShowFailed(err.Error())
		} else {
			logger.Log("[DONE] Installation completed successfully")
			guiLogger.SetAppID(state.AppID)
			guiLogger.ShowComplete()
		}
	}()

	// Run GUI on main thread — blocks until user closes
	guiLogger.Run()
}

func runWorkflow(ctx context.Context, state *installer.State, path string) error {
	if err := runPreparation(ctx, state, path); err != nil {
		return err
	}
	return runSelectedPipeline(ctx, state)
}

func runPreparation(ctx context.Context, state *installer.State, path string) error {
	runner := installer.NewRunner(state)
	smbInfo := smb.ParseSMBPath(path)
	if smbInfo != nil {
		runner.AddSteps(installer.NewMountSMB(smbInfo))
	}
	runner.AddSteps(installer.NewResolveInputMode())
	return runner.Run(ctx)
}

func runSelectedPipeline(ctx context.Context, state *installer.State) error {
	runner := installer.NewRunner(state)
	isISO := strings.HasSuffix(strings.ToLower(state.InputPath), ".iso")

	switch state.InputMode {
	case installer.InputModeInstaller:
		if isISO {
			runner.AddSteps(
				installer.NewMountISO(),
				installer.NewFindInstaller(),
				installer.NewShutdownSteam(),
				installer.NewAddToSteam(),
				installer.NewConfigureProton(),
				installer.NewStartSteamForRedists(),
				installer.NewInstallSteamRedists(),
				installer.NewShutdownSteam(),
				installer.NewRunInstaller(),
				installer.NewWaitForExit(),
				installer.NewFindGame(),
				installer.NewUpdateShortcut(),
				installer.NewUnmount(),
				installer.NewFinalRestart(),
			)
			return runner.Run(ctx)
		}

		state.InstallerPath = state.InputPath
		state.GameName = installer.DeriveGameName(state.InputPath)
		runner.AddSteps(
			installer.NewConfirmGameName(),
			installer.NewShutdownSteam(),
			installer.NewAddToSteam(),
			installer.NewConfigureProton(),
			installer.NewStartSteamForRedists(),
			installer.NewInstallSteamRedists(),
			installer.NewShutdownSteam(),
			installer.NewRunInstaller(),
			installer.NewWaitForExit(),
			installer.NewFindGame(),
			installer.NewUpdateShortcut(),
			installer.NewFinalRestart(),
		)
		return runner.Run(ctx)

	case installer.InputModePortable:
		runner.AddSteps(
			installer.NewFindPortableGame(),
			installer.NewConfirmGameName(),
			installer.NewShutdownSteam(),
			installer.NewAddGameToSteam(),
			installer.NewConfigureProton(),
			installer.NewStartSteamForRedists(),
			installer.NewInstallSteamRedists(),
			installer.NewFinalRestart(),
		)
		return runner.Run(ctx)

	default:
		return fmt.Errorf("unsupported input mode: %s", state.InputMode)
	}
}
