package iso

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Manager struct {
	loopDevice string
	mountPoint string
	isRoot     bool
	logFn      func(string)
}

func NewManager() *Manager {
	return &Manager{
		logFn: func(string) {}, // no-op by default
	}
}

func (m *Manager) SetLogger(logFn func(string)) {
	m.logFn = logFn
}

func (m *Manager) Mount(path string) (string, error) {
	m.logFn("Checking for existing mount...")
	if existing, err := findExistingMount(path); err == nil && existing != nil {
		m.loopDevice = existing.loopDevice
		m.mountPoint = existing.mountPoint
		m.isRoot = false
		m.logFn("Found existing mount at: " + m.mountPoint)
		return m.mountPoint, nil
	}

	m.logFn("Setting up loop device...")
	setupCmd := exec.Command("udisksctl", "loop-setup", "-f", path)
	setupOut, err := setupCmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(setupOut)))
	}

	loopMatch := regexp.MustCompile(`as (/dev/loop\d+)\.`).FindStringSubmatch(string(setupOut))
	if len(loopMatch) < 2 {
		return "", errors.New("failed to parse loop device from: " + string(setupOut))
	}
	m.loopDevice = loopMatch[1]
	m.logFn("Loop device created: " + m.loopDevice)

	m.logFn("Mounting loop device...")
	mountCmd := exec.Command("udisksctl", "mount", "-b", m.loopDevice)
	mountOut, err := mountCmd.CombinedOutput()
	if err != nil {
		return "", errors.New(strings.TrimSpace(string(mountOut)))
	}

	mpMatch := regexp.MustCompile(`at (/\S+)`).FindStringSubmatch(string(mountOut))
	if len(mpMatch) < 2 {
		return "", errors.New("failed to parse mount point from: " + string(mountOut))
	}

	m.mountPoint = strings.TrimRight(mpMatch[1], ".")
	m.isRoot = false
	m.logFn("Successfully mounted at: " + m.mountPoint)
	return m.mountPoint, nil
}

func (m *Manager) MountRoot(path string) (string, error) {
	m.logFn("Creating temporary mount directory...")
	tmp, err := os.MkdirTemp("", "steamer_mnt_")
	if err != nil {
		return "", err
	}

	uid := os.Getuid()
	gid := os.Getgid()
	options := fmt.Sprintf("loop,ro,uid=%d,gid=%d", uid, gid)

	m.logFn("Requesting elevated permissions to mount ISO...")
	cmd := exec.Command("pkexec", "mount", "-o", options, path, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		return "", errors.New(strings.TrimSpace(string(out)))
	}

	m.mountPoint = tmp
	m.isRoot = true
	m.logFn("Successfully mounted with elevated permissions at: " + tmp)
	return m.mountPoint, nil
}

func (m *Manager) Unmount() {
	if m.isRoot && m.mountPoint != "" {
		_ = exec.Command("pkexec", "umount", m.mountPoint).Run()
		_ = os.RemoveAll(m.mountPoint)
		m.mountPoint = ""
		m.isRoot = false
		return
	}

	if m.mountPoint != "" && m.loopDevice != "" {
		_ = exec.Command("udisksctl", "unmount", "-b", m.loopDevice).Run()
	}
	if m.loopDevice != "" {
		_ = exec.Command("udisksctl", "loop-delete", "-b", m.loopDevice).Run()
	}

	m.mountPoint = ""
	m.loopDevice = ""
}

type existingMount struct {
	loopDevice string
	mountPoint string
}

func findExistingMount(path string) (*existingMount, error) {
	cmd := exec.Command("losetup", "-j", path)
	out, err := cmd.CombinedOutput()
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

	mpCmd := exec.Command("lsblk", "-no", "MOUNTPOINT", loop)
	mpOut, _ := mpCmd.CombinedOutput()
	mp := strings.TrimSpace(string(mpOut))
	if mp != "" {
		return &existingMount{loopDevice: loop, mountPoint: mp}, nil
	}

	mountCmd := exec.Command("udisksctl", "mount", "-b", loop)
	mountOut, err := mountCmd.CombinedOutput()
	if err == nil {
		mpMatch := regexp.MustCompile(`at (/\S+)`).FindStringSubmatch(string(mountOut))
		if len(mpMatch) >= 2 {
			return &existingMount{loopDevice: loop, mountPoint: strings.TrimRight(mpMatch[1], ".")}, nil
		}
	}

	return nil, errors.New("not mounted")
}

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

func indexOf(list []string, value string) int {
	for i, v := range list {
		if v == value {
			return i
		}
	}
	return -1
}
