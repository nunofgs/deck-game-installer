// Package redist installs Windows redistributables into Proton prefixes.
package redist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MissingToolError means neither protontricks nor a safe winetricks fallback is available.
type MissingToolError struct {
	Reason string
}

func (e *MissingToolError) Error() string {
	if e.Reason == "" {
		return "protontricks/winetricks not available"
	}
	return e.Reason
}

// CommandRunner runs a command and returns combined output.
type CommandRunner func(ctx context.Context, name string, args []string, env []string) ([]byte, error)

// Installer installs winetricks verbs for a Steam shortcut.
type Installer struct {
	lookPath func(string) (string, error)
	run      CommandRunner
}

// NewInstaller creates an installer using PATH command discovery.
func NewInstaller() *Installer {
	return &Installer{
		lookPath: exec.LookPath,
		run:      runCommand,
	}
}

// NewInstallerForTest creates an installer with fake command hooks.
func NewInstallerForTest(lookPath func(string) (string, error), run CommandRunner) *Installer {
	return &Installer{lookPath: lookPath, run: run}
}

// InstallRedists installs verbs into the shortcut's Proton prefix.
// protonDir is the installation directory of the configured Proton version.
// When non-empty and winetricks is available, winetricks is run with Proton's
// wine binary directly, bypassing protontricks' app-ID lookup (which fails for
// non-Steam shortcuts with CRC-based app IDs).
func (i *Installer) InstallRedists(ctx context.Context, appID int32, prefixRoot string, protonDir string, verbs []string, log func(string)) error {
	verbs = dedupe(verbs)
	if len(verbs) == 0 {
		return nil
	}

	// Prefer winetricks + Proton wine when we have a known prefix and Proton dir.
	if protonDir != "" {
		if winetricks, err := i.lookPath("winetricks"); err == nil {
			wineBin := protonWineBinary(protonDir)
			return i.runWithWinetricks(ctx, winetricks, wineBin, prefixRoot, verbs, log)
		}
	}

	// Fall back to protontricks (works for Steam store apps; may fail for
	// non-Steam shortcuts if protontricks can't resolve the CRC app ID).
	appIDText := strconv.FormatUint(uint64(uint32(appID)), 10)
	if protontricks, err := i.lookPath("protontricks"); err == nil {
		if _, err := os.Stat(prefixRoot); err != nil {
			log("Creating Proton prefix with protontricks...")
			if out, runErr := i.run(ctx, protontricks, []string{appIDText, "wineboot"}, nil); runErr != nil {
				return fmt.Errorf("failed to create Proton prefix with protontricks: %w: %s", runErr, strings.TrimSpace(string(out)))
			}
		}
		args := append([]string{appIDText}, verbs...)
		log("Installing redistributables with protontricks: " + strings.Join(verbs, ", "))
		if out, err := i.run(ctx, protontricks, args, nil); err != nil {
			return fmt.Errorf("protontricks failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Last resort: winetricks without Proton wine (prefix must already exist).
	winetricks, err := i.lookPath("winetricks")
	if err != nil {
		return &MissingToolError{Reason: "protontricks is not installed"}
	}
	if _, err := os.Stat(prefixRoot); err != nil {
		return &MissingToolError{Reason: "winetricks is installed, but the Proton prefix does not exist yet; protontricks is needed to create it safely"}
	}
	return i.runWithWinetricks(ctx, winetricks, "", prefixRoot, verbs, log)
}

// runWithWinetricks runs winetricks verbs with WINEPREFIX set to prefixRoot.
// When wineBin is non-empty, WINE is also set. The prefix directory is created
// if absent so wine can chdir into it; winetricks handles full initialization.
func (i *Installer) runWithWinetricks(ctx context.Context, winetricks, wineBin, prefixRoot string, verbs []string, log func(string)) error {
	env := []string{"WINEPREFIX=" + prefixRoot}
	if wineBin != "" {
		env = append(env, "WINE="+wineBin)
		if err := os.MkdirAll(prefixRoot, 0o755); err != nil {
			return fmt.Errorf("failed to create prefix directory: %w", err)
		}
	}
	args := append([]string{"-q"}, verbs...)
	log("Installing redistributables with winetricks: " + strings.Join(verbs, ", "))
	if out, err := i.run(ctx, winetricks, args, env); err != nil {
		return fmt.Errorf("winetricks failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// protonWineBinary returns the path to the wine binary inside a Proton directory.
func protonWineBinary(protonDir string) string {
	for _, rel := range []string{"files/bin/wine", "bin/wine"} {
		p := filepath.Join(protonDir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// IsMissingTool reports whether err is a MissingToolError.
func IsMissingTool(err error) bool {
	var missing *MissingToolError
	return errors.As(err, &missing)
}

func runCommand(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
