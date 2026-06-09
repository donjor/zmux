package bar

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func tabsRowFixture() []tmux.LogicalPaneRow {
	return []tmux.LogicalPaneRow{
		// raw window, active
		{
			PaneID: "%1", Session: "dev", WindowID: "@1", WindowIndex: 0, WindowName: "vim",
			WindowActive: true, WindowPanes: 1, PaneActive: true,
		},
		// full tab, labeled, running
		{
			PaneID: "%2", Session: "dev", WindowID: "@2", WindowIndex: 1, WindowName: "node",
			WindowPanes: 2, TabID: "ztab_bud", Label: "buddy", State: "running",
		},
		// rider pane-of inside @2, done
		{
			PaneID: "%3", Session: "dev", WindowID: "@2", WindowIndex: 1, WindowName: "node",
			WindowPanes: 2, TabID: "ztab_tst", Label: "tests", State: "done", Anchor: "ztab_bud",
		},
		// docked tab, origin dev, attention
		{
			PaneID: "%4", Session: tabs.DockSession, WindowID: "@9", WindowIndex: 0, WindowName: "logs",
			WindowPanes: 1, TabID: "ztab_log", Label: "logs", State: "attention", Hidden: "dev",
		},
		// second docked tab, origin dev, stateless — pins the per-entry
		// dim reset (the first tab's glyph fg must not bleed into it)
		{
			PaneID: "%7", Session: tabs.DockSession, WindowID: "@10", WindowIndex: 2, WindowName: "brk",
			WindowPanes: 1, TabID: "ztab_brk", Label: "brk", Hidden: "dev",
		},
		// another session's window + docked tab from elsewhere: excluded
		{
			PaneID: "%5", Session: "ops", WindowID: "@5", WindowIndex: 0, WindowName: "htop",
			WindowPanes: 1,
		},
		{
			PaneID: "%6", Session: tabs.DockSession, WindowID: "@8", WindowIndex: 1, WindowName: "etl",
			WindowPanes: 1, TabID: "ztab_etl", Label: "etl", Hidden: "ops",
		},
	}
}

// stripStyles drops #[...] directives so assertions see what lands on screen.
var styleDirective = regexp.MustCompile(`#\[[^\]]*\]`)

func stripStyles(s string) string {
	return styleDirective.ReplaceAllString(s, "")
}

func TestRenderTabsRowComposition(t *testing.T) {
	now := time.Unix(0, 0)
	out := RenderTabsRow(testPalette(), Default, "dev", "dev", tabsRowFixture(), false, now)
	flat := stripStyles(out)

	for _, want := range []string{
		" 0 vim",            // raw window, by index
		" 1 buddy",          // full tab shows its label, not the auto name
		SpinnerFrame(now),   // running glyph animates from the wall clock
		"+tests",            // pane-of rider in the host cell
		stateGlyphs["done"], // rider's own pane-canonical glyph
		"(logs~",            // docked tab grouped in the hidden section
		stateGlyphs["attention"],
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("row missing %q:\n%s", want, flat)
		}
	}
	for _, reject := range []string{"htop", "etl", "node"} {
		if strings.Contains(flat, reject) {
			t.Errorf("row must not contain %q:\n%s", reject, flat)
		}
	}
}

// TestRenderTabsRowHiddenGroupDimReset pins the docked-group fg reset: after
// a docked tab's state glyph, the next docked name must re-dim instead of
// inheriting the glyph color.
func TestRenderTabsRowHiddenGroupDimReset(t *testing.T) {
	out := RenderTabsRow(testPalette(), Default, "dev", "dev", tabsRowFixture(), false, time.Unix(0, 0))
	dim := testPalette().Dim.Hex()
	if !strings.Contains(out, "#[fg="+dim+",nobold]brk~") {
		t.Errorf("second docked tab must re-dim after prior glyph:\n%s", out)
	}
}

func TestRenderTabsRowActiveWindowAccented(t *testing.T) {
	out := RenderTabsRow(testPalette(), Default, "dev", "dev", tabsRowFixture(), false, time.Unix(0, 0))
	accent := testPalette().Accent.Hex()
	if !strings.Contains(out, "#[fg="+accent+",bold] 0 vim") {
		t.Errorf("active window must render accented+bold:\n%s", out)
	}
}

