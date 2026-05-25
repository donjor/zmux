package pane

// Layout renderers for the pane preview. Each layout (split, grid, stacked,
// focus-rail) decides geometry then delegates each pane to renderPaneBlock,
// which owns the per-pane chrome (border, header, metrics, body).

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/preview"
)

// ── Layouts ──

func renderSplit(primary, secondary paneSpec, width int, headerMode, dividerMode string, hints bool, auxPct int) string {
	rightW := clamp(width*auxPct/100, 28, width-32)
	leftW := width - rightW - 1
	if leftW < 32 {
		leftW = 32
		rightW = width - leftW - 1
	}
	gap := dividerGap(dividerMode, primary.Focused || secondary.Focused)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderPaneBlock(primary, leftW, 10, headerMode, hints),
		gap,
		renderPaneBlock(secondary, rightW, 10, headerMode, hints),
	)
}

func renderGrid(panes []paneSpec, width int, headerMode, dividerMode string, hints bool, auxPct int) string {
	rightW := clamp(width*auxPct/100, 26, width-26)
	leftW := width - rightW - 1
	if leftW < 26 {
		leftW = 26
		rightW = width - leftW - 1
	}
	left := renderPaneBlock(panes[0], leftW, 16, headerMode, hints)
	upper := renderPaneBlock(panes[1], rightW, 6, headerMode, hints)
	lower := renderPaneBlock(panes[2], rightW, 6, headerMode, hints)
	right := upper + "\n" + lower
	return lipgloss.JoinHorizontal(lipgloss.Top, left, dividerGap(dividerMode, anyFocused(panes)), right)
}

func renderStacked(panes []paneSpec, width int, headerMode, dividerMode string, hints bool) string {
	var blocks []string
	for _, pane := range panes {
		blocks = append(blocks, renderPaneBlock(pane, width, 5, headerMode, hints))
	}
	return strings.Join(blocks, "\n")
}

func renderFocusRail(panes []paneSpec, width int, headerMode, dividerMode string, hints bool, railPct int) string {
	focused := panes[0]
	for _, pane := range panes {
		if pane.Focused {
			focused = pane
			break
		}
	}
	rail := make([]paneSpec, 0, len(panes)-1)
	for _, pane := range panes {
		if pane.ID != focused.ID {
			rail = append(rail, pane)
		}
	}
	rightW := clamp(width*railPct/100, 28, width-34)
	leftW := width - rightW - 1
	left := renderPaneBlock(focused, leftW, 14, headerMode, hints)
	var rightBlocks []string
	for _, pane := range rail {
		rightBlocks = append(rightBlocks, renderPaneBlock(pane, rightW, 5, headerMode, hints))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, dividerGap(dividerMode, true), strings.Join(rightBlocks, "\n"))
}

// ── Single pane block + chrome ──

