package tabs

// View rendering for the Bar tab. Split out so bar.go can stay focused on
// lifecycle + message routing.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/bar"
)

// previewSessions are mock session names for the two-line preview.
var previewSessions = []string{"main", "api", "tests"}

func (t *BarTab) View() string {
	var b strings.Builder
	cursorLine := 0
	lineCount := 0

	b.WriteString("\n")
	lineCount++
	currentLabel := "default"
	if t.currentBar != "" {
		currentLabel = t.currentBar
	}
	if t.layout != "" {
		currentLabel += " (" + t.layout + ")"
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("\n\n")
	lineCount += 2

	// ── Presets ──

	for i, preset := range t.presets {
		selected := t.currentSection() == barPresets && t.cursor == i
		isCurrent := preset.String() == t.currentBar

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		nameStyle := t.styles.Normal
		if selected {
			nameStyle = t.styles.Accent.Bold(true)
		}

		currentMark := ""
		if isCurrent {
			currentMark = t.styles.Success.Render(" *")
		}

		b.WriteString("  " + cursor + nameStyle.Render(preset.String()) + currentMark + "\n")
		lineCount++

		if t.palette != nil {
			preview := t.renderPresetPreview(preset)
			for _, line := range strings.Split(preview, "\n") {
				b.WriteString("    " + line + "\n")
				lineCount++
			}
		}
		b.WriteString("\n")
		lineCount++
	}

	// ── Layout options ──

	b.WriteString("  " + t.styles.Muted.Render("Layout") + "\n\n")
	lineCount += 2

	P := len(t.presets)
	for i, opt := range barLayoutOptions {
		idx := P + i
		selected := t.cursor == idx

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		value := t.layoutValue(opt.Field)
		labelStyle := t.styles.Normal
		valueStyle := t.styles.Success
		if selected {
			labelStyle = t.styles.Accent
		}

		fmt.Fprintf(
			&b,
			"  %s%s  ◀ %s ▶\n",
			cursor,
			labelStyle.Render(opt.Label+":"),
			valueStyle.Render(value),
		)
		lineCount++
	}
	b.WriteString("  " + t.styles.Muted.Render(layoutHint(t.layoutValue("layout"))) + "\n")
	lineCount++
	b.WriteString("\n")
	lineCount++

	// ── Segment toggles ──

	b.WriteString("  " + t.styles.Muted.Render("Segments") + "\n\n")
	lineCount += 2

	segBase := P + len(barLayoutOptions)
	for i, seg := range barSegmentLabels {
		idx := segBase + i
		selected := t.cursor == idx

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		enabled := t.segmentEnabled(seg.Field)
		checkbox := t.styles.Dim.Render("[ ]")
		if enabled {
			checkbox = t.styles.Success.Render("[x]")
		}

		label := t.styles.Normal.Render(seg.Label)
		if selected {
			label = t.styles.Accent.Render(seg.Label)
		}

		b.WriteString("  " + cursor + checkbox + " " + label + "\n")
		lineCount++
	}

	t.vp.SetContent(b.String())
	ensureCursorVisible(&t.vp, cursorLine)
	return renderScrollable(t.vp, t.styles)
}

func (t *BarTab) renderPresetPreview(preset bar.Preset) string {
	width := t.width - 8
	if width < 40 {
		width = 60
	}

	switch t.layout {
	case "two-line", "split":
		topRow := bar.RenderTopPreviewVariant(
			preset, t.palette, previewSessions, previewSessions[0], width, t.topBar,
		)
		if topRow == "" {
			return bar.RenderPreviewWithSegments(preset, t.palette, t.segments)
		}
		noWS := t.segments
		noWS.Workspace = false
		bottomRow := bar.RenderBarPreviewOverride(preset, t.palette, noWS, width,
			func(bctx *bar.BarContext) {
				// Top row owns identity in two-line mode (plan 024) — flag it
				// so the dashboard preview matches the live bottom-left.
				bctx.TopRowActive = true
				switch t.indicator {
				case "dots":
					bctx.SessionIndicator = bar.CompactDots(previewSessions, previewSessions[0], nil)
				case "none":
					bctx.WorkspaceCount = 1
				}
			})
		return topRow + "\n" + bottomRow
	default:
		return bar.RenderPreviewWithSegments(preset, t.palette, t.segments)
	}
}
