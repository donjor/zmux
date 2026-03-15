package theme

import (
	"os"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/config"
)

// memFS is an in-memory FS for theme testing.
type memFS struct {
	files   map[string][]byte
	dirs    map[string]bool
	homeDir string
}

func newMemFS(home string) *memFS {
	return &memFS{
		files:   make(map[string][]byte),
		dirs:    make(map[string]bool),
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
	if m.dirs[path] {
		return fakeFileInfo{name: path, isDir: true}, nil
	}
	if _, ok := m.files[path]; ok {
		return fakeFileInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (m *memFS) UserHomeDir() (string, error) { return m.homeDir, nil }

func (m *memFS) Glob(pattern string) ([]string, error) {
	// Simple glob: match files whose paths start with the directory
	// This is a simplification for testing — real FS uses filepath.Glob
	var matches []string
	// Strip the trailing *
	dir := pattern[:len(pattern)-1]
	for path := range m.files {
		if len(path) > len(dir) && path[:len(dir)] == dir {
			// Only match direct children (no deeper slashes)
			rest := path[len(dir):]
			if rest != "" && rest[0] != '/' {
				matches = append(matches, path)
			}
		}
	}
	return matches, nil
}

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string      { return f.name }
func (f fakeFileInfo) Size() int64       { return 0 }
func (f fakeFileInfo) Mode() os.FileMode { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool       { return f.isDir }
func (f fakeFileInfo) Sys() any          { return nil }

// Ensure memFS satisfies config.FS
var _ config.FS = (*memFS)(nil)

const ayuDarkTheme = `palette = 0=#11151c
palette = 1=#ea6c73
palette = 2=#7fd962
palette = 3=#f9af4f
palette = 4=#53bdfa
palette = 5=#cda1fa
palette = 6=#90e1c6
palette = 7=#c7c7c7
palette = 8=#686868
palette = 9=#f07178
palette = 10=#aad94c
palette = 11=#ffb454
palette = 12=#59c2ff
palette = 13=#d2a6ff
palette = 14=#95e6cb
palette = 15=#ffffff
background = #0b0e14
foreground = #bfbdb6
cursor-color = #e6b450
cursor-text = #0b0e14
selection-background = #409fff
selection-foreground = #0b0e14
`

func TestParseBytes_AyuDark(t *testing.T) {
	th, err := ParseBytes([]byte(ayuDarkTheme))
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	// Check background
	assertColor(t, "Background", th.Background, Color{0x0b, 0x0e, 0x14})

	// Check foreground
	assertColor(t, "Foreground", th.Foreground, Color{0xbf, 0xbd, 0xb6})

	// Check cursor
	assertColor(t, "Cursor", th.Cursor, Color{0xe6, 0xb4, 0x50})

	// Check selection
	assertColor(t, "Selection", th.Selection, Color{0x40, 0x9f, 0xff})

	// Check palette entries
	assertColor(t, "Palette[0]", th.Palette[0], Color{0x11, 0x15, 0x1c})
	assertColor(t, "Palette[1]", th.Palette[1], Color{0xea, 0x6c, 0x73})
	assertColor(t, "Palette[2]", th.Palette[2], Color{0x7f, 0xd9, 0x62})
	assertColor(t, "Palette[3]", th.Palette[3], Color{0xf9, 0xaf, 0x4f})
	assertColor(t, "Palette[4]", th.Palette[4], Color{0x53, 0xbd, 0xfa})
	assertColor(t, "Palette[5]", th.Palette[5], Color{0xcd, 0xa1, 0xfa})
	assertColor(t, "Palette[6]", th.Palette[6], Color{0x90, 0xe1, 0xc6})
	assertColor(t, "Palette[7]", th.Palette[7], Color{0xc7, 0xc7, 0xc7})
	assertColor(t, "Palette[8]", th.Palette[8], Color{0x68, 0x68, 0x68})
	assertColor(t, "Palette[9]", th.Palette[9], Color{0xf0, 0x71, 0x78})
	assertColor(t, "Palette[10]", th.Palette[10], Color{0xaa, 0xd9, 0x4c})
	assertColor(t, "Palette[11]", th.Palette[11], Color{0xff, 0xb4, 0x54})
	assertColor(t, "Palette[12]", th.Palette[12], Color{0x59, 0xc2, 0xff})
	assertColor(t, "Palette[13]", th.Palette[13], Color{0xd2, 0xa6, 0xff})
	assertColor(t, "Palette[14]", th.Palette[14], Color{0x95, 0xe6, 0xcb})
	assertColor(t, "Palette[15]", th.Palette[15], Color{0xff, 0xff, 0xff})
}

func TestParseBytes_WithComments(t *testing.T) {
	data := `# This is a comment
background = #1a1b26
foreground = #c0caf5
# Another comment
cursor-color = #c0caf5
palette = 0=#15161e
palette = 1=#f7768e
`
	th, err := ParseBytes([]byte(data))
	if err != nil {
		t.Fatalf("ParseBytes with comments failed: %v", err)
	}

	assertColor(t, "Background", th.Background, Color{0x1a, 0x1b, 0x26})
	assertColor(t, "Foreground", th.Foreground, Color{0xc0, 0xca, 0xf5})
	assertColor(t, "Palette[0]", th.Palette[0], Color{0x15, 0x16, 0x1e})
	assertColor(t, "Palette[1]", th.Palette[1], Color{0xf7, 0x76, 0x8e})
}

func TestParseBytes_Malformed(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "invalid hex in background",
			data: "background = #zzzzzz",
		},
		{
			name: "invalid palette index",
			data: "palette = 99=#ffffff",
		},
		{
			name: "invalid palette format",
			data: "palette = notanumber",
		},
		{
			name: "short hex color",
			data: "background = #fff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBytes([]byte(tt.data))
			if err == nil {
				t.Error("expected error for malformed input")
			}
		})
	}
}

func TestParseBytes_EmptyAndBlankLines(t *testing.T) {
	data := `

background = #000000

foreground = #ffffff

`
	th, err := ParseBytes([]byte(data))
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	assertColor(t, "Background", th.Background, Color{0, 0, 0})
	assertColor(t, "Foreground", th.Foreground, Color{255, 255, 255})
}

func TestIsDark(t *testing.T) {
	tests := []struct {
		name   string
		bg     Color
		isDark bool
	}{
		{"dark bg", Color{0x0b, 0x0e, 0x14}, true},
		{"light bg", Color{0xf5, 0xf5, 0xf5}, false},
		{"mid dark", Color{0x40, 0x40, 0x40}, true},
		{"mid light", Color{0x90, 0x90, 0x90}, false},
		{"black", Color{0, 0, 0}, true},
		{"white", Color{255, 255, 255}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := Theme{Background: tt.bg}
			if got := th.IsDark(); got != tt.isDark {
				t.Errorf("IsDark() = %v, want %v (bg=%v, sum=%d)",
					got, tt.isDark, tt.bg,
					int(tt.bg.R)+int(tt.bg.G)+int(tt.bg.B))
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input string
		want  Color
	}{
		{"#000000", Color{0, 0, 0}},
		{"#ffffff", Color{255, 255, 255}},
		{"#0b0e14", Color{0x0b, 0x0e, 0x14}},
		{"0b0e14", Color{0x0b, 0x0e, 0x14}},   // without #
		{" #0b0e14 ", Color{0x0b, 0x0e, 0x14}}, // with whitespace
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseHexColor(tt.input)
			if err != nil {
				t.Fatalf("ParseHexColor(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseHexColor(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorHex(t *testing.T) {
	c := Color{0x0b, 0x0e, 0x14}
	if got := c.Hex(); got != "#0b0e14" {
		t.Errorf("Color.Hex() = %q, want %q", got, "#0b0e14")
	}
}

func TestParseFile(t *testing.T) {
	fs := newMemFS("/home/test")
	fs.files["/themes/test-theme"] = []byte(ayuDarkTheme)

	th, err := ParseFile(fs, "/themes/test-theme")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	assertColor(t, "Background", th.Background, Color{0x0b, 0x0e, 0x14})
}

func TestParseFile_NotFound(t *testing.T) {
	fs := newMemFS("/home/test")

	_, err := ParseFile(fs, "/themes/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseBundledAyuDark(t *testing.T) {
	// Test that the actual embedded ayu-dark theme can be parsed
	data, err := bundledFS.ReadFile("bundled/ayu-dark")
	if err != nil {
		t.Fatalf("failed to read bundled ayu-dark: %v", err)
	}

	th, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("failed to parse bundled ayu-dark: %v", err)
	}

	// Verify it's a dark theme
	if !th.IsDark() {
		t.Error("ayu-dark should be a dark theme")
	}

	// Verify background is correct
	assertColor(t, "Background", th.Background, Color{0x0b, 0x0e, 0x14})
}

func assertColor(t *testing.T, name string, got, want Color) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v (#%02x%02x%02x), want %v (#%02x%02x%02x)",
			name, got, got.R, got.G, got.B, want, want.R, want.G, want.B)
	}
}
