package theme

import (
	"testing"
)

func TestResolver_ResolveBundled(t *testing.T) {
	fs := newMemFS("/home/test")
	r := NewResolver(fs, "/home/test/.zmux/themes", "/home/test/.zmux/themes/iterm2")

	th, err := r.Resolve("ayu-dark")
	if err != nil {
		t.Fatalf("Resolve(ayu-dark) error: %v", err)
	}

	if th.Name != "ayu-dark" {
		t.Errorf("Name = %q, want %q", th.Name, "ayu-dark")
	}

	assertColor(t, "Background", th.Background, Color{0x0b, 0x0e, 0x14})
}

func TestResolver_ResolveMissing(t *testing.T) {
	fs := newMemFS("/home/test")
	r := NewResolver(fs, "/home/test/.zmux/themes", "/home/test/.zmux/themes/iterm2")

	_, err := r.Resolve("nonexistent-theme")
	if err == nil {
		t.Fatal("expected error for nonexistent theme")
	}
}

func TestResolver_ResolveUserPriority(t *testing.T) {
	fs := newMemFS("/home/test")

	// Create a user theme with same name as bundled but different colors
	userTheme := `background = #ffffff
foreground = #000000
cursor-color = #000000
selection-background = #cccccc
palette = 0=#111111
palette = 1=#222222
palette = 2=#333333
palette = 3=#444444
palette = 4=#555555
palette = 5=#666666
palette = 6=#777777
palette = 7=#888888
palette = 8=#999999
palette = 9=#aaaaaa
palette = 10=#bbbbbb
palette = 11=#cccccc
palette = 12=#dddddd
palette = 13=#eeeeee
palette = 14=#f0f0f0
palette = 15=#f5f5f5
`
	fs.files["/home/test/.zmux/themes/ayu-dark"] = []byte(userTheme)

	r := NewResolver(fs, "/home/test/.zmux/themes", "/home/test/.zmux/themes/iterm2")

	th, err := r.Resolve("ayu-dark")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	// Should get the user theme (white bg), not the bundled one (dark bg)
	assertColor(t, "Background", th.Background, Color{0xff, 0xff, 0xff})
}

func TestResolver_ResolveIterm2(t *testing.T) {
	fs := newMemFS("/home/test")

	iterm2Theme := `background = #282c34
foreground = #abb2bf
cursor-color = #528bff
selection-background = #3e4451
palette = 0=#282c34
palette = 1=#e06c75
palette = 2=#98c379
palette = 3=#e5c07b
palette = 4=#61afef
palette = 5=#c678dd
palette = 6=#56b6c2
palette = 7=#abb2bf
palette = 8=#545862
palette = 9=#e06c75
palette = 10=#98c379
palette = 11=#e5c07b
palette = 12=#61afef
palette = 13=#c678dd
palette = 14=#56b6c2
palette = 15=#c8ccd4
`
	fs.files["/home/test/.zmux/themes/iterm2/one-dark"] = []byte(iterm2Theme)

	r := NewResolver(fs, "/home/test/.zmux/themes", "/home/test/.zmux/themes/iterm2")

	th, err := r.Resolve("one-dark")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	assertColor(t, "Background", th.Background, Color{0x28, 0x2c, 0x34})
	if th.Name != "one-dark" {
		t.Errorf("Name = %q, want %q", th.Name, "one-dark")
	}
}

func TestResolver_ListBundled(t *testing.T) {
	fs := newMemFS("/home/test")
	r := NewResolver(fs, "", "") // no user or iterm2 dirs

	themes := r.List()
	if len(themes) == 0 {
		t.Fatal("List() returned no themes; expected bundled themes")
	}

	// Check that ayu-dark is in the list
	found := false
	for _, ti := range themes {
		if ti.Name == "ayu-dark" {
			found = true
			if ti.Source != SourceBundled {
				t.Errorf("ayu-dark source = %q, want %q", ti.Source, SourceBundled)
			}
			if !ti.IsDark {
				t.Error("ayu-dark should be dark")
			}
			break
		}
	}
	if !found {
		t.Error("ayu-dark not found in List()")
	}

	// All bundled themes should be present
	expectedBundled := []string{
		"atom-one-dark", "ayu-dark", "carbonfox", "catppuccin-mocha",
		"dracula", "gruvbox-dark", "kanagawa-dragon", "material-darker",
		"nord", "rose-pine", "tokyonight",
	}
	for _, name := range expectedBundled {
		found := false
		for _, ti := range themes {
			if ti.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected bundled theme %q not found in List()", name)
		}
	}
}

func TestResolver_ListSorted(t *testing.T) {
	fs := newMemFS("/home/test")
	r := NewResolver(fs, "", "")

	themes := r.List()
	for i := 1; i < len(themes); i++ {
		if themes[i].Name < themes[i-1].Name {
			t.Errorf("themes not sorted: %q comes after %q", themes[i].Name, themes[i-1].Name)
		}
	}
}

func TestResolver_ListDeduplicates(t *testing.T) {
	fs := newMemFS("/home/test")

	// User theme with same name as bundled
	fs.files["/home/test/.zmux/themes/ayu-dark"] = []byte(ayuDarkTheme)

	r := NewResolver(fs, "/home/test/.zmux/themes", "")
	themes := r.List()

	count := 0
	for _, ti := range themes {
		if ti.Name == "ayu-dark" {
			count++
			if ti.Source != SourceUser {
				t.Errorf("deduplicated ayu-dark should be from user, got %q", ti.Source)
			}
		}
	}
	if count != 1 {
		t.Errorf("ayu-dark appeared %d times, want 1", count)
	}
}
