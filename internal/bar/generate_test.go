package bar

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/theme"
)

// testPalette returns a deterministic palette for testing.
func testPalette() *theme.Palette {
	return &theme.Palette{
		BG:        theme.Color{R: 10, G: 14, B: 20},
		FG:        theme.Color{R: 203, G: 203, B: 203},
		Surface:   theme.Color{R: 30, G: 34, B: 40},
		Error:     theme.Color{R: 255, G: 85, B: 85},
		Success:   theme.Color{R: 80, G: 200, B: 120},
		Accent:    theme.Color{R: 255, G: 180, B: 84},
		Info:      theme.Color{R: 90, G: 160, B: 230},
		Special:   theme.Color{R: 200, G: 130, B: 200},
		Meta:      theme.Color{R: 100, G: 200, B: 200},
		Muted:     theme.Color{R: 180, G: 180, B: 180},
		Dim:       theme.Color{R: 80, G: 80, B: 80},
		Highlight: theme.Color{R: 255, G: 255, B: 255},
		BGDim:     theme.Color{R: 25, G: 29, B: 35},
		BGPrefix:  theme.Color{R: 35, G: 39, B: 45},
	}
}

// findOpt looks up a TmuxOption by key in a slice.
func findOpt(opts []TmuxOption, key string) (TmuxOption, bool) {
	for _, o := range opts {
		if o.Key == key {
			return o, true
		}
	}
	return TmuxOption{}, false
}

func TestGenerateSharedOptions(t *testing.T) {
	p := testPalette()

	for _, preset := range AllPresets() {
		t.Run(preset.String(), func(t *testing.T) {
			opts := Generate(preset, p)

			// All presets must include these shared keys.
			sharedKeys := []string{
				"pane-border-style",
				"pane-active-border-style",
				"message-style",
				"message-command-style",
				"mode-style",
				"clock-mode-colour",
				"window-active-style",
				"window-style",
			}
			for _, key := range sharedKeys {
				opt, ok := findOpt(opts, key)
				if !ok {
					t.Errorf("missing shared option %q", key)
					continue
				}
				if opt.Value == "" {
					t.Errorf("shared option %q has empty value", key)
				}
			}

			// Shared options reference palette colors.
			borderOpt, _ := findOpt(opts, "pane-border-style")
			if !strings.Contains(borderOpt.Value, p.Dim.Hex()) {
				t.Errorf("pane-border-style should reference Dim color %s, got %q",
					p.Dim.Hex(), borderOpt.Value)
			}

			activeBorderOpt, _ := findOpt(opts, "pane-active-border-style")
			if !strings.Contains(activeBorderOpt.Value, p.Accent.Hex()) {
				t.Errorf("pane-active-border-style should reference Accent color %s, got %q",
					p.Accent.Hex(), activeBorderOpt.Value)
			}
		})
	}
}

func TestGeneratePresetSpecificKeys(t *testing.T) {
	p := testPalette()

	// All presets must produce these per-preset keys.
	presetKeys := []string{
		"status-style",
		"status-left",
		"status-right",
		"window-status-format",
		"window-status-current-format",
		"window-status-separator",
		"status-left-length",
		"status-right-length",
	}

	for _, preset := range AllPresets() {
		t.Run(preset.String(), func(t *testing.T) {
			opts := Generate(preset, p)
			for _, key := range presetKeys {
				if _, ok := findOpt(opts, key); !ok {
					t.Errorf("preset %s missing option %q", preset, key)
				}
			}
		})
	}
}

func TestGenerateDefaultPrefixHints(t *testing.T) {
	p := testPalette()
	opts := Generate(Default, p)

	right, ok := findOpt(opts, "status-right")
	if !ok {
		t.Fatal("missing status-right")
	}

	// Default preset should include prefix hint content
	if !strings.Contains(right.Value, "client_prefix") {
		t.Error("default status-right should contain client_prefix conditional")
	}
	if !strings.Contains(right.Value, "rename") {
		t.Error("default status-right should contain prefix hint text like 'rename'")
	}
	// "switch" is split across tmux format: key "s" highlighted, then "witch" in dim
	if !strings.Contains(right.Value, "witch") {
		t.Error("default status-right should contain prefix hint for switch (s + witch)")
	}
}

