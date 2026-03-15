package theme

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HTTPClient abstracts HTTP requests for testability.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

const (
	// iterm2TarballURL is the GitHub tarball URL for the iterm2-color-schemes repo.
	iterm2TarballURL = "https://github.com/mbadolato/iTerm2-Color-Schemes/archive/refs/heads/master.tar.gz"

	// ghosttyDirPrefix is the prefix inside the tarball for the ghostty directory.
	// The tarball extracts as "iTerm2-Color-Schemes-master/Ghostty/"
	ghosttyDirPrefix = "iTerm2-Color-Schemes-master/Ghostty/"
)

// Download downloads Ghostty-format theme files from the iterm2-color-schemes
// GitHub repository and stores them in destDir. Returns the count of themes
// downloaded.
func Download(client HTTPClient, destDir string) (int, error) {
	resp, err := client.Get(iterm2TarballURL)
	if err != nil {
		return 0, fmt.Errorf("download tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download tarball: HTTP %d", resp.StatusCode)
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, fmt.Errorf("create dest dir: %w", err)
	}

	return extractGhosttyThemes(resp.Body, destDir)
}

// extractGhosttyThemes reads a tar.gz stream and extracts files from the
// Ghostty/ directory into destDir, returning the count extracted.
func extractGhosttyThemes(r io.Reader, destDir string) (int, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return 0, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	count := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("read tar: %w", err)
		}

		// Only extract regular files from the Ghostty directory.
		if header.Typeflag != tar.TypeReg {
			continue
		}

		if !strings.HasPrefix(header.Name, ghosttyDirPrefix) {
			continue
		}

		// Get just the filename (skip the path prefix).
		name := strings.TrimPrefix(header.Name, ghosttyDirPrefix)
		if name == "" || strings.Contains(name, "/") {
			continue
		}

		destPath := filepath.Join(destDir, name)

		data, err := io.ReadAll(tr)
		if err != nil {
			return count, fmt.Errorf("read %s: %w", name, err)
		}

		if err := os.WriteFile(destPath, data, 0o644); err != nil {
			return count, fmt.Errorf("write %s: %w", name, err)
		}

		count++
	}

	return count, nil
}
