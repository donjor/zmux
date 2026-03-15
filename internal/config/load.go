package config

import (
	"fmt"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// Load reads a TOML config file and returns a Config with defaults applied
// for any missing/zero-value fields.
func Load(fs FS, path string) (Config, error) {
	expanded, err := ExpandHome(fs, path)
	if err != nil {
		return Config{}, fmt.Errorf("expand path: %w", err)
	}

	data, err := fs.ReadFile(expanded)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	return cfg, nil
}

// Save marshals a Config to TOML and writes it to the given path.
func Save(fs FS, path string, cfg Config) error {
	expanded, err := ExpandHome(fs, path)
	if err != nil {
		return fmt.Errorf("expand path: %w", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := fs.WriteFile(expanded, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ExpandHome expands a leading ~ in a path to the user's home directory.
func ExpandHome(fs FS, path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := fs.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	if path == "~" {
		return home, nil
	}

	// Handle ~/something
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(os.PathSeparator)) {
		return home + path[1:], nil
	}

	// ~other is not supported
	return path, nil
}

// applyDefaults fills in zero-value fields with defaults.
func applyDefaults(cfg *Config) {
	defaults := DefaultConfig()

	if cfg.Theme == "" {
		cfg.Theme = defaults.Theme
	}
	if cfg.Prefix == "" {
		cfg.Prefix = defaults.Prefix
	}
	if cfg.Bar.Preset == "" {
		cfg.Bar.Preset = defaults.Bar.Preset
	}
	// For auto_cleanup_tmp: TOML unmarshals missing bools as false, but our
	// default is true. We check if the sessions section was entirely absent
	// (both fields at zero value) and apply the default in that case.
	// If the user explicitly wrote [sessions] with auto_cleanup_tmp = false,
	// DefaultShell would likely also be set, or at minimum the section exists.
	// This is imperfect but correct for the common case.
	if !cfg.Sessions.AutoCleanupTmp && cfg.Sessions.DefaultShell == "" {
		cfg.Sessions.AutoCleanupTmp = defaults.Sessions.AutoCleanupTmp
	}

	if cfg.Sync.Target == "" {
		cfg.Sync.Target = defaults.Sync.Target
	}
	if cfg.Sync.GhosttyConfig == "" {
		cfg.Sync.GhosttyConfig = defaults.Sync.GhosttyConfig
	}
	if len(cfg.Templates.Paths) == 0 {
		cfg.Templates.Paths = defaults.Templates.Paths
	}
}
