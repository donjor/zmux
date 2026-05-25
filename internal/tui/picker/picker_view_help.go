package picker

import (
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// viewHelp renders the bottom help bar, with hints adapting to the
// current mode and the row kind under the cursor.
func (m PickerModel) viewHelp() string {
	switch m.mode {
	case modeConfirmDelete:
		return m.styles.Help.Render("  y:confirm  any:cancel")
	case modeConfirmDeleteAttached:
		return m.styles.Help.Render("  y:confirm detach  any:cancel")
	case modeTemplateSelect:
		return m.styles.Help.Render("  enter:select  esc:cancel")
	case modeTemplateName:
		return m.styles.Help.Render("  enter:create  esc:back")
	}

	parts := []string{}
	row := m.tree.CurrentSelectable()

	if row == nil {
		parts = append(parts, "enter:tmp")
	} else {
		switch row.Kind {
		case outline.RowTopAction:
			if m.state.workspaceQuery == "" {
				parts = append(parts, "enter:tmp")
			} else {
				parts = append(parts, "enter:create")
			}
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](row); ok && ws != nil && len(ws.LiveSessions) == 0 {
				parts = append(parts, "enter:create+attach")
			} else {
				parts = append(parts, "enter:attach")
			}
			parts = append(parts, "ctrl+x:kill")
		case outline.RowSession:
			parts = append(parts, "enter:attach")
			parts = append(parts, "ctrl+x:kill")
		case outline.RowExternalGroup:
			parts = append(parts, "enter:toggle")
		case outline.RowExternalEntry:
			parts = append(parts, "enter:connect")
		}
	}

	parts = append(parts, "tab:complete")
	toggleLabel := "ctrl+h:show-empty"
	if m.state.showEmpty {
		toggleLabel = "ctrl+h:hide-empty"
	}
	parts = append(parts, toggleLabel)
	parts = append(parts, "ctrl+t:template")
	if m.state.workspaceQuery != "" || m.state.sessionQuery != "" {
		parts = append(parts, "esc:clear")
	} else {
		parts = append(parts, "esc:quit")
	}

	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}

// ghostCmd renders the dimmed-prompt preview of the command the picker
// will issue if the user hits enter right now.
func (m PickerModel) ghostCmd() string {
	switch m.mode {
	case modeTemplateName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name != "" {
			return "zmux new -t " + m.selectedTemplate + " " + name
		}
		return "zmux new -t " + m.selectedTemplate
	case modeTemplateSelect:
		if m.templateCursor < len(m.templates) {
			return "zmux new -t " + m.templates[m.templateCursor].Name
		}
		return "zmux new -t ..."
	case modeConfirmDelete, modeConfirmDeleteAttached:
		if m.confirm != nil {
			return "zmux kill " + m.confirm.name
		}
		return "zmux kill ..."
	}

	row := m.tree.CurrentSelectable()
	if row == nil {
		return "zmux new"
	}

	switch row.Kind {
	case outline.RowTopAction:
		wsQuery := strings.TrimSpace(m.state.workspaceQuery)
		sessQuery := strings.TrimSpace(m.state.sessionQuery)
		if wsQuery != "" && sessQuery != "" {
			return "zmux new " + wsQuery + " " + sessQuery
		}
		if wsQuery != "" {
			return "zmux new " + wsQuery
		}
		return "zmux new  # tmp-N session"
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		if ws == nil {
			return "zmux"
		}
		if ws.IsPseudo {
			return "# " + ws.Name
		}
		// Session query present → "zmux new <ws> <session>".
		if m.state.sessionQuery != "" {
			return "zmux new " + ws.Name + " " + m.state.sessionQuery
		}
		if len(ws.LiveSessions) == 0 {
			return "zmux new " + ws.Name
		}
		return "zmux " + ws.Name + "  # choose session"
	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return "zmux"
		}
		wsName := parentWorkspaceName(row, m.tree)
		if s.Attached {
			if wsName != "" {
				return "zmux " + wsName + " " + s.Name + "  →  " + s.Name + "-b"
			}
			return "zmux " + s.Name + "  →  " + s.Name + "-b"
		}
		if wsName != "" {
			return "zmux " + wsName + " " + s.Name
		}
		return "zmux " + s.Name
	case outline.RowExternalEntry:
		return "# " + row.Label
	case outline.RowExternalGroup:
		return "# " + row.Label
	}
	return "zmux"
}
