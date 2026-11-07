package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const latestURL = "https://api.github.com/repos/nunofgs/deck-game-installer/releases/latest"
const maxArchiveSize = 128 << 20

// Update replaces release builds with a newer GitHub release, if available.
func Update(ctx context.Context, version string, log func(string)) (bool, error) {
	if version == "dev" || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, err
	}

	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return update(ctx, &http.Client{Timeout: 30 * time.Second}, latestURL, executable, version, log)
}

type release struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func update(ctx context.Context, client *http.Client, endpoint, executable, version string, log func(string)) (bool, error) {
	log("Checking for updates...")
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	data, err := download(checkCtx, client, endpoint, 1<<20)
	cancel()
	if err != nil {
		return false, err
	}

	var latest release
	if err := json.Unmarshal(data, &latest); err != nil {
		return false, err
	}

	if latest.Draft || latest.Prerelease || !newer(latest.Tag, version) {
		return false, nil
	}

	for _, asset := range latest.Assets {
		if asset.Name != "deck-game-installer.zip" {
			continue
		}

		log("Downloading " + latest.Tag + "...")
		archive, err := download(ctx, client, asset.URL, maxArchiveSize)
		if err != nil {
			return false, err
		}

		digest := sha256.Sum256(archive)
		if asset.Digest != "sha256:"+hex.EncodeToString(digest[:]) {
			return false, fmt.Errorf("release archive checksum missing or incorrect")
		}

		if err := replace(executable, archive); err != nil {
			return false, err
		}

		log("Updated to " + latest.Tag + ". Restarting...")
		return true, nil
	}

	return false, fmt.Errorf("release %s has no deck-game-installer.zip asset", latest.Tag)
}

func download(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "deck-game-installer")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update request returned %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("update response exceeds size limit")
	}

	return data, nil
}

func replace(executable string, archive []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		if file.Name != "deck-game-installer" {
			continue
		}

		source, err := file.Open()
		if err != nil {
			return err
		}
		defer source.Close()

		target, err := os.CreateTemp(filepath.Dir(executable), ".deck-game-installer-*")
		if err != nil {
			return err
		}
		defer os.Remove(target.Name())
		defer target.Close()

		size, err := io.Copy(target, io.LimitReader(source, maxArchiveSize+1))
		if err != nil {
			return err
		}

		if size == 0 || size > maxArchiveSize {
			return fmt.Errorf("invalid executable size")
		}

		if err := target.Chmod(0o755); err != nil {
			return err
		}

		if err := target.Sync(); err != nil {
			return err
		}

		if err := target.Close(); err != nil {
			return err
		}

		return os.Rename(target.Name(), executable)
	}

	return fmt.Errorf("release archive has no executable")
}

func newer(candidate, current string) bool {
	a, ok := parseVersion(candidate)
	if !ok {
		return false
	}

	b, ok := parseVersion(current)
	if !ok {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}

	return false
}

func parseVersion(version string) ([3]uint64, bool) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != len(result) {
		return result, false
	}

	for i, part := range parts {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return result, false
		}

		result[i] = number
	}

	return result, true
}
