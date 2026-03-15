package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/donjor/zmux/internal/tmux"
)

// HeaderStyles holds the lipgloss styles needed by the header renderer.
type HeaderStyles struct {
	Accent  lipgloss.Style
	Normal  lipgloss.Style
	Muted   lipgloss.Style
	Dim     lipgloss.Style
	Title   lipgloss.Style
	Selected lipgloss.Style
}

// RenderHeader renders the dashboard header with session name, directory,
// and window tab pills.
func RenderHeader(session string, dir string, windows []tmux.Window, styles HeaderStyles, width int) string {
	var b strings.Builder

	// Session name as accent pill.
	pill := styles.Accent.
		Bold(true).
		Padding(0, 1).
		Render(session)
	b.WriteString(pill)

	// Directory.
	if dir != "" {
		b.WriteString("  ")
		b.WriteString(styles.Dim.Render(dir))
	}

	b.WriteString("\n")

	// Window tab pills.
	if len(windows) > 0 {
		var tabs []string
		for _, w := range windows {
			label := w.Name
			if w.Active {
				tab := styles.Selected.
					Padding(0, 1).
					Render(label)
				tabs = append(tabs, tab)
			} else {
				tab := styles.Muted.
					Padding(0, 1).
					Render(label)
				tabs = append(tabs, tab)
			}
		}
		b.WriteString(strings.Join(tabs, " "))
		b.WriteString("\n")
	}

	return b.String()
}
