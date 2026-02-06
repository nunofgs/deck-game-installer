package iso

import (
	"errors"
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
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Mount(path string) (string, error) {
	if existing, err := findExistingMount(path); err == nil && existing != nil {
		m.loopDevice = existing.loopDevice
		m.mountPoint = existing.mountPoint
		m.isRoot = false
		return m.mountPoint, nil
	}

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
	return m.mountPoint, nil
}

func (m *Manager) MountRoot(path string) (string, error) {
	tmp, err := os.MkdirTemp("", "steamer_mnt_")
	if err != nil {
		return "", err
	}

	cmd := exec.Command("pkexec", "mount", "-o", "loop,ro", path, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmp)
		return "", errors.New(strings.TrimSpace(string(out)))
	}

	m.mountPoint = tmp
	m.isRoot = true
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