func renderPaneBlock(p paneSpec, width, bodyHeight int, headerMode string, hints bool) string {
	innerW := max(12, width-2)
	accent := paneAccent(p)
	border := preview.Dim
	if p.Focused {
		border = preview.Gold
	} else if p.State == stateAttention {
		border = preview.Orange
	} else if p.State == stateStale {
		border = preview.Red
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(innerW)
	if p.Focused {
		style = style.Bold(true)
	}

	header := richPaneHeader(p, innerW, headerMode, hints, accent)
	metrics := richPaneMetrics(p, innerW, accent)
	body := richPaneBody(p, innerW, bodyHeight)
	return style.Render(header + "\n" + metrics + "\n" + body)
}

func richPaneHeader(p paneSpec, width int, mode string, hints bool, accent color.Color) string {
	mark := "○"
	if p.Focused {
		mark = "●"
	}
	state := stateLabel(p)
	var raw string
	if mode == headerCompact {
		raw = mark + " " + p.ID + " " + p.Title
		if state != "" {
			raw += "  " + state
		}
		return fitVisual(lipgloss.NewStyle().Foreground(accent).Bold(p.Focused).Render(fit(raw, width)), width)
	}
	if mode == headerRibbon {
		raw = mark + " " + p.Title
		if hints {
			raw += "  " + localHints(p, hints)
		}
		return fitVisual(lipgloss.NewStyle().Foreground(preview.BGDark).Background(accent).Bold(true).Render(fit(" "+raw+" ", width)), width)
	}
	raw = mark + " " + p.ID + " " + p.Title + "  " + p.Command
	if state != "" {
		raw += "  " + state
	}
	return fitVisual(lipgloss.NewStyle().Foreground(accent).Bold(p.Focused).Render(fit(raw, width)), width)
}

func richPaneMetrics(p paneSpec, width int, accent color.Color) string {
	trend := preview.Sparkline(paneTrend(p), accent)
	trendW := lipgloss.Width(trend)
	metaW := max(1, width-trendW-1)
	meta := preview.MuteStyle.Render(fit(p.CWD+" · "+p.Size, metaW))
	gap := width - lipgloss.Width(meta) - trendW
	if gap < 1 {
		gap = 1
	}
	return fitVisual(meta+strings.Repeat(" ", gap)+trend, width)
}

func richPaneBody(p paneSpec, width, bodyHeight int) string {
	var lines []string
	for i := 0; i < bodyHeight; i++ {
		content := ""
		if i < len(p.Lines) {
			content = p.Lines[i]
		}
		if i == len(p.Lines)+1 && !p.Focused {
			content = "Alt+Shift+Arrow focuses this pane"
		}
		prefix := preview.DimStyle.Render("│ ")
		if p.Focused {
			prefix = lipgloss.NewStyle().Foreground(preview.Gold).Bold(true).Render("▌ ")
		} else if p.State == stateAttention {
			prefix = lipgloss.NewStyle().Foreground(preview.Orange).Render("│ ")
		} else if p.State == stateStale {
			prefix = lipgloss.NewStyle().Foreground(preview.Red).Render("│ ")
		}
		text := preview.FGStyle.Render(fit(content, max(1, width-2)))
		lines = append(lines, prefix+fitVisual(text, max(1, width-2)))
	}
	return strings.Join(lines, "\n")
}

// ── Per-pane attributes ──

func paneAccent(p paneSpec) color.Color {
	if p.Focused {
		return preview.Gold
	}
	switch p.Slot {
	case focusSecondary:
		return preview.Blue
	case focusTertiary:
		return preview.Purple
	default:
		return preview.Teal
	}
}

func paneTrend(p paneSpec) []float64 {
	switch p.State {
	case stateAttention:
		return []float64{0.32, 0.44, 0.58, 0.73, 0.88, 0.79, 0.9, 0.82}
	case stateStale:
		return []float64{0.7, 0.62, 0.52, 0.44, 0.38, 0.3, 0.24, 0.2}
	default:
		return []float64{0.2, 0.34, 0.31, 0.45, 0.58, 0.52, 0.64, 0.72}
	}
}

func dividerGap(mode string, active bool) string {
	glyph := " "
	switch mode {
	case dividerStrong:
		glyph = "┃"
	case dividerRounded:
		glyph = "┆"
	}
	if active && glyph != " " {
		return lipgloss.NewStyle().Foreground(preview.Gold).Bold(true).Render(glyph)
	}
	return preview.DimStyle.Render(glyph)
}

func stateLabel(p paneSpec) string {
	switch p.State {
	case stateAttention:
		return "◆ attention"
	case stateStale:
		return "△ stale"
	default:
		return ""
	}
}

func localHints(p paneSpec, enabled bool) string {
	if !enabled {
		return ""
	}
	switch p.Slot {
	case focusSecondary:
		if p.Title == "clean-ui" {
			return "A-S-← main · sidecar-focus · sidecar-close"
		}
		return "A-S-←/→ focus · pfx+z zoom · pfx+q ids"
	case focusTertiary:
		return "A-S-↑/↓ focus · x close"
	default:
		return "A-S-→ next pane · pfx+q ids"
	}
}

func anyFocused(panes []paneSpec) bool {
	for _, p := range panes {
		if p.Focused {
			return true
		}
	}
	return false
}
