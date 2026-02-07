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
	shareUNC := "//" + info.Server + "/" + info.Share

	// Check if this share is already mounted
	if existing := findExistingSMBMount(shareUNC); existing != "" {
		return &SMBMount{MountPoint: existing}, nil
	}

	// If we get here, no existing mount was found
	mnt, err := os.MkdirTemp("", "steamer_smb_")
	if err != nil {
		return nil, err
	}

	uid := os.Getuid()
	gid := os.Getgid()
	options := fmt.Sprintf("ro,guest,uid=%d,gid=%d", uid, gid)

	cmd := exec.Command("pkexec", "mount", "-t", "cifs", shareUNC, mnt, "-o", options)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(mnt)
		return nil, errors.New(strings.TrimSpace(string(out)))
	}

	absMount, _ := filepath.Abs(mnt)
	return &SMBMount{MountPoint: absMount}, nil
}

func findExistingSMBMount(shareUNC string) string {
	// Use mount command to find existing CIFS mounts
	cmd := exec.Command("mount", "-t", "cifs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Normalize the share UNC for comparison (case-insensitive)
	shareUNCLower := strings.ToLower(shareUNC)

	// Parse mount output to find matching share
	// Format: //server/share on /mount/point type cifs (options)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.HasPrefix(lineLower, shareUNCLower) || strings.Contains(lineLower, shareUNCLower+" on ") {
			parts := strings.Split(line, " on ")
			if len(parts) >= 2 {
				mountParts := strings.Split(parts[1], " type ")
				if len(mountParts) >= 1 {
					return strings.TrimSpace(mountParts[0])
				}
			}
		}
	}

	return ""
}

func (m *SMBMount) Unmount() error {
	if m == nil || m.MountPoint == "" {
		return nil
	}
	_ = exec.Command("pkexec", "umount", m.MountPoint).Run()
	return os.RemoveAll(m.MountPoint)
}
