package session

import (
	"testing"
)

func TestParseTemplateValid(t *testing.T) {
	data := []byte(`
name = "dev"
description = "Full dev environment"

[[windows]]
name = "editor"
command = "nvim ."

[[windows]]
name = "server"

[[windows]]
name = "git"
command = "git status"

[options]
focus = "editor"
`)

	tmpl, err := ParseTemplate(data)
	if err != nil {
		t.Fatalf("ParseTemplate() error: %v", err)
	}

	if tmpl.Name != "dev" {
		t.Errorf("expected name 'dev', got %q", tmpl.Name)
	}
	if tmpl.Description != "Full dev environment" {
		t.Errorf("expected description 'Full dev environment', got %q", tmpl.Description)
	}
	if len(tmpl.Windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(tmpl.Windows))
	}
	if tmpl.Windows[0].Name != "editor" {
		t.Errorf("expected first window name 'editor', got %q", tmpl.Windows[0].Name)
	}
	if tmpl.Windows[0].Command != "nvim ." {
		t.Errorf("expected first window command 'nvim .', got %q", tmpl.Windows[0].Command)
	}
	if tmpl.Windows[1].Name != "server" {
		t.Errorf("expected second window name 'server', got %q", tmpl.Windows[1].Name)
	}
	if tmpl.Windows[1].Command != "" {
		t.Errorf("expected second window command to be empty, got %q", tmpl.Windows[1].Command)
	}
	if tmpl.Options.Focus != "editor" {
		t.Errorf("expected focus 'editor', got %q", tmpl.Options.Focus)
	}
}

func TestParseTemplateMissingName(t *testing.T) {
	data := []byte(`
description = "No name template"

[[windows]]
name = "shell"
`)

	_, err := ParseTemplate(data)
	if err == nil {
		t.Fatal("expected error when template name is missing")
	}
}

func TestParseTemplateMissingFieldsDefault(t *testing.T) {
	data := []byte(`
name = "minimal"
`)

	tmpl, err := ParseTemplate(data)
	if err != nil {
		t.Fatalf("ParseTemplate() error: %v", err)
	}

	if tmpl.Description != "" {
		t.Errorf("expected empty description, got %q", tmpl.Description)
	}
	if len(tmpl.Windows) != 0 {
		t.Errorf("expected 0 windows, got %d", len(tmpl.Windows))
	}
	if tmpl.Options.Focus != "" {
		t.Errorf("expected empty focus, got %q", tmpl.Options.Focus)
	}
}

func TestParseTemplateInvalidTOML(t *testing.T) {
	data := []byte(`not valid toml {{{`)

	_, err := ParseTemplate(data)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestEmbeddedTemplatesAccessible(t *testing.T) {
	templates := EmbeddedTemplates()

	if len(templates) < 4 {
		t.Fatalf("expected at least 4 embedded templates, got %d", len(templates))
	}

	// Verify that all expected templates are present.
	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}

	expected := []string{"dev", "claude", "webdev", "monitor"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing embedded template: %q", name)
		}
	}

	// Verify each template has at least one window.
	for _, tmpl := range templates {
		if len(tmpl.Windows) == 0 {
			t.Errorf("template %q has no windows", tmpl.Name)
		}
		if tmpl.Description == "" {
			t.Errorf("template %q has no description", tmpl.Name)
		}
	}
}

func TestEmbeddedDevTemplate(t *testing.T) {
	templates := EmbeddedTemplates()

	var dev *Template
	for _, tmpl := range templates {
		if tmpl.Name == "dev" {
			tmpl := tmpl // capture
			dev = &tmpl
			break
		}
	}

	if dev == nil {
		t.Fatal("dev template not found")
	}

	if len(dev.Windows) != 3 {
		t.Fatalf("expected 3 windows in dev template, got %d", len(dev.Windows))
	}

	if dev.Windows[0].Name != "editor" {
		t.Errorf("expected first window 'editor', got %q", dev.Windows[0].Name)
	}
	if dev.Windows[0].Command != "nvim ." {
		t.Errorf("expected editor command 'nvim .', got %q", dev.Windows[0].Command)
	}
	if dev.Options.Focus != "editor" {
		t.Errorf("expected focus 'editor', got %q", dev.Options.Focus)
	}
}
