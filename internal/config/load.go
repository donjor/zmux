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

	// Second decode into a raw map so defaults only fill keys the user did
	// not write — a plain bool field can't distinguish absent from an
	// explicit false (go-toml/v2 exposes no decoded-keys metadata).
	var raw map[string]any
	_ = toml.Unmarshal(data, &raw)

	applyDefaults(&cfg, raw)
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

// tomlHas reports whether the raw decoded TOML contains the given key path.
func tomlHas(raw map[string]any, keys ...string) bool {
	cur := any(raw)
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		if cur, ok = m[k]; !ok {
			return false
		}
	}
	return true
}

// applyDefaults fills in fields the user did not write with defaults; raw is
// the untyped decode of the same TOML, used for absent-vs-explicit-false.
func applyDefaults(cfg *Config, raw map[string]any) {
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
	// Layout: the bar is always two-line (plan 024). Normalize empty, the
	// removed "single" layout, and any unknown value to two-line so the bar is
	// never a single reflowing line; "split" is the only alternate.
	if cfg.Bar.Layout != "two-line" && cfg.Bar.Layout != "split" {
		cfg.Bar.Layout = defaults.Bar.Layout
	}
	// Bool defaults are true, so they only apply when the key is absent —
	// an explicit `= false` must survive the load.
	if !tomlHas(raw, "sessions", "auto_cleanup_tmp") {
		cfg.Sessions.AutoCleanupTmp = defaults.Sessions.AutoCleanupTmp
	}
	for key, field := range map[string]*bool{
		"workspace": &cfg.Bar.Segments.Workspace,
		"git":       &cfg.Bar.Segments.Git,
		"lang":      &cfg.Bar.Segments.Lang,
		"clock":     &cfg.Bar.Segments.Clock,
		"directory": &cfg.Bar.Segments.Directory,
		"process":   &cfg.Bar.Segments.Process,
		"group":     &cfg.Bar.Segments.Group,
	} {
		if !tomlHas(raw, "bar", "segments", key) {
			*field = true
		}
	}

	if cfg.Sync.Target == "" {
		cfg.Sync.Target = defaults.Sync.Target
	}
	if cfg.Sync.GhosttyConfig == "" {
		cfg.Sync.GhosttyConfig = defaults.Sync.GhosttyConfig
	}
	if len(cfg.Recipes.Paths) == 0 {
		cfg.Recipes.Paths = defaults.Recipes.Paths
	}
}
