package config

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
	Preset    string      `toml:"preset"`
	Layout    string      `toml:"layout"`    // "single", "two-line", "split"
	Indicator string      `toml:"indicator"` // "none", "numbers", "dots"
	TopBar    string      `toml:"top_bar"`   // "tabs", "dots", "minimal"
	Segments  BarSegments `toml:"segments"`
}

// BarSegments controls which segments are shown in the status bar.
type BarSegments struct {
	Workspace bool `toml:"workspace"`
	Git       bool `toml:"git"`
	Lang      bool `toml:"lang"`
	Clock     bool `toml:"clock"`
	Directory bool `toml:"directory"`
	Process   bool `toml:"process"`
	Group     bool `toml:"group"`
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
	Target        string `toml:"target"`
	GhosttyConfig string `toml:"ghostty_config"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Theme:  "ayu-dark",
		Prefix: "C-Space",
		Bar: BarConfig{
			Preset:    "default",
			Layout:    "two-line",
			Indicator: "dots",
			TopBar:    "tabs",
			Segments: BarSegments{
				Workspace: true,
				Git:       true,
				Lang:      true,
				Clock:     true,
				Directory: true,
				Process:   true,
				Group:     true,
			},
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

// ConfigPath returns the active profile's config file path (~/.zmux.toml, or
// ~/.zzmux.toml when invoked as zzmux). Profile-aware so every caller — including
// app-less TUI components that only hold an FS — inherits isolation for free.
func ConfigPath(fs FS) (string, error) {
	if _, err := fs.UserHomeDir(); err != nil {
		return "", err
	}
	return ActiveProfile(fs).ConfigFile, nil
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
