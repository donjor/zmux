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
