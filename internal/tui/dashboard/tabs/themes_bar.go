package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

// ============================================================================
// Bar segment labels
// ============================================================================

var themesBarSegmentLabels = []struct {
	Label string
	Field string
}{
	{"Git branch", "git"},
	{"Workspace", "workspace"},
	{"Clock", "clock"},
	{"Language", "lang"},
	{"Directory", "directory"},
	{"Process", "process"},
	{"Group indicator", "group"},
}

// ============================================================================
// Key handling — Bar section
// ============================================================================

func (t *ThemesTab) handleBarKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	totalPresets := len(t.barPresets)
	totalSegments := len(themesBarSegmentLabels)

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx <= 0 {
				t.barInSegments = false
				t.barCursor = totalPresets - 1
			} else {
				t.barCursor--
			}
		} else {
			if t.barCursor > 0 {
				t.barCursor--
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx < totalSegments-1 {
				t.barCursor++
			}
		} else {
			if t.barCursor < totalPresets-1 {
				t.barCursor++
			} else {
				t.barInSegments = true
				t.barCursor = totalPresets
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx >= 0 && segIdx < totalSegments {
				t.toggleSegment(themesBarSegmentLabels[segIdx].Field)
				t.cfg.Bar.Segments = t.barSegments
				return t, t.saveConfig()
			}
		} else if t.barCursor < totalPresets {
			preset := t.barPresets[t.barCursor]
			t.currentBar = preset.String()
			t.cfg.Bar.Preset = preset.String()
			return t, t.saveConfig()
		}
		return t, nil
	}

	return t, nil
}

// ============================================================================
// View — Bar section
// ============================================================================

func (t *ThemesTab) viewBar() string {
	var b strings.Builder

	b.WriteString("\n")
	currentLabel := "default"
	if t.currentBar != "" {
		currentLabel = t.currentBar
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("\n\n")

	// Resolve palette for previews.
	var palette *theme.Palette
	if t.resolver != nil && t.currentTheme != "" {
		resolved, err := t.resolver.Resolve(t.currentTheme)
		if err == nil {
			p := resolved.SemanticPalette()
			palette = &p
		}
	}

	for i, preset := range t.barPresets {
		selected := i == t.barCursor && !t.barInSegments
		isCurrent := preset.String() == t.currentBar

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

		if palette != nil {
			preview := bar.RenderPreviewWithSegments(preset, palette, t.barSegments)
			b.WriteString("    " + preview + "\n")
		}
		b.WriteString("\n")
	}

	// Segment toggles.
	b.WriteString("  " + t.styles.Muted.Render("Segments") + "\n\n")

	totalPresets := len(t.barPresets)
	for i, seg := range themesBarSegmentLabels {
		idx := totalPresets + i
		selected := t.barInSegments && t.barCursor == idx

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
	}

	return b.String()
}

// ============================================================================
// Bar segment helpers
// ============================================================================

func (t *ThemesTab) toggleSegment(field string) {
	switch field {
	case "git":
		t.barSegments.Git = !t.barSegments.Git
	case "workspace":
		t.barSegments.Workspace = !t.barSegments.Workspace
	case "clock":
		t.barSegments.Clock = !t.barSegments.Clock
	case "lang":
		t.barSegments.Lang = !t.barSegments.Lang
	case "directory":
		t.barSegments.Directory = !t.barSegments.Directory
	case "process":
		t.barSegments.Process = !t.barSegments.Process
	case "group":
		t.barSegments.Group = !t.barSegments.Group
	}
}

func (t *ThemesTab) segmentEnabled(field string) bool {
	switch field {
	case "git":
		return t.barSegments.Git
	case "workspace":
		return t.barSegments.Workspace
	case "clock":
		return t.barSegments.Clock
	case "lang":
		return t.barSegments.Lang
	case "directory":
		return t.barSegments.Directory
	case "process":
		return t.barSegments.Process
	case "group":
		return t.barSegments.Group
	}
	return false
}
