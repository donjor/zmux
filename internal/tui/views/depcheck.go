package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// DepCheckStyles holds the lipgloss styles needed by the dependency check view.
type DepCheckStyles struct {
	Success lipgloss.Style
	Error   lipgloss.Style
	Normal  lipgloss.Style
	Muted   lipgloss.Style
}

// RenderDepCheck renders a dependency check summary with check/cross marks.
// tmuxVersion is the detected tmux version string (empty if not found).
// clipboard is the detected clipboard tool name (empty if not found).
func RenderDepCheck(tmuxVersion string, clipboard string, styles DepCheckStyles) string {
	var b strings.Builder

	// tmux check
	if tmuxVersion != "" {
		b.WriteString(styles.Success.Render("  [ok]"))
		b.WriteString(styles.Normal.Render(fmt.Sprintf("  tmux %s", tmuxVersion)))
	} else {
		b.WriteString(styles.Error.Render("  [!!]"))
		b.WriteString(styles.Normal.Render("  tmux not found"))
	}
	b.WriteString("\n")

	// clipboard check
	if clipboard != "" {
		b.WriteString(styles.Success.Render("  [ok]"))
		b.WriteString(styles.Normal.Render(fmt.Sprintf("  clipboard: %s", clipboard)))
	} else {
		b.WriteString(styles.Muted.Render("  [--]"))
		b.WriteString(styles.Muted.Render("  clipboard: none detected (optional)"))
	}
	b.WriteString("\n")

	return b.String()
}
