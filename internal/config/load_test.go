package config

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// memFS is an in-memory FS for testing.
type memFS struct {
	files   map[string][]byte
	homeDir string
}

func newMemFS(home string) *memFS {
	return &memFS{
		files:   make(map[string][]byte),
		homeDir: home,
	}
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memFS) MkdirAll(_ string, _ os.FileMode) error { return nil }

func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return fakeFileInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (m *memFS) UserHomeDir() (string, error) { return m.homeDir, nil }

func (m *memFS) Glob(_ string) ([]string, error) { return nil, nil }

// fakeFileInfo satisfies os.FileInfo for Stat calls.
type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		check   func(t *testing.T, cfg Config)
		wantErr bool
	}{
		{
			name: "valid full config",
			toml: `
theme = "tokyonight"
prefix = "C-a"

[bar]
preset = "minimal"

[sessions]
auto_cleanup_tmp = false
default_shell = "/bin/zsh"

[recipes]
paths = ["/custom/recipes"]

[sync]
target = "ghostty"
ghostty_config = "/home/user/.config/ghostty/config"
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.Theme != "tokyonight" {
					t.Errorf("Theme = %q, want %q", cfg.Theme, "tokyonight")
				}
				if cfg.Prefix != "C-a" {
					t.Errorf("Prefix = %q, want %q", cfg.Prefix, "C-a")
				}
				if cfg.Bar.Preset != "minimal" {
					t.Errorf("Bar.Preset = %q, want %q", cfg.Bar.Preset, "minimal")
				}
				if cfg.Sessions.AutoCleanupTmp != false {
					t.Errorf("Sessions.AutoCleanupTmp = %v, want false", cfg.Sessions.AutoCleanupTmp)
				}
				if cfg.Sessions.DefaultShell != "/bin/zsh" {
					t.Errorf("Sessions.DefaultShell = %q, want %q", cfg.Sessions.DefaultShell, "/bin/zsh")
				}
				if len(cfg.Recipes.Paths) != 1 || cfg.Recipes.Paths[0] != "/custom/recipes" {
					t.Errorf("Recipes.Paths = %v, want [/custom/recipes]", cfg.Recipes.Paths)
				}
				if cfg.Sync.Target != "ghostty" {
					t.Errorf("Sync.Target = %q, want %q", cfg.Sync.Target, "ghostty")
				}
				if cfg.Sync.GhosttyConfig != "/home/user/.config/ghostty/config" {
					t.Errorf("Sync.GhosttyConfig = %q, want the explicit path", cfg.Sync.GhosttyConfig)
				}
			},
		},
		{
			name: "missing fields get defaults",
			toml: `theme = "nord"`,
			check: func(t *testing.T, cfg Config) {
				if cfg.Theme != "nord" {
					t.Errorf("Theme = %q, want %q", cfg.Theme, "nord")
				}
				if cfg.Prefix != "C-Space" {
					t.Errorf("Prefix = %q, want default %q", cfg.Prefix, "C-Space")
				}
				if cfg.Bar.Preset != "default" {
					t.Errorf("Bar.Preset = %q, want default %q", cfg.Bar.Preset, "default")
				}
				if cfg.Sync.Target != "none" {
					t.Errorf("Sync.Target = %q, want default %q", cfg.Sync.Target, "none")
				}
				if cfg.Sync.GhosttyConfig != "auto" {
					t.Errorf("Sync.GhosttyConfig = %q, want default %q", cfg.Sync.GhosttyConfig, "auto")
				}
				if len(cfg.Recipes.Paths) != 1 || cfg.Recipes.Paths[0] != "~/.zmux/recipes" {
					t.Errorf("Recipes.Paths = %v, want default", cfg.Recipes.Paths)
				}
			},
		},
		{
			name: "empty file gets all defaults",
			toml: ``,
			check: func(t *testing.T, cfg Config) {
				d := DefaultConfig()
				if cfg.Theme != d.Theme {
					t.Errorf("Theme = %q, want %q", cfg.Theme, d.Theme)
				}
				if cfg.Prefix != d.Prefix {
					t.Errorf("Prefix = %q, want %q", cfg.Prefix, d.Prefix)
				}
				if cfg.Bar.Preset != d.Bar.Preset {
					t.Errorf("Bar.Preset = %q, want %q", cfg.Bar.Preset, d.Bar.Preset)
				}
			},
		},
		{
			// Regression: the old zero-value heuristic flipped a bare
			// explicit false back to the true default.
			name: "explicit auto_cleanup_tmp=false survives without default_shell",
			toml: "[sessions]\nauto_cleanup_tmp = false\n",
			check: func(t *testing.T, cfg Config) {
				if cfg.Sessions.AutoCleanupTmp {
					t.Error("explicit auto_cleanup_tmp=false was overridden to true")
				}
			},
		},
		{
			// Regression: disabling every segment used to read as "section
			// absent" and re-enable all seven; partial sections must also
			// only default the keys not written.
			name: "explicit bar segment toggles survive",
			toml: "[bar.segments]\nworkspace = false\ngit = false\nlang = false\nclock = false\ndirectory = false\nprocess = false\ngroup = false\n",
			check: func(t *testing.T, cfg Config) {
				if cfg.Bar.Segments.Workspace || cfg.Bar.Segments.Git ||
					cfg.Bar.Segments.Lang || cfg.Bar.Segments.Clock ||
					cfg.Bar.Segments.Directory || cfg.Bar.Segments.Process ||
					cfg.Bar.Segments.Group {
					t.Errorf("all-false segments re-enabled: %+v", cfg.Bar.Segments)
				}
			},
		},
		{
			name: "partial bar segments default the unwritten keys",
			toml: "[bar.segments]\ngit = false\n",
			check: func(t *testing.T, cfg Config) {
				if cfg.Bar.Segments.Git {
					t.Error("explicit git=false was overridden")
				}
				if !cfg.Bar.Segments.Workspace || !cfg.Bar.Segments.Clock {
					t.Errorf("unwritten segments should default true: %+v", cfg.Bar.Segments)
				}
			},
		},
		{
			name:    "invalid TOML returns error",
			toml:    `this is not [valid toml`,
			wantErr: true,
		},
		{
			name:    "nonexistent file returns error",
			toml:    "", // won't be written
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newMemFS("/home/testuser")
			path := "/home/testuser/.zmux.toml"

			// For the "nonexistent file" case, don't write the file
			if tt.name != "nonexistent file returns error" {
				fs.files[path] = []byte(tt.toml)
			}

			cfg, err := Load(fs, path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// The bar is always two-line (plan 024): the removed "single" layout, an empty
// value, and any unknown string must all normalize to two-line on load, so the
// bar can never end up a single reflowing line. "split" is the only alternate.
func TestLoad_LayoutNormalization(t *testing.T) {
	cases := map[string]string{
		`[bar]` + "\nlayout = 'single'\n":   "two-line",
		`[bar]` + "\nlayout = 'one-line'\n": "two-line",
		`[bar]` + "\npreset = 'minimal'\n":  "two-line", // layout omitted
		`[bar]` + "\nlayout = 'two-line'\n": "two-line",
		`[bar]` + "\nlayout = 'split'\n":    "split",
	}
	for toml, want := range cases {
		fs := newMemFS("/home/testuser")
		path := "/home/testuser/.zmux.toml"
		fs.files[path] = []byte(toml)
		cfg, err := Load(fs, path)
		if err != nil {
			t.Fatalf("Load(%q): %v", toml, err)
		}
		if cfg.Bar.Layout != want {
			t.Errorf("Load(%q): layout = %q, want %q", toml, cfg.Bar.Layout, want)
		}
	}
}

func TestSave(t *testing.T) {
	fs := newMemFS("/home/testuser")
	path := "/home/testuser/.zmux.toml"

	cfg := DefaultConfig()
	cfg.Theme = "dracula"

	if err := Save(fs, path, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file was written
	data, ok := fs.files[path]
	if !ok {
		t.Fatal("file not written")
	}

	// Verify it's valid TOML by loading it back
	loaded, err := Load(fs, path)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if loaded.Theme != "dracula" {
		t.Errorf("Theme = %q after round-trip, want %q", loaded.Theme, "dracula")
	}

	// Verify the written data contains expected content
	content := string(data)
	if !strings.Contains(content, "dracula") {
		t.Error("saved TOML should contain theme name")
	}
}

func TestConfigExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		fs := newMemFS("/home/testuser")
		fs.files["/home/testuser/.zmux.toml"] = []byte("theme = \"test\"")
		if !ConfigExists(fs) {
			t.Error("ConfigExists = false, want true")
		}
	})

	t.Run("missing", func(t *testing.T) {
		fs := newMemFS("/home/testuser")
		if ConfigExists(fs) {
			t.Error("ConfigExists = true, want false")
		}
	})
}

func TestConfigPath(t *testing.T) {
	fs := newMemFS("/home/testuser")
	path, err := ConfigPath(fs)
	if err != nil {
		t.Fatalf("ConfigPath error: %v", err)
	}
	if path != "/home/testuser/.zmux.toml" {
		t.Errorf("ConfigPath = %q, want %q", path, "/home/testuser/.zmux.toml")
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"tilde slash", "~/config", "/home/testuser/config"},
		{"bare tilde", "~", "/home/testuser"},
		{"absolute path", "/etc/config", "/etc/config"},
		{"relative path", "config", "config"},
		{"tilde in middle", "/path/~/file", "/path/~/file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newMemFS("/home/testuser")
			got, err := ExpandHome(fs, tt.path)
			if err != nil {
				t.Fatalf("ExpandHome error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ExpandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// errFS is an FS that always fails on UserHomeDir.
type errFS struct{ memFS }

func (errFS) UserHomeDir() (string, error) {
	return "", errors.New("no home")
}

func TestExpandHomeError(t *testing.T) {
	fs := &errFS{}
	_, err := ExpandHome(fs, "~/config")
	if err == nil {
		t.Error("expected error when home dir unavailable")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "ayu-dark" {
		t.Errorf("default Theme = %q, want %q", cfg.Theme, "ayu-dark")
	}
	if cfg.Prefix != "C-Space" {
		t.Errorf("default Prefix = %q, want %q", cfg.Prefix, "C-Space")
	}
	if cfg.Bar.Preset != "default" {
		t.Errorf("default Bar.Preset = %q, want %q", cfg.Bar.Preset, "default")
	}
	if cfg.Sessions.AutoCleanupTmp != true {
		t.Error("default Sessions.AutoCleanupTmp should be true")
	}
	if cfg.Sync.Target != "none" {
		t.Errorf("default Sync.Target = %q, want %q", cfg.Sync.Target, "none")
	}
	if cfg.Sync.GhosttyConfig != "auto" {
		t.Errorf("default Sync.GhosttyConfig = %q, want %q", cfg.Sync.GhosttyConfig, "auto")
	}
	if len(cfg.Recipes.Paths) != 1 || cfg.Recipes.Paths[0] != "~/.zmux/recipes" {
		t.Errorf("default Recipes.Paths = %v, want ~/.zmux/recipes", cfg.Recipes.Paths)
	}
}
