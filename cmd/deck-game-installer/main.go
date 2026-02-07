package main

import (
	"fmt"
	"os"

	"deck-game-installer/gui"
	"deck-game-installer/installer"
)

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  deck-game-installer install <path-to-iso-or-exe>")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "install":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}
		path := os.Args[2]

		logWin := gui.NewLogWindow("Deck Game Installer")
		inst := installer.NewInstaller(logWin)

		go func() {
			if err := inst.Install(path); err != nil {
				logWin.Log("Error: " + err.Error())
			}
		}()

		logWin.Run()
	default:
		usage()
		os.Exit(1)
	}
}
