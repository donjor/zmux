package pane

// Generic value-extraction, width-fitting, and clamp helpers for the pane
// preview. Nothing here knows about panes — these are utilities shared by
// the layout/block renderers and the top-level Render entry point.

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/preview"
)

const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
)

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func stringValue(ctx preview.RenderContext, id preview.ControlID, fallback string) string {
	if v, ok := ctx.Values[id].(string); ok && v != "" {
		return v
	}
	return fallback
}

func boolValue(ctx preview.RenderContext, id preview.ControlID, fallback bool) bool {
	if v, ok := ctx.Values[id].(bool); ok {
		return v
	}
	return fallback
}

func intValue(ctx preview.RenderContext, id preview.ControlID, fallback int) int {
	if v, ok := ctx.Values[id].(int); ok {
		return v
	}
	return fallback
}

func fitVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		if width == 1 {
			return "…"
		}
		return string(r[:width-1]) + "…"
	}
	return s + strings.Repeat(" ", width-len(r))
}

func dim(s string) string { return ansiDim + s + ansiReset }

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
