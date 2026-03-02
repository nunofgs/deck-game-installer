// Command vdfprint: Pretty-print a Steam VDF file (text or binary)
// Usage: vdfprint /path/to/file.vdf
package main

import (
	"fmt"
	"os"

	"deck-game-installer/steam"
	"deck-game-installer/vdf"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s /path/to/file.vdf\n", os.Args[0])
		os.Exit(1)
	}
	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	// Try binary VDF first, then text VDF
	if result, err := steam.ReadBinaryVDF(data); err == nil {
		fmt.Printf("%#v\n", result)
	} else if result, err := vdf.Parse(string(data)); err == nil {
		fmt.Printf("%#v\n", result)
	} else {
		fmt.Fprintf(os.Stderr, "Failed to parse VDF: %v\n", err)
		os.Exit(1)
	}
}
