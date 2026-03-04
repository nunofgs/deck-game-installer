// Package iso provides ISO mounting and installer detection functionality.
package iso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	reLoopDevice = regexp.MustCompile(`as (/dev/loop\d+)\.`)
	reMountPoint = regexp.MustCompile(`at (/\S+)`)
)

// Manager handles ISO mounting and unmounting operations.
type Manager struct {
	loopDevice  string
	mountPoint  string
	isRoot      bool
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

// Mount mounts an ISO file using udisksctl.
// It first checks for existing mounts before creating a new one.
func (m *Manager) Mount(ctx context.Context, path string) (string, error) {
	m.logFn("Checking for existing mount...")
	m.logFn("ISO path: " + path)

	// Check for existing mount
	if existing, err := findExistingMount(path); err == nil && existing != nil {
		m.loopDevice = existing.loopDevice
		m.mountPoint = existing.mountPoint
		m.isRoot = false
		m.wasExisting = true
		m.logFn("Found existing mount at: " + m.mountPoint)
		return m.mountPoint, nil
	}

	// Set up loop device
	m.logFn("Setting up loop device...")
	setupCmd := exec.CommandContext(ctx, "udisksctl", "loop-setup", "-f", path)
	setupOut, err := setupCmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(setupOut)))
	}

	loopMatch := reLoopDevice.FindStringSubmatch(string(setupOut))
	if len(loopMatch) < 2 {
		return "", errors.New("failed to parse loop device from: " + string(setupOut))
	}
	m.loopDevice = loopMatch[1]
	m.logFn("Loop device created: " + m.loopDevice)

	// Mount the loop device
	m.logFn("Mounting loop device...")
	mountCmd := exec.CommandContext(ctx, "udisksctl", "mount", "-b", m.loopDevice)
	mountOut, err := mountCmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(mountOut)))
	}

	mpMatch := reMountPoint.FindStringSubmatch(string(mountOut))
	if len(mpMatch) < 2 {
		return "", errors.New("failed to parse mount point from: " + string(mountOut))
	}

	m.mountPoint = strings.TrimRight(mpMatch[1], ".")
	m.isRoot = false
	m.wasExisting = false
	m.logFn("Successfully mounted at: " + m.mountPoint)

	if err := waitForMount(ctx, m.mountPoint, m.logFn); err != nil {
		return "", err
	}
	return m.mountPoint, nil
}

// MountRoot mounts an ISO file using pkexec for elevated permissions.
// This is used when udisksctl fails or isn't available.
func (m *Manager) MountRoot(ctx context.Context, path string) (string, error) {
	m.logFn("Creating temporary mount directory...")
	tmp, err := os.MkdirTemp("", "deck-game-installer_mnt_")
	if err != nil {
		return "", err
	}

	uid := os.Getuid()
	gid := os.Getgid()
	options := fmt.Sprintf("loop,ro,uid=%d,gid=%d", uid, gid)

	m.logFn("Requesting elevated permissions to mount ISO...")
	cmd := exec.CommandContext(ctx, "pkexec", "mount", "-o", options, path, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		if removeErr := os.RemoveAll(tmp); removeErr != nil {
			m.logFn("Warning: failed to remove temp mount dir: " + removeErr.Error())
		}
		return "", errors.New(strings.TrimSpace(string(out)))
	}

	m.mountPoint = tmp
	m.isRoot = true
	m.wasExisting = false
	m.logFn("Successfully mounted with elevated permissions at: " + tmp)

	if err := waitForMount(ctx, m.mountPoint, m.logFn); err != nil {
		return "", err
	}
	return m.mountPoint, nil
}

// Unmount unmounts the ISO and cleans up resources.
// It does nothing if the mount was already existing when Mount was called.
func (m *Manager) Unmount() {
	// Don't unmount if we found an existing mount
	if m.wasExisting {
		m.mountPoint = ""
		m.loopDevice = ""
		m.wasExisting = false
		return
	}

	if m.isRoot && m.mountPoint != "" {
		if err := exec.Command("pkexec", "umount", m.mountPoint).Run(); err != nil {
			m.logFn("Warning: failed to unmount ISO: " + err.Error())
		}
		if err := os.RemoveAll(m.mountPoint); err != nil {
			m.logFn("Warning: failed to remove mount dir: " + err.Error())
		}
		m.mountPoint = ""
		m.isRoot = false
		return
	}

	if m.mountPoint != "" && m.loopDevice != "" {
		if err := exec.Command("udisksctl", "unmount", "-b", m.loopDevice).Run(); err != nil {
			m.logFn("Warning: failed to unmount loop device: " + err.Error())
		}
	}
	if m.loopDevice != "" {
		if err := exec.Command("udisksctl", "loop-delete", "-b", m.loopDevice).Run(); err != nil {
			m.logFn("Warning: failed to delete loop device: " + err.Error())
		}
	}

	m.mountPoint = ""
	m.loopDevice = ""
}

// existingMount holds information about an already-mounted ISO.
type existingMount struct {
	loopDevice string
	mountPoint string
}

// findExistingMount checks if the ISO is already mounted.
func findExistingMount(path string) (*existingMount, error) {
	basename := filepath.Base(path)
	absPath, _ := filepath.Abs(path)

	// Check mount table first
	cmd := exec.Command("mount")
	out, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if (strings.Contains(line, absPath) || strings.Contains(line, basename)) && strings.Contains(line, " on ") {
				parts := strings.Split(line, " on ")
				if len(parts) >= 2 {
					mountParts := strings.Split(parts[1], " type ")
					if len(mountParts) >= 1 {
						mp := strings.TrimSpace(mountParts[0])
						return &existingMount{loopDevice: "", mountPoint: mp}, nil
					}
				}
			}
		}
	}

	// Check for loop devices
	cmd = exec.Command("losetup", "-j", path)
	out, err = cmd.CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil, errors.New("no existing mount")
	}

	parts := strings.SplitN(string(out), ":", 2)
	if len(parts) < 1 {
		return nil, errors.New("failed to parse loop device")
	}
	loop := strings.TrimSpace(parts[0])
	if loop == "" {
		return nil, errors.New("failed to parse loop device")
	}

	// Check if loop device is mounted
	mpCmd := exec.Command("lsblk", "-no", "MOUNTPOINT", loop)
	mpOut, _ := mpCmd.CombinedOutput()
	mp := strings.TrimSpace(string(mpOut))
	if mp != "" {
		return &existingMount{loopDevice: loop, mountPoint: mp}, nil
	}

	// Try to mount the existing loop device
	mountCmd := exec.Command("udisksctl", "mount", "-b", loop)
	mountOut, err := mountCmd.CombinedOutput()
	if err == nil {
		mpMatch := reMountPoint.FindStringSubmatch(string(mountOut))
		if len(mpMatch) >= 2 {
			return &existingMount{loopDevice: loop, mountPoint: strings.TrimRight(mpMatch[1], ".")}, nil
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
// udisksctl can return before the filesystem is fully available.
func waitForMount(ctx context.Context, mountPoint string, logFn func(string)) error {
	logFn("Waiting for mount point to become accessible: " + mountPoint)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(mountPoint); err == nil {
			logFn("Mount point is accessible.")
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
