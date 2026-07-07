package views

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderEmptyState renders a centered empty state message.
func RenderEmptyState(message string, hint string, dim lipgloss.Style) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + dim.Render(message) + "\n")
	if hint != "" {
		b.WriteString("  " + dim.Render(hint) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}
