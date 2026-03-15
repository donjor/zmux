package theme

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

// ThemeSource indicates where a theme was found.
type ThemeSource string

const (
	SourceUser    ThemeSource = "user"
	SourceBundled ThemeSource = "bundled"
	SourceIterm2  ThemeSource = "iterm2"
)

// ThemeInfo holds metadata about a discovered theme.
type ThemeInfo struct {
	Name   string
	Source ThemeSource
	IsDark bool
}

// Resolver discovers and loads themes from multiple sources.
type Resolver struct {
	fs        config.FS
	userDir   string // e.g., ~/.zmux/themes
	iterm2Dir string // e.g., ~/.zmux/themes/iterm2
}

// NewResolver creates a Resolver that searches the given directories.
func NewResolver(fs config.FS, userDir, iterm2Dir string) *Resolver {
	return &Resolver{
		fs:        fs,
		userDir:   userDir,
		iterm2Dir: iterm2Dir,
	}
}

// Resolve looks up a theme by name, searching in priority order:
// user > bundled > iterm2.
func (r *Resolver) Resolve(name string) (Theme, error) {
	// 1. User themes
	if r.userDir != "" {
		path := filepath.Join(r.userDir, name)
		if _, err := r.fs.Stat(path); err == nil {
			t, err := ParseFile(r.fs, path)
			if err != nil {
				return Theme{}, fmt.Errorf("user theme %q: %w", name, err)
			}
			t.Name = name
			return t, nil
		}
	}

	// 2. Bundled themes (embedded)
	data, err := bundledFS.ReadFile("bundled/" + name)
	if err == nil {
		t, err := ParseBytes(data)
		if err != nil {
			return Theme{}, fmt.Errorf("bundled theme %q: %w", name, err)
		}
		t.Name = name
		return t, nil
	}

	// 3. iterm2 themes (downloaded)
	if r.iterm2Dir != "" {
		path := filepath.Join(r.iterm2Dir, name)
		if _, err := r.fs.Stat(path); err == nil {
			t, err := ParseFile(r.fs, path)
			if err != nil {
				return Theme{}, fmt.Errorf("iterm2 theme %q: %w", name, err)
			}
			t.Name = name
			return t, nil
		}
	}

	return Theme{}, fmt.Errorf("theme %q not found", name)
}

// List returns all available themes from all sources, deduplicated.
// User themes take priority over bundled, which take priority over iterm2.
func (r *Resolver) List() []ThemeInfo {
	seen := make(map[string]bool)
	var themes []ThemeInfo

	// 1. User themes
	if r.userDir != "" {
		userThemes := r.listDir(r.userDir, SourceUser)
		for _, ti := range userThemes {
			if !seen[ti.Name] {
				seen[ti.Name] = true
				themes = append(themes, ti)
			}
		}
	}

	// 2. Bundled themes
	bundled := r.listBundled()
	for _, ti := range bundled {
		if !seen[ti.Name] {
			seen[ti.Name] = true
			themes = append(themes, ti)
		}
	}

	// 3. iterm2 themes
	if r.iterm2Dir != "" {
		iterm2Themes := r.listDir(r.iterm2Dir, SourceIterm2)
		for _, ti := range iterm2Themes {
			if !seen[ti.Name] {
				seen[ti.Name] = true
				themes = append(themes, ti)
			}
		}
	}

	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})

	return themes
}

// listDir lists theme files in a filesystem directory.
func (r *Resolver) listDir(dir string, source ThemeSource) []ThemeInfo {
	pattern := filepath.Join(dir, "*")
	matches, err := r.fs.Glob(pattern)
	if err != nil {
		return nil
	}

	var themes []ThemeInfo
	for _, path := range matches {
		name := filepath.Base(path)
		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := r.fs.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		data, err := r.fs.ReadFile(path)
		if err != nil {
			continue
		}

		t, err := ParseBytes(data)
		if err != nil {
			continue
		}

		themes = append(themes, ThemeInfo{
			Name:   name,
			Source: source,
			IsDark: t.IsDark(),
		})
	}

	return themes
}

// listBundled lists themes from the embedded bundled filesystem.
func (r *Resolver) listBundled() []ThemeInfo {
	entries, err := fs.ReadDir(bundledFS, "bundled")
	if err != nil {
		return nil
	}

	var themes []ThemeInfo
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		data, err := bundledFS.ReadFile("bundled/" + entry.Name())
		if err != nil {
			continue
		}

		t, err := ParseBytes(data)
		if err != nil {
			continue
		}

		themes = append(themes, ThemeInfo{
			Name:   entry.Name(),
			Source: SourceBundled,
			IsDark: t.IsDark(),
		})
	}

	return themes
}
