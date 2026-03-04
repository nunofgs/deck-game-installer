// Package iso provides ISO mounting and installer detection functionality.
package iso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Manager handles ISO mounting and unmounting operations.
type Manager struct {
	mountPoint  string
	wasExisting bool
	logFn       func(string)
}

// NewManager creates a new ISO manager.
func NewManager() *Manager {
	return &Manager{
		logFn: func(string) {}, // no-op by default
	}
}

// SetLogger sets the logging function for mount operations.
func (m *Manager) SetLogger(logFn func(string)) {
	m.logFn = logFn
}

// MountPoint returns the current mount point, or empty if not mounted.
func (m *Manager) MountPoint() string {
	return m.mountPoint
}

// WasExisting returns true if the mount was already present when Mount was called.
func (m *Manager) WasExisting() bool {
	return m.wasExisting
}

// Mount mounts an ISO file using pkexec, which works reliably on Steam Deck
// regardless of whether the process is part of an active desktop session.
// It first checks for an existing mount to avoid duplicates.
func (m *Manager) Mount(ctx context.Context, path string) (string, error) {
	m.logFn("Checking for existing mount...")

	if existing, err := findExistingMount(path); err == nil && existing != nil {
		m.mountPoint = existing.mountPoint
		m.wasExisting = true
		m.logFn("Found existing mount at: " + m.mountPoint)
		return m.mountPoint, nil
	}

	tmp, err := os.MkdirTemp("", "deck-game-installer_mnt_")
	if err != nil {
		return "", err
	}

	uid := os.Getuid()
	gid := os.Getgid()
	options := fmt.Sprintf("loop,ro,uid=%d,gid=%d", uid, gid)

	m.logFn("Mounting ISO...")
	cmd := exec.CommandContext(ctx, "pkexec", "mount", "-o", options, path, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		if removeErr := os.RemoveAll(tmp); removeErr != nil {
			m.logFn("Warning: failed to remove temp mount dir: " + removeErr.Error())
		}
		return "", errors.New(strings.TrimSpace(string(out)))
	}

	m.mountPoint = tmp
	m.wasExisting = false
	m.logFn("Mounted at: " + tmp)

	if err := waitForMount(ctx, m.mountPoint, m.logFn); err != nil {
		return "", err
	}
	return m.mountPoint, nil
}

// Unmount unmounts the ISO and cleans up resources.
// It does nothing if the mount was already existing when Mount was called.
func (m *Manager) Unmount() {
	if m.wasExisting {
		m.mountPoint = ""
		m.wasExisting = false
		return
	}

	if m.mountPoint != "" {
		if err := exec.Command("pkexec", "umount", m.mountPoint).Run(); err != nil {
			m.logFn("Warning: failed to unmount ISO: " + err.Error())
		}
		if err := os.RemoveAll(m.mountPoint); err != nil {
			m.logFn("Warning: failed to remove mount dir: " + err.Error())
		}
		m.mountPoint = ""
	}
}

// existingMount holds information about an already-mounted ISO.
type existingMount struct {
	mountPoint string
}

// findExistingMount checks if the ISO is already mounted by scanning the mount table.
func findExistingMount(path string) (*existingMount, error) {
	basename := filepath.Base(path)
	absPath, _ := filepath.Abs(path)

	out, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return nil, errors.New("failed to read mount table")
	}

	for _, line := range strings.Split(string(out), "\n") {
		if (strings.Contains(line, absPath) || strings.Contains(line, basename)) && strings.Contains(line, " on ") {
			parts := strings.Split(line, " on ")
			if len(parts) >= 2 {
				mp := strings.TrimSpace(strings.Split(parts[1], " type ")[0])
				return &existingMount{mountPoint: mp}, nil
			}
		}
	}

	return nil, errors.New("not mounted")
}

// FindInstallers scans a directory for installer executables.
// Returns a prioritized list with setup/install files first.
func FindInstallers(root string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".exe") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Prioritize common installer names
	priority := []string{"setup.exe", "install.exe", "installer.exe", "autorun.exe"}
	var prioritized []string
	var middle []string
	var others []string

	for _, exe := range found {
		name := strings.ToLower(filepath.Base(exe))
		if idx := indexOf(priority, name); idx >= 0 {
			prioritized = append(prioritized, exe)
			continue
		}
		if strings.Contains(name, "setup") || strings.Contains(name, "install") || strings.Contains(name, "autorun") {
			middle = append(middle, exe)
			continue
		}
		others = append(others, exe)
	}

	return append(append(prioritized, middle...), others...), nil
}

// waitForMount polls until the mount point is accessible, or times out.
// pkexec mount can return before the filesystem is fully available.
func waitForMount(ctx context.Context, mountPoint string, logFn func(string)) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(mountPoint); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("mount point %q did not become accessible within 10 seconds", mountPoint)
}

// indexOf returns the index of value in list, or -1 if not found.
func indexOf(list []string, value string) int {
	for i, v := range list {
		if v == value {
			return i
		}
	}
	return -1
}
