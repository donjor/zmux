// Package draft holds the in-progress multi-session status bar
// renderers. Graduates to internal/bar/ in Phase 1+.
//
// Two axes:
//
//   - Layout: single / two-line / split — how many rows, where they sit.
//   - Variant: dots / tabs — what the session indicator looks like.
//
// Dots adapt to available space: compact (●○○) when inline on a
// single-line bar, enriched (● main  ○ feat-auth  ○ two) when they
// have a full row. Tabs use the preset's window-tab styling applied
// to sessions — same chrome as window tabs, different data.
//
// The preset drives all styling. Layouts use bar.RenderPreviewWithSegments
// for the main bar strip so switching layouts never changes the bar's
// visual identity.
package draft

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

// Session is the minimal shape for rendering.
type Session struct {
	Name    string
	Index   int // 1-based
	Current bool
}

// Layout names — structural: how many rows.
const (
	LayoutSingle  = "single"
	LayoutTwoLine = "two-line"
	LayoutSplit   = "split"
)

// TopBar variants — how the workspace/session top row renders
// (only applies to two-line and split layouts).
const (
	TopBarMinimal = "minimal" // workspace name + session names (plain text)
	TopBarTabs    = "tabs"    // preset-styled session tab pills
	TopBarDots    = "dots"    // workspace + enriched dots (● name  ○ name)
)

// Indicator names — what goes INSIDE the session name pill on the
// main bar line.
const (
	IndicatorNone    = "none"
	IndicatorNumbers = "numbers"
	IndicatorDots    = "dots"
)

// ── Top bar content renderers ────────────────────────────────────────
//
// These render the session content for the top row (two-line/split).
// The workspace pill is prepended by the layout renderer.

func renderTopBarContent(variant string, pal *theme.Palette, sessions []Session, currentIdx int, preset bar.Preset) string {
	switch variant {
	case TopBarTabs:
		return RenderSessionTabs(pal, sessions, currentIdx, preset)
	case TopBarDots:
		return renderDotsEnriched(pal, sessions, currentIdx)
	case TopBarMinimal:
		return renderMinimalSessionList(pal, sessions, currentIdx)
	default:
		return renderMinimalSessionList(pal, sessions, currentIdx)
	}
}

// RenderDotsEnrichedStr renders enriched dots with names for the top bar.
func RenderDotsEnrichedStr(pal *theme.Palette, sessions []Session, currentIdx int) string {
	return renderDotsEnriched(pal, sessions, currentIdx)
}

func renderDotsEnriched(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		dot := "○"
		nameStyle := pal.Dim
		if s.Index == currentIdx {
			dot = "●"
			nameStyle = pal.Accent
		}
		dotStr := fg(nameStyle, dot)
		nameStr := fg(nameStyle, s.Name)
		if s.Index == currentIdx {
			nameStr = fg(pal.Accent, bold(s.Name))
		}
		parts = append(parts, dotStr+" "+nameStr)
	}
	return strings.Join(parts, fg(pal.Dim, "   "))
}

// RenderMinimalSessionList renders plain session names for the top bar.
func RenderMinimalSessionList(pal *theme.Palette, sessions []Session, currentIdx int) string {
	return renderMinimalSessionList(pal, sessions, currentIdx)
}

func renderMinimalSessionList(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts, fg(pal.Accent, bold(s.Name)))
		} else {
			parts = append(parts, fg(pal.Dim, s.Name))
		}
	}
	return strings.Join(parts, fg(pal.Dim, "  "))
}

// ── Indicator helpers ────────────────────────────────────────────────
//
// The indicator goes INSIDE the session name pill on the main bar.
// It replaces the "2/3" numbers with compact dots or leaves as-is.

