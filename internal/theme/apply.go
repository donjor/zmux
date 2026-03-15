package theme

import (
	"fmt"

	"github.com/donjor/zmux/internal/config"
)

// EnvSetter abstracts the ability to set tmux environment variables.
// This avoids importing the tmux package directly (which would cause
// a circular dependency since tmux/conf.go imports theme).
type EnvSetter interface {
	SetEnvironment(key, value string) error
}

// Apply updates the theme name in config, saves it to disk, and sets tmux
// environment variables for the theme colors. The caller is responsible for
// applying the bar preset separately (to avoid a circular dependency between
// theme and bar packages).
func Apply(env EnvSetter, fs config.FS, cfg *config.Config, t Theme, configPath string) error {
	// Update theme name in config.
	cfg.Theme = t.Name

	// Save config to disk.
	if err := config.Save(fs, configPath, *cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Get the semantic palette and set tmux environment variables.
	palette := t.SemanticPalette()

	envVars := map[string]string{
		"ZMUX_THEME":     t.Name,
		"ZMUX_BG":        palette.BG.Hex(),
		"ZMUX_FG":        palette.FG.Hex(),
		"ZMUX_ACCENT":    palette.Accent.Hex(),
		"ZMUX_SURFACE":   palette.Surface.Hex(),
		"ZMUX_ERROR":     palette.Error.Hex(),
		"ZMUX_SUCCESS":   palette.Success.Hex(),
		"ZMUX_INFO":      palette.Info.Hex(),
		"ZMUX_SPECIAL":   palette.Special.Hex(),
		"ZMUX_META":      palette.Meta.Hex(),
		"ZMUX_MUTED":     palette.Muted.Hex(),
		"ZMUX_DIM":       palette.Dim.Hex(),
		"ZMUX_HIGHLIGHT": palette.Highlight.Hex(),
	}

	for key, val := range envVars {
		if err := env.SetEnvironment(key, val); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	return nil
}
