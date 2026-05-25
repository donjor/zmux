package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// WindowRow holds the pre-computed display data for a single window entry.
type WindowRow struct {
	Index      int
	Name       string
	IsActive   bool
	IsSelected bool
	Command    string // primary pane command
	Dir        string // shortened directory
	Uptime     string // formatted uptime
	CPU        string // e.g. "2.0%" or ""
	Mem        string // e.g. "566MB" or ""
	IsIdle     bool   // true if shell with no children
}

// RenderWindowRow renders a two-line window entry in premium format:
//
//	▸ 1: claude  ●                              1h 38m
//	  claude (claude code)  ~/project  2.0%  566MB
func RenderWindowRow(row WindowRow, styles SessionRowStyles, width int) string {
	var b strings.Builder

	// ── Line 1: cursor + index:name + active dot + uptime ──

	cursor := "  "
	if row.IsSelected {
		cursor = styles.Accent.Render("▸ ")
	}

	// Name styling.
	nameStyle := styles.Normal.Bold(true)
	if row.IsSelected {
		nameStyle = styles.Accent.Bold(true)
	}

	nameStr := nameStyle.Render(fmt.Sprintf("%d: %s", row.Index, row.Name))

	// Active indicator.
	activeDot := ""
	if row.IsActive {
		activeDot = "  " + styles.Success.Render("●")
	}

	// Uptime (right-aligned).
	uptimeStr := ""
	if row.Uptime != "" {
		uptimeStr = styles.Dim.Render(row.Uptime)
	}

	leftL1 := "  " + cursor + nameStr + activeDot
	rightL1 := uptimeStr

	leftWidth := lipgloss.Width(leftL1)
	rightWidth := lipgloss.Width(rightL1)
	gap := width - leftWidth - rightWidth - 4
	if gap < 2 {
		gap = 2
	}

	b.WriteString(leftL1 + strings.Repeat(" ", gap) + rightL1 + "\n")

	// ── Line 2: indent + command + dir + CPU/mem ──

	indent := "      "

	var parts []string

	if row.IsIdle {
		parts = append(parts, styles.Dim.Render("(idle)"))
	} else if row.Command != "" {
		parts = append(parts, styles.Normal.Render(row.Command))
	}

	if row.Dir != "" {
		parts = append(parts, styles.Dim.Render(row.Dir))
	}

	if row.CPU != "" {
		parts = append(parts, styles.Info.Render(row.CPU))
	}

	if row.Mem != "" {
		parts = append(parts, styles.Info.Render(row.Mem))
	}

	if len(parts) > 0 {
		b.WriteString(indent + strings.Join(parts, "  ") + "\n")
	}

	return b.String()
}