// applyIndicator sets the SessionIndicator field on a BarContext based
// on the indicator choice. This is called via the override callback.
func applyIndicator(ctx *bar.BarContext, indicator string, pal *theme.Palette, sessions []Session, currentIdx int) {
	switch indicator {
	case IndicatorDots:
		ctx.SessionIndicator = RenderDotsPlain(sessions, currentIdx)
	case IndicatorNumbers:
		// Leave SessionIndicator empty — SessionLabel() falls back to "N/M".
	case IndicatorNone:
		// Suppress both dots and numbers.
		ctx.WorkspaceCount = 1
	}
}

// RenderDotsPlain returns the dot string WITHOUT ANSI colors — it goes
// inside the session pill which already has its own fg/bg from the
// preset. The dots use unicode chars that render well in any color.
func RenderDotsPlain(sessions []Session, currentIdx int) string {
	if len(sessions) <= 1 {
		return ""
	}
	var b strings.Builder
	for _, s := range sessions {
		if s.Index == currentIdx {
			b.WriteRune('●')
		} else {
			b.WriteRune('○')
		}
	}
	return b.String()
}

// ── Tabs rendering ──────────────────────────────────────────────────
//
// Tabs renders sessions using the SAME visual language the preset uses
// for window tabs. This reuses the pill/separator/arrow chrome so
// switching presets changes session tab styling automatically.
//
// For the prototype this is a simplified approximation using ANSI
// directly (not tmux format strings). Phase 1+ will generate actual
// tmux format strings through the preset's windowFmt/windowCurrentFmt.

func RenderSessionTabs(pal *theme.Palette, sessions []Session, currentIdx int, preset bar.Preset) string {
	if len(sessions) <= 1 {
		return ""
	}

	switch preset {
	case bar.Powerline:
		return renderTabsPowerline(pal, sessions, currentIdx)
	case bar.Rpowerline:
		return renderTabsRpowerline(pal, sessions, currentIdx)
	case bar.Rounded:
		return renderTabsRounded(pal, sessions, currentIdx)
	case bar.Blocks:
		return renderTabsBlocks(pal, sessions, currentIdx)
	case bar.Hacker:
		return renderTabsHacker(pal, sessions, currentIdx)
	case bar.Zen:
		return renderTabsZen(pal, sessions, currentIdx)
	case bar.Minimal:
		return renderTabsMinimal(pal, sessions, currentIdx)
	default:
		return renderTabsDefault(pal, sessions, currentIdx)
	}
}

// Each preset-specific renderer mirrors the window tab styling from
// generate.go but outputs ANSI instead of tmux format strings.

func renderTabsDefault(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts, fgbg(pal.BG, pal.Accent, " "+bold(s.Name)+" "))
		} else {
			parts = append(parts, fgbg(pal.Dim, pal.Surface, " "+s.Name+" "))
		}
	}
	return strings.Join(parts, " ")
}

func renderTabsPowerline(pal *theme.Palette, sessions []Session, currentIdx int) string {
	// Mirrors the two-section powerline window format from generate.go:
	//   [▸index▸name▸]
	var parts []string
	for _, s := range sessions {
		idx := fmt.Sprintf("%d", s.Index)
		if s.Index == currentIdx {
			parts = append(parts,
				fgonlybg(pal.BG, pal.Accent, "")+
					fgbg(pal.BG, pal.Accent, bold(" "+idx+" "))+
					fgonlybg(pal.Accent, pal.Surface, "")+
					fgbg(pal.FG, pal.Surface, bold(" "+s.Name+" "))+
					fgonly(pal.Surface, ""))
		} else {
			parts = append(parts,
				fgonlybg(pal.BG, pal.Dim, "")+
					fgbg(pal.Surface, pal.Dim, " "+idx+" ")+
					fgonlybg(pal.Dim, pal.Surface, "")+
					fgbg(pal.Muted, pal.Surface, " "+s.Name+" ")+
					fgonly(pal.Surface, ""))
		}
	}
	return strings.Join(parts, "")
}

