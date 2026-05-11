// Package redist installs Windows redistributables into Proton prefixes.
package redist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
func (i *Installer) InstallRedists(ctx context.Context, appID int32, prefixRoot string, verbs []string, log func(string)) error {
	verbs = dedupe(verbs)
	if len(verbs) == 0 {
		return nil
	}
	appIDText := strconv.FormatUint(uint64(uint32(appID)), 10)

	if protontricks, err := i.lookPath("protontricks"); err == nil {
		if _, err := os.Stat(prefixRoot); err != nil {
			log("Creating Proton prefix with protontricks...")
			if out, runErr := i.run(ctx, protontricks, []string{appIDText, "wineboot"}, nil); runErr != nil {
				return fmt.Errorf("failed to create Proton prefix with protontricks: %w: %s", runErr, strings.TrimSpace(string(out)))
			}
		}
		args := append([]string{appIDText}, verbs...)
		log("Installing redists with protontricks: " + strings.Join(verbs, ", "))
		if out, err := i.run(ctx, protontricks, args, nil); err != nil {
			return fmt.Errorf("protontricks failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	winetricks, err := i.lookPath("winetricks")
	if err != nil {
		return &MissingToolError{Reason: "protontricks is not installed"}
	}
	if _, err := os.Stat(prefixRoot); err != nil {
		return &MissingToolError{Reason: "winetricks is installed, but the Proton prefix does not exist yet; protontricks is needed to create it safely"}
	}

	args := append([]string{"-q"}, verbs...)
	log("Installing redists with winetricks: " + strings.Join(verbs, ", "))
	if out, err := i.run(ctx, winetricks, args, []string{"WINEPREFIX=" + prefixRoot}); err != nil {
		return fmt.Errorf("winetricks failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
