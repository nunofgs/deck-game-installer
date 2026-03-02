// Command vdf-print parses a Steam VDF file (text or binary shortcuts.vdf /
// config.vdf) and prints the resulting data structure to stdout. Useful for
// inspecting Steam configuration files.
//
// Usage: vdf-print /path/to/file.vdf
package main

import (
	"fmt"
	"os"

	"deck-game-installer/steam"
	"deck-game-installer/vdf"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintf(os.Stderr, "vdf-print — parse and print a Steam VDF file (text or binary)\n\n")
		fmt.Fprintf(os.Stderr, "Usage: vdf-print <path-to-file.vdf>\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  vdf-print ~/.steam/steam/userdata/*/config/shortcuts.vdf\n")
		fmt.Fprintf(os.Stderr, "  vdf-print ~/.steam/steam/config/config.vdf\n")
		if len(os.Args) == 1 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vdf-print: failed to read %q: %v\n", path, err)
		os.Exit(1)
	}

	// Try binary VDF first (shortcuts.vdf), then text VDF (config.vdf).
	if result, err := steam.ReadBinaryVDF(data); err == nil {
		fmt.Printf("%#v\n", result)
	} else if result, err := vdf.Parse(string(data)); err == nil {
		fmt.Printf("%#v\n", result)
	} else {
		fmt.Fprintf(os.Stderr, "vdf-print: not a recognised VDF format: %v\n", err)
		os.Exit(1)
	}
}
