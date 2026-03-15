package bar

import (
	"strings"
	"testing"
)

func TestRenderPreviewContainsANSI(t *testing.T) {
	p := testPalette()

	for _, preset := range AllPresets() {
		t.Run(preset.String(), func(t *testing.T) {
			preview := RenderPreview(preset, p)

			if preview == "" {
				t.Fatal("preview is empty")
			}

			// ANSI escape sequences start with ESC[ (\x1b[)
			if !strings.Contains(preview, "\x1b[") {
				t.Errorf("preview should contain ANSI escape sequences, got %q", preview)
			}
		})
	}
}

func TestRenderPreviewDistinctPerPreset(t *testing.T) {
	p := testPalette()
	presets := AllPresets()
	previews := make(map[string]Preset)

	for _, preset := range presets {
		preview := RenderPreview(preset, p)
		stripped := stripANSI(preview)
		if other, ok := previews[stripped]; ok {
			t.Errorf("preset %s and %s produce identical visible output", preset, other)
		}
		previews[stripped] = preset
	}
}

func TestRenderPreviewPowerlineArrows(t *testing.T) {
	p := testPalette()
	preview := RenderPreview(Powerline, p)
	stripped := stripANSI(preview)

	// Powerline preview should contain arrow characters
	if !strings.Contains(stripped, "\ue0b0") && !strings.Contains(stripped, "\ue0b2") {
		t.Error("powerline preview should contain powerline arrow characters")
	}
}

func TestRenderPreviewBlocksBrackets(t *testing.T) {
	p := testPalette()
	preview := RenderPreview(Blocks, p)
	stripped := stripANSI(preview)

	if !strings.Contains(stripped, "[") || !strings.Contains(stripped, "]") {
		t.Error("blocks preview should contain square brackets")
	}
	if !strings.Contains(stripped, "[main]") {
		t.Errorf("blocks preview should contain [main], got visible: %q", stripped)
	}
}

func TestRenderPreviewDefaultSession(t *testing.T) {
	p := testPalette()
	preview := RenderPreview(Default, p)
	stripped := stripANSI(preview)

	if !strings.Contains(stripped, "main") {
		t.Errorf("default preview should show session name 'main', got visible: %q", stripped)
	}
}
