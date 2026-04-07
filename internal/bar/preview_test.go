package bar

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
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

// Disabling the clock segment must hide both the time pill AND the date pill
// in every preset that renders them. Regression test for the bug where pills
// rendered as empty chrome (icon only) when ctx.Time/Date were cleared.
func TestClockSegmentTogglesHideTimeAndDate(t *testing.T) {
	p := *testPalette()
	segs := config.BarSegments{
		Workspace: true, Git: true, Lang: true, Clock: false,
		Directory: true, Process: true, Group: true,
	}
	for _, preset := range AllPresets() {
		preview := stripANSI(RenderPreviewWithSegments(preset, &p, segs))
		// 14:30 is the placeholder time; Apr 07 is the placeholder date.
		if strings.Contains(preview, "14:30") {
			t.Errorf("preset %s: clock disabled but '14:30' visible: %q", preset, preview)
		}
		if strings.Contains(preview, "Apr 07") {
			t.Errorf("preset %s: clock disabled but 'Apr 07' visible: %q", preset, preview)
		}
		// The clock icon (󱑍) should also disappear when the time pill is hidden.
		if strings.Contains(preview, "󱑍") {
			t.Errorf("preset %s: clock disabled but clock icon 󱑍 still rendered: %q", preset, preview)
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
