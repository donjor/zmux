package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/donjor/zmux/internal/tui"
)

// RenderTabBar renders the top tab bar with the active tab highlighted.
func RenderTabBar(tabs []TabID, active TabID, styles tui.Styles, width int) string {
	var parts []string

	for _, id := range tabs {
		label := tabLabel(id)
		if id == active {
			// Active tab: bold accent with underline.
			style := styles.Accent.
				Bold(true).
				Underline(true).
				Padding(0, 2)
			parts = append(parts, style.Render(label))
		} else {
			// Inactive tab: muted.
			style := styles.Dim.
				Padding(0, 2)
			parts = append(parts, style.Render(label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	// Separator line spanning full width.
	sepChar := "─"
	sep := styles.Dim.Render(strings.Repeat(sepChar, max(0, width)))

	return bar + "\n" + sep
}

// RenderHelpBar renders the bottom help bar with contextual hints.
func RenderHelpBar(tabHelp string, styles tui.Styles, width int) string {
	// Global keys always shown.
	globalHelp := "1-4:tabs  tab/shift+tab:cycle  esc:quit"

	left := styles.Dim.Render(tabHelp)
	right := styles.Dim.Render(globalHelp)

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		// Narrow mode: stack vertically.
		return left + "\n" + right
	}

	return left + strings.Repeat(" ", gap) + right
}

// RenderStatusFlash renders a status message in the tab bar area.
func RenderStatusFlash(text string, isError bool, styles tui.Styles) string {
	if text == "" {
		return ""
	}
	if isError {
		return styles.Error.Render(" " + text)
	}
	return styles.Success.Render(" " + text)
}

// RenderTooSmall renders the minimum-size warning.
func RenderTooSmall(styles tui.Styles) string {
	msg := styles.Dim.Render("Terminal too small. Resize to at least 60x16.")
	return "\n\n" + msg
}

func tabLabel(id TabID) string {
	switch id {
	case TabCurrent:
		return "This Session"
	case TabSessions:
		return "Sessions"
	case TabSettings:
		return "Settings"
	case TabHelp:
		return "Help"
	default:
		return string(id)
	}
}
