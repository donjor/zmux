package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// TabBarEntry holds display data for a single tab in the tab bar.
type TabBarEntry struct {
	Label    string
	IsActive bool
}

// TabBarStyles holds the lipgloss styles needed by the tab bar renderer.
type TabBarStyles struct {
	Accent lipgloss.Style
	Dim    lipgloss.Style
}

// RenderTabBar renders a horizontal tab bar with accent underline on the active tab.
func RenderTabBar(entries []TabBarEntry, styles TabBarStyles, width int) string {
	var parts []string

	for _, e := range entries {
		if e.IsActive {
			style := styles.Accent.
				Bold(true).
				Underline(true).
				Padding(0, 2)
			parts = append(parts, style.Render(e.Label))
		} else {
			style := styles.Dim.
				Padding(0, 2)
			parts = append(parts, style.Render(e.Label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	// Subtle separator underneath.
	sep := styles.Dim.Render(strings.Repeat("─", max(0, width)))

	return bar + "\n" + sep
}
