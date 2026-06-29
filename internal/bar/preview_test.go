package bar

import (
	"regexp"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/keys"
)

var tmuxStyleRE = regexp.MustCompile(`#\[[^\]]*\]`)

// stripTmuxStyles removes tmux #[...] style directives so tests can assert
// against the visible payload.
func stripTmuxStyles(s string) string { return tmuxStyleRE.ReplaceAllString(s, "") }

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

func TestDefaultPrefixHintsIncludeRename(t *testing.T) {
	p := testPalette()
	hints := prefixHints(p, BarContext{})
	for _, want := range []string{"dash", "etach", "tab", "session", "rename", "close", "help"} {
		if !strings.Contains(hints, want) {
			t.Errorf("prefix hints should include %q, got %q", want, hints)
		}
	}
	// A single (or unknown-count) pane shows none of the pane-layout cluster.
	for _, gone := range []string{"orient", "move", "even"} {
		if strings.Contains(hints, gone) {
			t.Errorf("non-split prefix hints should not include %q, got %q", gone, hints)
		}
	}
}

// Phase 1d: a split window appends the pane-layout hint cluster so the keys
// are discoverable exactly when they apply.
func TestPrefixHintsSplitAddsLayoutCluster(t *testing.T) {
	hints := prefixHints(testPalette(), BarContext{WindowPanes: 2})
	for _, want := range []string{"orient", "move", "even"} {
		if !strings.Contains(hints, want) {
			t.Errorf("split prefix hints should include %q, got %q", want, hints)
		}
	}
}

// TestPrefixHintsMatchRegistry ensures the hint surface stays glued to the
// keys registry. Regression for the historical bug where the hint said
// ".rename" but "." was bound to label-tab — the real rename key is ",".
func TestPrefixHintsMatchRegistry(t *testing.T) {
	hints := stripTmuxStyles(prefixHints(testPalette(), BarContext{}))
	cases := []struct{ key, label string }{
		{keys.RenameSession.Key, "rename"},
		{keys.LabelTab.Key, "label"},
		{keys.NewTab.Key, "tab"},
		{keys.NewSession.Key, "session"},
		{keys.TabKill.Key, "close"},
		{keys.Help.Key, "help"},
	}
	for _, c := range cases {
		want := c.key + c.label
		if !strings.Contains(hints, want) {
			t.Errorf("prefix hints should contain %q (key+label), got %q", want, hints)
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
