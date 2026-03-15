package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

// GhosttyTarget reads the current theme from Ghostty's config file.
type GhosttyTarget struct {
	fs         config.FS
	configPath string // explicit path, or "auto" for auto-detection
}

// NewGhosttyTarget creates a GhosttyTarget. If configPath is "auto",
// the target will auto-detect the Ghostty config location.
func NewGhosttyTarget(fs config.FS, configPath string) *GhosttyTarget {
	return &GhosttyTarget{
		fs:         fs,
		configPath: configPath,
	}
}

// Name returns "ghostty".
func (g *GhosttyTarget) Name() string {
	return "ghostty"
}

// Pull reads the `theme = X` line from the Ghostty config and returns the
// theme name.
func (g *GhosttyTarget) Pull() (string, error) {
	path, err := g.resolveConfigPath()
	if err != nil {
		return "", err
	}

	data, err := g.fs.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read ghostty config %q: %w", path, err)
	}

	return parseGhosttyTheme(string(data))
}

// resolveConfigPath determines which config file to read.
func (g *GhosttyTarget) resolveConfigPath() (string, error) {
	if g.configPath != "auto" && g.configPath != "" {
		return g.configPath, nil
	}

	// Auto-detect: try standard locations.
	home, err := g.fs.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	// Check XDG_CONFIG_HOME first, then fallback.
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	candidates := []string{
		filepath.Join(xdgConfig, "ghostty", "config"),
		filepath.Join(home, ".config", "ghostty", "config"),
	}

	for _, path := range candidates {
		if _, err := g.fs.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("ghostty config not found (tried %s)", strings.Join(candidates, ", "))
}

// parseGhosttyTheme extracts the theme name from a Ghostty config file's
// content. It looks for the last `theme = <name>` line.
func parseGhosttyTheme(content string) (string, error) {
	var themeName string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Match lines starting with "theme"
		if !strings.HasPrefix(line, "theme") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if key != "theme" {
			continue
		}

		val := strings.TrimSpace(parts[1])
		// Strip optional quotes.
		val = strings.Trim(val, "\"'")
		val = strings.TrimSpace(val)

		if val != "" {
			themeName = val
		}
	}

	if themeName == "" {
		return "", fmt.Errorf("no theme line found in ghostty config")
	}

	return themeName, nil
}