func renderTabsRpowerline(pal *theme.Palette, sessions []Session, currentIdx int) string {
	// Two-section rpowerline pills matching window tab format from generate.go:
	//   ╭ index ▸ name ╮
	// Current session gets accent highlight + ● icon; others get ○.
	var parts []string
	for _, s := range sessions {
		icon := "○"
		if s.Index == currentIdx {
			icon = "●"
		}
		if s.Index == currentIdx {
			parts = append(parts,
				fgonly(pal.Accent, "")+
					fgbg(pal.BG, pal.Accent, " "+icon+" ")+
					fgonlybg(pal.Accent, pal.Surface, "")+
					fgbg(pal.FG, pal.Surface, bold(" "+s.Name+" "))+
					fgonly(pal.Surface, ""))
		} else {
			parts = append(parts,
				fgonly(pal.Dim, "")+
					fgbg(pal.Surface, pal.Dim, " "+icon+" ")+
					fgonlybg(pal.Dim, pal.Surface, "")+
					fgbg(pal.Muted, pal.Surface, " "+s.Name+" ")+
					fgonly(pal.Surface, ""))
		}
	}
	return strings.Join(parts, "")
}

func renderTabsRounded(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts,
				fgonly(pal.Accent, "")+
					fgbg(pal.BG, pal.Accent, bold(" "+s.Name+" "))+
					fgonly(pal.Accent, ""))
		} else {
			parts = append(parts,
				fgonly(pal.Surface, "")+
					fgbg(pal.Dim, pal.Surface, " "+s.Name+" ")+
					fgonly(pal.Surface, ""))
		}
	}
	return strings.Join(parts, " ")
}

func renderTabsBlocks(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts, fg(pal.Accent, bold("["+s.Name+"]")))
		} else {
			parts = append(parts, fg(pal.Dim, "["+s.Name+"]"))
		}
	}
	return strings.Join(parts, " ")
}

func renderTabsHacker(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		label := fmt.Sprintf("%d:%s", s.Index, s.Name)
		if s.Index == currentIdx {
			parts = append(parts, fg(pal.Success, bold(label)))
		} else {
			parts = append(parts, fg(pal.Dim, label))
		}
	}
	return strings.Join(parts, fg(pal.Dim, "|"))
}

func renderTabsZen(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts, fg(pal.Muted, s.Name))
		} else {
			parts = append(parts, fg(pal.Dim, s.Name))
		}
	}
	return strings.Join(parts, fg(pal.Dim, " · "))
}

func renderTabsMinimal(pal *theme.Palette, sessions []Session, currentIdx int) string {
	var parts []string
	for _, s := range sessions {
		if s.Index == currentIdx {
			parts = append(parts, fg(pal.FG, bold(s.Name)))
		} else {
			parts = append(parts, fg(pal.Dim, s.Name))
		}
	}
	return strings.Join(parts, "  ")
}

// ── Workspace pill (preset-matched) ─────────────────────────────────
//
// These approximate each preset's workspace pill chrome so the top row
// of the two-line layout looks like it belongs. Phase 1+ will use the
// real preset renderer.

// RenderWorkspacePill renders a preset-matched workspace pill.
func RenderWorkspacePill(pal *theme.Palette, workspace string, preset bar.Preset) string {
	return renderWorkspacePill(pal, workspace, preset)
}

func renderWorkspacePill(pal *theme.Palette, workspace string, preset bar.Preset) string {
	if workspace == "" {
		return ""
	}
	label := "󱂬 " + workspace
	switch preset {
	case bar.Powerline:
		return fgonlybg(pal.BG, pal.Special, "") +
			fgbg(pal.BG, pal.Special, bold(" "+label+" ")) +
			fgonly(pal.Special, "")
	case bar.Rpowerline:
		return fgonly(pal.Special, "") +
			fgbg(pal.BG, pal.Special, bold(" "+label+" ")) +
			fgonly(pal.Special, "")
	case bar.Rounded:
		return fgonly(pal.Special, "") +
			fgbg(pal.BG, pal.Special, bold(" "+label+" ")) +
			fgonly(pal.Special, "")
	case bar.Blocks:
		return fg(pal.Special, bold("["+label+"]"))
	case bar.Hacker:
		return fg(pal.Success, label)
	case bar.Zen:
		return fg(pal.Dim, label)
	case bar.Minimal:
		return fg(pal.FG, bold(label))
	case bar.Starship:
		return fg(pal.Special, bold(label))
	default:
		// Default: rounded pill with special color (matches renderLeftDefault).
		return fgonly(pal.Special, "") +
			fgbg(pal.BG, pal.Special, bold(" "+label+" ")) +
			fgonly(pal.Special, "")
	}
}

