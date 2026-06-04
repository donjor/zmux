package bar

// Width-budget regression net for the window-status tabs (plan 024,
// bar-layout-density). The bar preview fakes tabs, and tmux's real
// window-list truncation is a runtime behavior driven by how much row the
// status sides consume. This models that deterministically from the actual
// RenderLeft/RenderRight + the generated side caps, so tab starvation is
// measured in CI rather than eyeballed in screenshots.
//
// tabBudget = clientWidth - min(leftW, leftCap) - min(rightW, rightCap) - markerReserve
//
// where leftW/rightW are glyph-aware cell widths. See
// .dump/plans/024_*/evidence/baseline.md for the captured baseline + findings
// (notably: the side caps do not bind at realistic content widths, so the
// dominant lever is the P2 bottom-left de-dup, not the caps).

import (
	"strconv"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

const tabMarkerReserve = 2 // pessimistic </> overflow-marker cells

// cellWidth is the glyph-aware display width of a tmux format string (nerd /
// powerline glyphs counted at their real cell width), via the preview's
// tmux→ANSI path then lipgloss cell measurement.
func cellWidth(tmuxFmt string) int { return lipgloss.Width(tmuxToANSI(tmuxFmt)) }

// generatedSideCaps returns the status-left-length / status-right-length the
// bar actually generates for a preset under the given layout (dynamic path).
func generatedSideCaps(preset Preset, pal *theme.Palette, layout string) (left, right int) {
	left, right = 100, 80
	for _, o := range GenerateWithLayout(preset, pal, BarLayoutConfig{Layout: layout}, "zmux") {
		switch o.Key {
		case "status-left-length":
			if v, err := strconv.Atoi(o.Value); err == nil {
				left = v
			}
		case "status-right-length":
			if v, err := strconv.Atoi(o.Value); err == nil {
				right = v
			}
		}
	}
	return left, right
}

// tabBudget models the window-list cell budget tmux leaves between the sides.
func tabBudget(clientWidth, leftW, rightW, leftCap, rightCap int) int {
	b := clientWidth - min(leftW, leftCap) - min(rightW, rightCap) - tabMarkerReserve
	if b < 0 {
		return 0
	}
	return b
}

func budgetPalette(t *testing.T) theme.Palette {
	t.Helper()
	th, err := theme.NewResolver(config.RealFS{}, "", "").Resolve(config.DefaultConfig().Theme)
	if err != nil {
		t.Fatalf("resolve default theme: %v", err)
	}
	return th.SemanticPalette()
}

// budgetCtxs returns representative contexts: idle, prefix-held (widens right),
// and realistic long content (long workspace+session+deep dir).
//
// All contexts run with TopRowActive=true: the default layout is two-line, so
// the bottom-left is the de-duped aux row (plan 024 P2), not the full identity
// pill chain. Modeling that is the whole point — it's what tmux actually leaves
// for the window list at runtime.
func budgetCtxs() map[string]func() BarContext {
	seg := config.BarSegments{
		Workspace: true, Git: true, Lang: true, Clock: true,
		Directory: true, Process: true, Group: true,
	}
	base := func() BarContext {
		c := makePreviewCtx(seg)
		c.TopRowActive = true
		return c
	}
	wide := func() BarContext {
		c := base()
		c.Workspace = "zmux.qol-bar"
		c.Session = "claude-review"
		c.PaneDir = "~/donjor/zmux.qol-bar/internal"
		return c
	}
	return map[string]func() BarContext{
		"noprefix": base,
		"prefix":   func() BarContext { c := base(); c.Prefix = true; return c },
		"wide":     wide,
		"wide+pfx": func() BarContext { c := wide(); c.Prefix = true; return c },
	}
}

// TestBarWidthBudget asserts the window-tab budget stays healthy across
// realistic content. It logs the full matrix so the baseline (and the P2
// de-dup improvement) is visible in `go test -v`.
//
// Floors locked after P2 (plan 024). The de-dup dropped leftW from ≤68 (full
// identity chain) to 0–32 (aux only), so the binding constraint is now the
// right side — and only under a held prefix, whose hint text widens it to ~57.
// We therefore relax the narrow-terminal (w=80) floor for prefix-held contexts:
// holding the prefix on an 80-col terminal legitimately yields tabs to the
// transient hints. Caps (status-left/right-length) still never bind at these
// widths, so they are deliberately left untouched — see evidence/baseline.md.
func TestBarWidthBudget(t *testing.T) {
	pal := budgetPalette(t)
	ctxs := budgetCtxs()
	// Deterministic ctx order for stable logs.
	order := []string{"noprefix", "prefix", "wide", "wide+pfx"}
	noPrefix := map[string]bool{"noprefix": true, "wide": true}

	const (
		wantMinAt200     = 100 // wide terminal: generous tab room, always
		wantMinAt120     = 30  // mid terminal: tabs still clearly visible
		wantMinAt80NoPfx = 20  // narrow terminal, prefix released
	)
	for _, p := range AllPresets() {
		lc, rc := generatedSideCaps(p, &pal, "two-line")
		for _, name := range order {
			ctx := ctxs[name]()
			lW := cellWidth(RenderLeft(&pal, ctx, p))
			rW := cellWidth(RenderRight(&pal, ctx, p))
			b80 := tabBudget(80, lW, rW, lc, rc)
			b120 := tabBudget(120, lW, rW, lc, rc)
			b200 := tabBudget(200, lW, rW, lc, rc)
			t.Logf("%-11s %-9s leftW=%-3d rightW=%-3d  budget: w80=%-3d w120=%-3d w200=%-3d",
				p.String(), name, lW, rW, b80, b120, b200)

			if b200 < wantMinAt200 {
				t.Errorf("%s/%s: w=200 tab budget %d < %d — status sides eat a wide bar",
					p.String(), name, b200, wantMinAt200)
			}
			if b120 < wantMinAt120 {
				t.Errorf("%s/%s: w=120 tab budget %d < %d — status sides crowd a mid bar",
					p.String(), name, b120, wantMinAt120)
			}
			if noPrefix[name] && b80 < wantMinAt80NoPfx {
				t.Errorf("%s/%s: w=80 tab budget %d < %d — status sides starve a narrow bar",
					p.String(), name, b80, wantMinAt80NoPfx)
			}
		}
	}
}
