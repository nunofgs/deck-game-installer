package iso

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type SMBInfo struct {
	Server  string
	Share   string
	RelPath string
}

type SMBMount struct {
	MountPoint string
}

func ParseKioPath(path string) *SMBInfo {
	re := regexp.MustCompile(`/smb/([^/]+)/([^/]+)/(.*)$`)
	match := re.FindStringSubmatch(path)
	if len(match) < 4 {
		return nil
	}
	return &SMBInfo{
		Server:  match[1],
		Share:   match[2],
		RelPath: match[3],
	}
}

func RemountSMB(info *SMBInfo) (*SMBMount, error) {
	mnt, err := os.MkdirTemp("", "steamer_smb_")
	if err != nil {
		return nil, err
	}

	shareUNC := "//" + info.Server + "/" + info.Share
	options := "ro,guest"

	cmd := exec.Command("pkexec", "mount", "-t", "cifs", shareUNC, mnt, "-o", options)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(mnt)
		return nil, errors.New(strings.TrimSpace(string(out)))
	}

	absMount, _ := filepath.Abs(mnt)
	return &SMBMount{MountPoint: absMount}, nil
}

func (m *SMBMount) Unmount() error {
	if m == nil || m.MountPoint == "" {
		return nil
	}
	_ = exec.Command("pkexec", "umount", m.MountPoint).Run()
	return os.RemoveAll(m.MountPoint)
}
