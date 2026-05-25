package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/session"
)

// SessionListStyles holds the lipgloss styles needed by the session list renderer.
type SessionListStyles struct {
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Accent   lipgloss.Style
	Dim      lipgloss.Style
	Muted    lipgloss.Style
	Success  lipgloss.Style
}

// RenderSessionList renders the session list with named sessions first and tmp
// sessions last. The currently selected session is highlighted.
func RenderSessionList(sessions []session.SessionInfo, cursor int, styles SessionListStyles, width int, height int) string {
	if len(sessions) == 0 {
		return styles.Muted.Render("  No sessions found.") + "\n"
	}

	var b strings.Builder

	// Calculate visible window for scrolling.
	start := 0
	if cursor >= height {
		start = cursor - height + 1
	}
	end := start + height
	if end > len(sessions) {
		end = len(sessions)
	}

	// Section tracking for named vs tmp divider.
	shownDivider := false
	for i := start; i < end; i++ {
		s := sessions[i]

		// Show divider before the first tmp session.
		if s.IsTmp && !shownDivider {
			// Check if there were any named sessions before this.
			hasNamed := false
			for j := 0; j < i; j++ {
				if !sessions[j].IsTmp {
					hasNamed = true
					break
				}
			}
			if hasNamed {
				b.WriteString(styles.Dim.Render("  ──── tmp ────") + "\n")
			}
			shownDivider = true
		}

		b.WriteString(renderSessionEntry(i, cursor, s, styles))
	}

	// Scroll indicators.
	if start > 0 {
		b.WriteString(styles.Dim.Render("  ... more above") + "\n")
	}
	if end < len(sessions) {
		b.WriteString(styles.Dim.Render("  ... more below") + "\n")
	}

	return b.String()
}

func renderSessionEntry(idx, cursor int, s session.SessionInfo, styles SessionListStyles) string {
	// Cursor indicator.
	cursorStr := "  "
	if idx == cursor {
		cursorStr = styles.Accent.Render("> ")
	}

	// Name.
	nameStyle := styles.Normal
	if idx == cursor {
		nameStyle = styles.Selected
	}
	if s.IsTmp {
		nameStyle = nameStyle.Foreground(styles.Dim.GetForeground())
	}
	name := nameStyle.Render(s.Name)

	// Metadata: window count.
	meta := styles.Dim.Render(fmt.Sprintf(" %dw", s.Windows))

	// Age.
	if !s.Activity.IsZero() {
		meta += styles.Dim.Render(" " + session.HumanAge(s.Activity))
	}

	// Dir (shortened).
	if s.Dir != "" {
		meta += styles.Dim.Render(" " + s.Dir)
	}

	// Attached indicator.
	attached := ""
	if s.Attached {
		attached = styles.Success.Render(" *")
	}

	return cursorStr + name + meta + attached + "\n"
}
