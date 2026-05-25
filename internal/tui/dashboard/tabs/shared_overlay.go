package tabs

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	"github.com/donjor/zmux/internal/tui/styles"
)

// renderRenameOverlayShared renders the inline rename input prompt used by
// both the Session & Workspace tab and the Workspaces tab. The kind label
// (e.g. "workspace", "session", "window") is appended to the prompt verb so
// users can see what they're renaming.
func renderRenameOverlayShared(styles styles.Styles, kind string, input textinput.Model) string {
	verb := "rename"
	if kind != "" {
		verb = "rename " + kind
	}
	prompt := styles.Accent.Render("  " + verb + " ▸ ")
	return prompt + input.View() + "\n\n"
}

// renderConfirmOverlayShared renders the y/N kill prompt. step 1 is the
// normal confirmation; step 2 is the red "this will detach you" warning
// shown when killing an attached workspace. Both tabs render this exactly
// the same way.
func renderConfirmOverlayShared(styles styles.Styles, c *confirmState, step int) string {
	if c == nil {
		return ""
	}

	var b strings.Builder
	if step == 2 {
		b.WriteString("  ")
		b.WriteString(styles.Error.Bold(true).Render("⚠ This will detach you from tmux."))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(styles.Error.Render("Kill workspace "))
		b.WriteString(styles.Error.Bold(true).Render(c.name))
		b.WriteString(styles.Error.Render("? "))
		b.WriteString(styles.Dim.Render("(y/N)"))
		b.WriteString("\n\n")
		return b.String()
	}

	verb := "Kill " + c.kind + " "
	b.WriteString("  ")
	b.WriteString(styles.Error.Render(verb))
	b.WriteString(styles.Error.Bold(true).Render(c.name))
	b.WriteString(styles.Error.Render("? "))
	b.WriteString(styles.Dim.Render("(y/N)"))
	b.WriteString("\n\n")
	return b.String()
}
