package session

import "embed"

//go:embed templates/*
var embeddedFS embed.FS

// EmbeddedTemplates returns all templates bundled into the binary.
func EmbeddedTemplates() []Template {
	entries, err := embeddedFS.ReadDir("templates")
	if err != nil {
		return nil
	}

	var templates []Template
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := embeddedFS.ReadFile("templates/" + e.Name())
		if err != nil {
			continue
		}
		tmpl, err := ParseTemplate(data)
		if err != nil {
			continue
		}
		templates = append(templates, tmpl)
	}

	return templates
}