func TestGenerateDefaultUsesAccentAndInfo(t *testing.T) {
	p := testPalette()
	opts := Generate(Default, p)

	left, ok := findOpt(opts, "status-left")
	if !ok {
		t.Fatal("missing status-left")
	}

	if !strings.Contains(left.Value, p.Accent.Hex()) {
		t.Errorf("default status-left should reference Accent %s", p.Accent.Hex())
	}
	if !strings.Contains(left.Value, p.Info.Hex()) {
		t.Errorf("default status-left should reference Info %s (prefix active color)", p.Info.Hex())
	}
}

func TestGeneratePowerlineArrows(t *testing.T) {
	p := testPalette()
	opts := Generate(Powerline, p)

	left, _ := findOpt(opts, "status-left")
	right, _ := findOpt(opts, "status-right")
	winCur, _ := findOpt(opts, "window-status-current-format")

	// Powerline uses  (U+E0B0) and  (U+E0B2) arrow separators.
	if !strings.Contains(left.Value, "\ue0b0") {
		t.Error("powerline status-left should contain right arrow \ue0b0")
	}
	if !strings.Contains(right.Value, "\ue0b2") {
		t.Error("powerline status-right should contain left arrow \ue0b2")
	}
	if !strings.Contains(winCur.Value, "\ue0b0") {
		t.Error("powerline window-status-current-format should contain arrow \ue0b0")
	}

	// Separator should be empty for powerline (arrows are inline)
	sep, _ := findOpt(opts, "window-status-separator")
	if sep.Value != "" {
		t.Errorf("powerline window-status-separator should be empty, got %q", sep.Value)
	}
}

func TestGenerateBlocksBrackets(t *testing.T) {
	p := testPalette()
	opts := Generate(Blocks, p)

	left, _ := findOpt(opts, "status-left")
	right, _ := findOpt(opts, "status-right")
	winFmt, _ := findOpt(opts, "window-status-format")
	winCur, _ := findOpt(opts, "window-status-current-format")

	// Blocks uses square brackets.
	if !strings.Contains(left.Value, "[#S]") {
		t.Errorf("blocks status-left should contain [#S], got %q", left.Value)
	}
	if !strings.Contains(right.Value, "[") && !strings.Contains(right.Value, "]") {
		t.Error("blocks status-right should contain brackets")
	}
	if !strings.Contains(winFmt.Value, "[#I:#W]") {
		t.Errorf("blocks window-status-format should contain [#I:#W], got %q", winFmt.Value)
	}
	if !strings.Contains(winCur.Value, "[#I:#W]") {
		t.Errorf("blocks window-status-current-format should contain [#I:#W], got %q", winCur.Value)
	}
}

func TestGenerateMinimalClean(t *testing.T) {
	p := testPalette()
	opts := Generate(Minimal, p)

	left, _ := findOpt(opts, "status-left")
	winFmt, _ := findOpt(opts, "window-status-format")

	// Minimal uses just session name, no decoration
	if !strings.Contains(left.Value, "#S") {
		t.Errorf("minimal status-left should contain #S, got %q", left.Value)
	}

	// Window format: just #W (name), no index
	if !strings.Contains(winFmt.Value, "#W") {
		t.Errorf("minimal window-status-format should contain #W, got %q", winFmt.Value)
	}
	if strings.Contains(winFmt.Value, "#I") {
		t.Errorf("minimal window-status-format should NOT contain #I (index), got %q", winFmt.Value)
	}
}

func TestGeneratePaletteColorReferences(t *testing.T) {
	p := testPalette()

	for _, preset := range AllPresets() {
		t.Run(preset.String(), func(t *testing.T) {
			opts := Generate(preset, p)

			// At minimum, the status-style should reference Surface and Muted.
			statusStyle, ok := findOpt(opts, "status-style")
			if !ok {
				t.Fatal("missing status-style")
			}
			if !strings.Contains(statusStyle.Value, p.Surface.Hex()) {
				t.Errorf("status-style should reference Surface color %s", p.Surface.Hex())
			}
			if !strings.Contains(statusStyle.Value, p.Muted.Hex()) {
				t.Errorf("status-style should reference Muted color %s", p.Muted.Hex())
			}
		})
	}
}
