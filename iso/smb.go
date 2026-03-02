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
)

// SMBInfo holds parsed information about an SMB path.
type SMBInfo struct {
	Server  string // SMB server hostname
	Share   string // Share name
	RelPath string // Relative path within the share
}

// SMBMount represents a mounted SMB share.
type SMBMount struct {
	MountPoint  string // Local mount point
	wasExisting bool   // True if we found an existing mount
}

// ParseSMBPath extracts SMB info from either a standard smb:// URI or a KDE
// KIO path (/smb/server/share/...). Returns nil if the path is neither.
func ParseSMBPath(path string) *SMBInfo {
	// Standard URI: smb://server/share/rel/path
	if re := regexp.MustCompile(`^smb://([^/]+)/([^/]+)/?(.*)`); true {
		if m := re.FindStringSubmatch(path); len(m) == 4 {
			return &SMBInfo{Server: m[1], Share: m[2], RelPath: m[3]}
		}
	}
	// KDE KIO path: /run/user/.../smb/server/share/rel/path or /smb/server/share/rel/path
	if m := regexp.MustCompile(`/smb/([^/]+)/([^/]+)/?(.*)`).FindStringSubmatch(path); len(m) == 4 {
		return &SMBInfo{Server: m[1], Share: m[2], RelPath: m[3]}
	}
	return nil
}

// RemountSMB mounts an SMB share locally for direct access.
// If the share is already mounted, returns the existing mount point.
func RemountSMB(ctx context.Context, info *SMBInfo) (*SMBMount, error) {
	shareUNC := "//" + info.Server + "/" + info.Share

	// Check for existing mount
	if existing := findExistingSMBMount(shareUNC); existing != "" {
		return &SMBMount{
			MountPoint:  existing,
			wasExisting: true,
		}, nil
	}

	// Create mount point
	mnt, err := os.MkdirTemp("", "deck-game-installer_smb_")
	if err != nil {
		return nil, err
	}

	uid := os.Getuid()
	gid := os.Getgid()
	options := fmt.Sprintf("ro,guest,uid=%d,gid=%d", uid, gid)

	// Mount with elevated permissions
	cmd := exec.CommandContext(ctx, "pkexec", "mount", "-t", "cifs", shareUNC, mnt, "-o", options)
	if out, err := cmd.CombinedOutput(); err != nil {
		if removeErr := os.RemoveAll(mnt); removeErr != nil {
			return nil, fmt.Errorf("%s (also failed to remove temp dir: %w)", strings.TrimSpace(string(out)), removeErr)
		}
		return nil, errors.New(strings.TrimSpace(string(out)))
	}

	absMount, _ := filepath.Abs(mnt)
	return &SMBMount{
		MountPoint:  absMount,
		wasExisting: false,
	}, nil
}

// findExistingSMBMount checks if an SMB share is already mounted.
func findExistingSMBMount(shareUNC string) string {
	cmd := exec.Command("mount", "-t", "cifs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	shareUNCLower := strings.ToLower(shareUNC)

	// Parse mount output: //server/share on /mount/point type cifs (options)
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

// Unmount unmounts the SMB share and removes the mount point.
// Does nothing if this was an existing mount we didn't create.
func (m *SMBMount) Unmount() error {
	if m == nil || m.MountPoint == "" {
		return nil
	}

	// Don't unmount if we found an existing mount
	if m.wasExisting {
		m.MountPoint = ""
		return nil
	}

	if err := exec.Command("pkexec", "umount", m.MountPoint).Run(); err != nil {
		return fmt.Errorf("failed to unmount SMB share: %w", err)
	}
	return os.RemoveAll(m.MountPoint)
}

// WasExisting returns true if this mount was already present.
func (m *SMBMount) WasExisting() bool {
	return m != nil && m.wasExisting
}

// GetFullPath returns the full local path to the file within the SMB share.
func (m *SMBMount) GetFullPath(info *SMBInfo) string {
	if m == nil || m.MountPoint == "" {
		return ""
	}
	return filepath.Join(m.MountPoint, info.RelPath)
}
