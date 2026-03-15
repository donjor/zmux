package config

import (
	"path/filepath"
)

// Config represents the full zmux configuration from ~/.zmux.toml.
type Config struct {
	Theme     string          `toml:"theme"`
	Prefix    string          `toml:"prefix"`
	Bar       BarConfig       `toml:"bar"`
	Sessions  SessionsConfig  `toml:"sessions"`
	Templates TemplatesConfig `toml:"templates"`
	Sync      SyncConfig      `toml:"sync"`
}

// BarConfig holds status bar settings.
type BarConfig struct {
	Preset string `toml:"preset"`
}

// SessionsConfig holds session management settings.
type SessionsConfig struct {
	AutoCleanupTmp bool   `toml:"auto_cleanup_tmp"`
	DefaultShell   string `toml:"default_shell"`
}

// TemplatesConfig holds template discovery paths.
type TemplatesConfig struct {
	Paths []string `toml:"paths"`
}

// SyncConfig holds theme sync settings.
type SyncConfig struct {
	Target       string `toml:"target"`
	GhosttyConfig string `toml:"ghostty_config"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Theme:  "ayu-dark",
		Prefix: "C-Space",
		Bar: BarConfig{
			Preset: "default",
		},
		Sessions: SessionsConfig{
			AutoCleanupTmp: true,
			DefaultShell:   "",
		},
		Templates: TemplatesConfig{
			Paths: []string{"~/.zmux/templates"},
		},
		Sync: SyncConfig{
			Target:        "none",
			GhosttyConfig: "auto",
		},
	}
}

// ConfigPath returns the default config file path (~/.zmux.toml).
func ConfigPath(fs FS) (string, error) {
	home, err := fs.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zmux.toml"), nil
}

// ConfigExists returns true if the config file exists at the default path.
func ConfigExists(fs FS) bool {
	path, err := ConfigPath(fs)
	if err != nil {
		return false
	}
	_, err = fs.Stat(path)
	return err == nil
}