// TestRenderTabsRowGlyphSpacing pins the glyph gap: state glyphs sit one
// space after the name they annotate, never jammed against it (QA finding
// 2026-06-06: "work✓").
func TestRenderTabsRowGlyphSpacing(t *testing.T) {
	now := time.Unix(0, 0)
	flat := stripStyles(RenderTabsRow(testPalette(), Default, "dev", "dev", tabsRowFixture(), false, now))

	for _, want := range []string{
		"buddy " + SpinnerFrame(now),        // full-tab glyph spaced
		"+tests " + stateGlyphs["done"],     // rider glyph spaced
		"logs~ " + stateGlyphs["attention"], // docked glyph spaced
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("row missing spaced glyph %q:\n%s", want, flat)
		}
	}
}

// TestRenderTabsRowRpowerlineChrome pins the preset chrome port: the dynamic
// row replaced tmux's native window list, so the rpowerline pill spec
// (rounded caps, two-tone index▸name segments) must render Go-side — losing
// it was the QA finding this guards against.
func TestRenderTabsRowRpowerlineChrome(t *testing.T) {
	p := testPalette()
	out := RenderTabsRow(p, Rpowerline, "dev", "dev", tabsRowFixture(), false, time.Unix(0, 0))

	for _, want := range []string{
		tabCapLeft, tabCapRight, tabArrow, // pill caps + section arrow
		"#[bg=" + p.Accent.Hex() + ",fg=" + p.BG.Hex() + ",bold]0" + tabThinSp, // active index segment
		"#[bg=" + p.Surface.Hex() + ",fg=" + p.FG.Hex() + ",bold] vim ",        // active name segment
		"#[bg=" + p.Surface.Hex() + ",fg=" + p.Muted.Hex() + "] buddy",         // inactive name segment
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rpowerline row missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "│") {
		t.Errorf("rpowerline must not use the flat │ separator:\n%s", out)
	}
}

// TestRenderTabsRowAllPresetsClean sweeps every preset: each renders a
// non-empty row, never leaks unexpanded tmux conditionals or #I/#W tokens
// (the chrome is a Go port, not a copied format string), and keeps the
// window names visible.
func TestRenderTabsRowAllPresetsClean(t *testing.T) {
	now := time.Unix(0, 0)
	for _, preset := range AllPresets() {
		out := RenderTabsRow(testPalette(), preset, "dev", "dev", tabsRowFixture(), false, now)
		if out == "" {
			t.Errorf("%s: empty row", preset)
			continue
		}
		for _, reject := range []string{"#{", "#I", "#W"} {
			if strings.Contains(out, reject) {
				t.Errorf("%s: leaked %q:\n%s", preset, reject, out)
			}
		}
		flat := stripStyles(out)
		for _, want := range []string{"vim", "buddy", "+tests"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%s: missing %q:\n%s", preset, want, flat)
			}
		}
	}
}

func TestRenderTabsRowEmptySessionRendersNothing(t *testing.T) {
	if out := RenderTabsRow(testPalette(), Default, "", "", tabsRowFixture(), false, time.Unix(0, 0)); out != "" {
		t.Errorf("empty session must render empty, got %q", out)
	}
}

func TestTabsRowStatusFormatEmbedsRenderCall(t *testing.T) {
	f := TabsRowStatusFormat("/usr/bin/zmux")
	if !strings.Contains(f, "#(/usr/bin/zmux bar-render tabs --session '#S'") {
		t.Errorf("format must call bar-render tabs: %s", f)
	}
	if !strings.Contains(f, "--prefix '#{client_prefix}'") {
		t.Errorf("format must pass client_prefix (conditionals can't expand inside #() output): %s", f)
	}
	if strings.Contains(f, "#{W:") {
		t.Errorf("dynamic row must replace the native window list: %s", f)
	}
	if !strings.Contains(f, "status-left") || !strings.Contains(f, "status-right") {
		t.Errorf("status-left/right sections must survive: %s", f)
	}
}
