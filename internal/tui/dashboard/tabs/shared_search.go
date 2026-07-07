package tabs

// Shared `/` search-input behavior for the Workspaces and Session &
// Workspace tabs. Mode transitions and pending-data flushes stay per-tab
// (their mode enums and data types differ); the key handling and the
// overlay prompt are identical and live here.

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
)

// handleSearchInputKey drives the inline search input. Typing live-filters
// via rebuild; Enter applies the filter, Esc clears it — both return
// done=true so the caller can leave search mode. Arrow keys move the cursor
// through the filtered results without leaving the input.
func handleSearchInputKey(msg tea.KeyMsg, input *textinput.Model, query *string, tree *outline.Tree, rebuild func()) (done bool, cmd tea.Cmd) {
	switch msg.String() {
	case "enter":
		*query = strings.TrimSpace(input.Value())
		return true, nil
	case "esc":
		*query = ""
		input.SetValue("")
		return true, nil
	case "up":
		tree.MoveUp()
		return false, nil
	case "down":
		tree.MoveDown()
		return false, nil
	}

	*input, cmd = input.Update(msg)
	*query = strings.TrimSpace(input.Value())
	rebuild()
	return false, cmd
}

// renderSearchOverlayShared renders the inline `/` search input prompt.
func renderSearchOverlayShared(st styles.Styles, input textinput.Model) string {
	return st.Accent.Render("  search ▸ ") + input.View() + "\n\n"
}

// confirmKillEscalate reports whether a confirmed workspace kill must route
// through the second "still has attached sessions" confirmation step.
func confirmKillEscalate(c *confirmState, atSecondStep bool) bool {
	return c.kind == "workspace" && c.attached && !atSecondStep
}
