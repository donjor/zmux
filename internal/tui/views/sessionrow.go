package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SessionRow holds the pre-computed display data for a single session entry.
// Both the session picker and sessions tab map their models into this struct,
// then hand it to the shared renderer.
type SessionRow struct {
	Name          string
	Age           string // e.g. "2h", "5m"
	StatusText    string // e.g. "attached", ""
	WindowsText   string // e.g. "[editor, server, git]"
	DirectoryText string // e.g. "~/work"
	IsCurrent     bool
	IsAttached    bool
	IsTmp         bool
	IsSelected    bool // cursor is on this row
	Index         int  // 1-based quick-select index (0 = no index)
}

// SessionRowStyles holds the lipgloss styles needed by the session row renderer.
type SessionRowStyles struct {
	Normal  lipgloss.Style
	Accent  lipgloss.Style
	Dim     lipgloss.Style
	Info    lipgloss.Style
	Success lipgloss.Style
}

// RenderSessionRow renders a two-line session entry in the premium format:
//
//	  ▸ dev                                  2h ago
//	    ● attached  [editor, server, git]   ~/work
func RenderSessionRow(row SessionRow, styles SessionRowStyles, width int) string {
	var b strings.Builder

	// ── Line 1: cursor + name + age ──

	// Cursor indicator.
	cursor := "  "
	if row.IsSelected {
		cursor = styles.Accent.Render("▸ ")
	}

	// Quick-select index.
	indexStr := " "
	if row.Index > 0 && row.Index <= 9 {
		indexStr = styles.Dim.Render(fmt.Sprintf("%d", row.Index))
	}

	// Name styling.
	nameStyle := styles.Normal.Bold(true)
	if row.IsSelected {
		nameStyle = styles.Accent.Bold(true)
	} else if row.IsTmp {
		nameStyle = styles.Dim
	}

	// Current session marker.
	currentMark := ""
	if row.IsCurrent {
		currentMark = styles.Success.Render(" *")
	}

	nameStr := nameStyle.Render(row.Name) + currentMark

	// Age (right-aligned effect via spacing).
	ageStr := ""
	if row.Age != "" {
		ageStr = styles.Dim.Render(row.Age)
	}

	// Build line 1 with flexible gap.
	leftL1 := "  " + cursor + indexStr + " " + nameStr
	rightL1 := ageStr

	leftWidth := lipgloss.Width(leftL1)
	rightWidth := lipgloss.Width(rightL1)
	gap := width - leftWidth - rightWidth - 4 // 4 for padding
	if gap < 2 {
		gap = 2
	}

	b.WriteString(leftL1 + strings.Repeat(" ", gap) + rightL1 + "\n")

	// ── Line 2: status + windows + directory ──

	// Indent to align with name (past cursor + index).
	indent := "      "

	// Attached status indicator.
	statusDot := ""
	statusLabel := ""
	if row.IsAttached {
		dotStyle := styles.Info
		if row.IsSelected {
			dotStyle = styles.Info.Bold(true)
		}
		statusDot = dotStyle.Render("●")
		statusLabel = styles.Info.Render(" attached")
	} else {
		statusDot = styles.Dim.Render("○")
		statusLabel = styles.Dim.Render(" idle")
	}

	// Windows list.
	windowsStr := ""
	if row.WindowsText != "" {
		windowsStr = "  " + styles.Dim.Render(row.WindowsText)
	}

	// Directory.
	dirStr := ""
	if row.DirectoryText != "" {
		dirStr = "  " + styles.Dim.Render(row.DirectoryText)
	}

	b.WriteString(indent + statusDot + statusLabel + windowsStr + dirStr + "\n")

	return b.String()
}

// RenderSessionDivider renders a subtle divider between named and tmp sessions.
func RenderSessionDivider(dim lipgloss.Style, width int) string {
	lineWidth := width - 8
	if lineWidth < 10 {
		lineWidth = 10
	}
	if lineWidth > 50 {
		lineWidth = 50
	}
	return "      " + dim.Render(strings.Repeat("─", lineWidth)) + "\n"
}

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
