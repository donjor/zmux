package session

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donjor/zmux/internal/config"
	toml "github.com/pelletier/go-toml/v2"
)

// TemplateWindow defines a window to create within a session template.
type TemplateWindow struct {
	Name    string `toml:"name"`
	Command string `toml:"command,omitempty"`
}

// TemplateOptions defines optional settings for a session template.
type TemplateOptions struct {
	Focus string `toml:"focus,omitempty"`
}

// Template defines a declarative session layout in TOML format.
type Template struct {
	Name        string           `toml:"name"`
	Description string           `toml:"description"`
	Windows     []TemplateWindow `toml:"windows"`
	Options     TemplateOptions  `toml:"options"`
}

// ParseTemplate parses a TOML template from raw bytes.
func ParseTemplate(data []byte) (Template, error) {
	var tmpl Template
	if err := toml.Unmarshal(data, &tmpl); err != nil {
		return Template{}, fmt.Errorf("parse template: %w", err)
	}

	if tmpl.Name == "" {
		return Template{}, fmt.Errorf("template missing required field: name")
	}

	return tmpl, nil
}

// LoadTemplates discovers and parses .toml template files from the given directories.
// It first loads embedded templates, then overlays any user-provided ones.
// If a user template has the same name as an embedded one, the user template wins.
func LoadTemplates(fs config.FS, dirs []string) ([]Template, error) {
	// Start with embedded templates.
	embedded := EmbeddedTemplates()
	byName := make(map[string]Template, len(embedded))
	for _, t := range embedded {
		byName[t.Name] = t
	}

	// Overlay user templates from directories.
	for _, dir := range dirs {
		// Expand ~ to home dir.
		if strings.HasPrefix(dir, "~/") {
			home, err := fs.UserHomeDir()
			if err != nil {
				continue
			}
			dir = filepath.Join(home, dir[2:])
		}

		pattern := filepath.Join(dir, "*.toml")
		matches, err := fs.Glob(pattern)
		if err != nil {
			continue // skip inaccessible dirs
		}

		for _, path := range matches {
			data, err := fs.ReadFile(path)
			if err != nil {
				continue
			}

			tmpl, err := ParseTemplate(data)
			if err != nil {
				continue
			}

			byName[tmpl.Name] = tmpl
		}
	}

	// Collect results.
	templates := make([]Template, 0, len(byName))
	for _, t := range byName {
		templates = append(templates, t)
	}

	return templates, nil
}
