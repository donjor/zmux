package bar

import (
	"strings"
	"testing"
)

func TestRenderPreviewContainsANSI(t *testing.T) {
	p := *testPalette()
	for _, preset := range AllPresets() {
		preview := RenderPreview(preset, &p)
		if !strings.Contains(preview, "\033[") {
			t.Errorf("preset %s: expected ANSI escape codes in preview", preset)
		}
	}
}

func TestRenderPreviewDistinctPerPreset(t *testing.T) {
	p := *testPalette()
	previews := make(map[string]Preset)
	for _, preset := range AllPresets() {
		preview := stripANSI(RenderPreview(preset, &p))
		if prev, exists := previews[preview]; exists {
			t.Errorf("preset %s and %s produce identical visible output", preset, prev)
		}
		previews[preview] = preset
	}
}

func TestRenderPreviewContainsSampleData(t *testing.T) {
	p := *testPalette()
	for _, preset := range AllPresets() {
		preview := stripANSI(RenderPreview(preset, &p))
		if !strings.Contains(preview, "main") {
			t.Errorf("preset %s: expected 'main' (session or branch) in preview", preset)
		}
	}
}

func TestTmuxToANSIConvertsColors(t *testing.T) {
	input := "#[fg=#ff0000,bold]hello#[bg=#00ff00] world"
	output := tmuxToANSI(input)
	if !strings.Contains(output, "\033[") {
		t.Error("expected ANSI codes in output")
	}
	if !strings.Contains(output, "hello") {
		t.Error("expected text preserved")
	}
}

func TestStripTmuxConditionals(t *testing.T) {
	input := "#{?client_prefix,ACTIVE,NORMAL}"
	normal := stripTmuxConditionals(input, false)
	if normal != "NORMAL" {
		t.Errorf("expected NORMAL, got %q", normal)
	}
	active := stripTmuxConditionals(input, true)
	if active != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %q", active)
	}
}