// ── Layout renderers ────────────────────────────────────────────────

// RenderSingle: one-line bar. The indicator (dots/numbers/none) goes
// INSIDE the session pill via the BarContext.SessionIndicator field.
func RenderSingle(indicator string, pal *theme.Palette, sessions []Session, currentIdx int, preset bar.Preset, segments config.BarSegments, width int) string {
	return bar.RenderPreviewWithSegmentsOverride(preset, pal, segments, func(ctx *bar.BarContext) {
		applyIndicator(ctx, indicator, pal, sessions, currentIdx)
	})
}

// RenderTwoLine:
//
//	top row = preset-styled workspace pill + session indicator (tabs/dots)
//	bottom row = session bar (no workspace, no position numbers)
//
// The workspace pill on the top row uses the preset's own rendering so
// the icon + colors + chrome match exactly. The bottom row strips
// the workspace segment AND the position suffix (since the session
// indicator already communicates which session you're on).
//
// Collapses to single-line when only 1 session.
func RenderTwoLine(topVariant, indicator string, pal *theme.Palette, sessions []Session, currentIdx int, preset bar.Preset, segments config.BarSegments, workspace string, width int) (top, bottom string) {
	// ── Top: workspace pill + session content (always present) ──
	wsPill := renderWorkspacePill(pal, workspace, preset)
	sessionContent := renderTopBarContent(topVariant, pal, sessions, currentIdx, preset)
	top = wsPill + "  " + sessionContent

	// ── Bottom: session bar (no workspace, indicator in session pill) ──
	noWS := segments
	noWS.Workspace = false
	bottom = bar.RenderPreviewWithSegmentsOverride(preset, pal, noWS, func(ctx *bar.BarContext) {
		applyIndicator(ctx, indicator, pal, sessions, currentIdx)
	})
	return top, bottom
}

// RenderSplit: same content as two-line; page.go places the bottom bar
// below the editor content instead of directly under the top bar.
func RenderSplit(variant, indicator string, pal *theme.Palette, sessions []Session, currentIdx int, preset bar.Preset, segments config.BarSegments, workspace string, width int) (top, bottom string) {
	return RenderTwoLine(variant, indicator, pal, sessions, currentIdx, preset, segments, workspace, width)
}

// ── ANSI helpers ────────────────────────────────────────────────────

// fg sets foreground only; terminates with [39m so outer BG survives.
func fg(c theme.Color, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[39m", c.R, c.G, c.B, s)
}

// fgonly sets foreground without any terminator — used for separator
// glyphs that sit between bg-colored segments.
func fgonly(c theme.Color, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s", c.R, c.G, c.B, s)
}

// fgonlybg sets both fg and bg without a terminator — used for
// transition glyphs (powerline arrows) between two bg-colored sections.
func fgonlybg(fgc, bgc theme.Color, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm%s",
		fgc.R, fgc.G, fgc.B, bgc.R, bgc.G, bgc.B, s)
}

// fgbg sets both fg and bg; terminates with reset so it doesn't bleed.
func fgbg(fgc, bgc theme.Color, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm%s\033[0m",
		fgc.R, fgc.G, fgc.B, bgc.R, bgc.G, bgc.B, s)
}

func bold(s string) string {
	return "\033[1m" + s + "\033[22m"
}

// VisualLen returns the visible character count, stripping ANSI.
func VisualLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == 0x1b {
			inEsc = true
			continue
		}
		n++
	}
	return n
}
