package sync

import (
	"os"
	"path/filepath"
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

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string        { return f.name }
func (f fakeFileInfo) Size() int64         { return 0 }
func (f fakeFileInfo) Mode() os.FileMode   { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time  { return time.Time{} }
func (f fakeFileInfo) IsDir() bool         { return false }
func (f fakeFileInfo) Sys() any            { return nil }

func TestGhosttyTarget_Pull(t *testing.T) {
	fs := newMemFS("/home/test")
	configPath := "/home/test/.config/ghostty/config"
	fs.files[configPath] = []byte(`
# Ghostty config
font-family = JetBrains Mono
font-size = 14
theme = tokyonight
`)

	target := NewGhosttyTarget(fs, configPath)

	name, err := target.Pull()
	if err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if name != "tokyonight" {
		t.Errorf("Pull() = %q, want %q", name, "tokyonight")
	}
}

func TestGhosttyTarget_Pull_QuotedValue(t *testing.T) {
	fs := newMemFS("/home/test")
	configPath := "/home/test/ghostty.conf"
	fs.files[configPath] = []byte(`theme = "catppuccin-mocha"`)

	target := NewGhosttyTarget(fs, configPath)

	name, err := target.Pull()
	if err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if name != "catppuccin-mocha" {
		t.Errorf("Pull() = %q, want %q", name, "catppuccin-mocha")
	}
}

func TestGhosttyTarget_Pull_LastThemeWins(t *testing.T) {
	fs := newMemFS("/home/test")
	configPath := "/home/test/ghostty.conf"
	fs.files[configPath] = []byte(`
theme = nord
theme = dracula
`)

	target := NewGhosttyTarget(fs, configPath)

	name, err := target.Pull()
	if err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if name != "dracula" {
		t.Errorf("Pull() = %q, want %q (last theme should win)", name, "dracula")
	}
}

func TestGhosttyTarget_Pull_MissingConfig(t *testing.T) {
	fs := newMemFS("/home/test")
	target := NewGhosttyTarget(fs, "/nonexistent/config")

	_, err := target.Pull()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestGhosttyTarget_Pull_NoThemeLine(t *testing.T) {
	fs := newMemFS("/home/test")
	configPath := "/home/test/ghostty.conf"
	fs.files[configPath] = []byte(`
font-family = JetBrains Mono
font-size = 14
`)

	target := NewGhosttyTarget(fs, configPath)

	_, err := target.Pull()
	if err == nil {
		t.Fatal("expected error when no theme line present")
	}
}

func TestGhosttyTarget_Pull_AutoDetect(t *testing.T) {
	fs := newMemFS("/home/test")
	autoPath := filepath.Join("/home/test", ".config", "ghostty", "config")
	fs.files[autoPath] = []byte("theme = rose-pine\n")

	target := NewGhosttyTarget(fs, "auto")

	name, err := target.Pull()
	if err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if name != "rose-pine" {
		t.Errorf("Pull() = %q, want %q", name, "rose-pine")
	}
}

func TestGhosttyTarget_Pull_AutoDetectMissing(t *testing.T) {
	fs := newMemFS("/home/test")
	target := NewGhosttyTarget(fs, "auto")

	_, err := target.Pull()
	if err == nil {
		t.Fatal("expected error when auto-detect finds no config")
	}
}

func TestGhosttyTarget_Name(t *testing.T) {
	target := NewGhosttyTarget(nil, "")
	if name := target.Name(); name != "ghostty" {
		t.Errorf("Name() = %q, want %q", name, "ghostty")
	}
}

func TestParseGhosttyTheme(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "theme = nord",
			want:  "nord",
		},
		{
			name:  "with spaces",
			input: "theme =   ayu-dark  ",
			want:  "ayu-dark",
		},
		{
			name:  "double quoted",
			input: `theme = "dracula"`,
			want:  "dracula",
		},
		{
			name:  "single quoted",
			input: `theme = 'gruvbox'`,
			want:  "gruvbox",
		},
		{
			name:    "empty value",
			input:   "theme = ",
			wantErr: true,
		},
		{
			name:    "no theme key",
			input:   "font-size = 14",
			wantErr: true,
		},
		{
			name:    "comment only",
			input:   "# theme = nord",
			wantErr: true,
		},
		{
			name:  "theme after other config",
			input: "font-size = 14\ntheme = material-darker",
			want:  "material-darker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGhosttyTheme(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseGhosttyTheme() = %q, want %q", got, tt.want)
			}
		})
	}
}
