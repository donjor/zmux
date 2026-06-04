package picker

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// handleKey routes a key event through the active modal state.
func (m PickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Confirm delete (first step) ──
	if m.mode == modeConfirmDelete {
		switch msg.String() {
		case "y", "Y", "ctrl+x":
			// y or pressing ctrl+x again confirms. If we're killing a
			// workspace with a live attached client, route through the
			// second-step confirm before running the mutation — deleting an
			// attached workspace from outside tmux silently kills the user's
			// only client, so we require a second explicit confirm.
			if m.confirm != nil && m.confirm.kind == "workspace" && m.confirm.attached {
				m.mode = modeConfirmDeleteAttached
				return m, nil
			}
			m.applyConfirmedDelete()
			m.mode = modeNormal
			m.confirm = nil
			return m, m.reloadWorkspaces()
		default:
			m.mode = modeNormal
			m.confirm = nil
			return m, nil
		}
	}

	// ── Confirm delete (second step — attached workspace) ──
	if m.mode == modeConfirmDeleteAttached {
		switch msg.String() {
		case "y", "Y", "ctrl+x":
			m.applyConfirmedDelete()
			m.mode = modeNormal
			m.confirm = nil
			return m, m.reloadWorkspaces()
		default:
			m.mode = modeNormal
			m.confirm = nil
			return m, nil
		}
	}

	// ── Template select mode ──
	if m.mode == modeTemplateSelect {
		switch {
		case key.Matches(msg, Keys.Back), key.Matches(msg, Keys.Quit):
			m.mode = modeNormal
			m.input.Focus()
			return m, textinput.Blink
		case key.Matches(msg, Keys.Up):
			if m.templateCursor > 0 {
				m.templateCursor--
			}
			return m, nil
		case key.Matches(msg, Keys.Down):
			if m.templateCursor < len(m.templates)-1 {
				m.templateCursor++
			}
			return m, nil
		case key.Matches(msg, Keys.Enter):
			if m.templateCursor < len(m.templates) {
				tmpl := m.templates[m.templateCursor]
				m.selectedTemplate = tmpl.Name
				m.mode = modeTemplateName
				m.nameInput.SetValue(tmpl.Name)
				m.nameInput.Placeholder = "blank for " + tmpl.Name
				m.nameInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
		return m, nil
	}

	// ── Template name input mode ──
	if m.mode == modeTemplateName {
		switch {
		case key.Matches(msg, Keys.Back):
			m.mode = modeTemplateSelect
			m.nameInput.Blur()
			m.nameInput.SetValue("")
			return m, nil
		case key.Matches(msg, Keys.Enter):
			name := strings.TrimSpace(m.nameInput.Value())
			// Scope the template to the workspace the cursor is currently on.
			wsName := ""
			if row := m.tree.CurrentSelectable(); row != nil && row.Kind == outline.RowWorkspaceHeader {
				if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](row); ok && ws != nil && !ws.IsPseudo {
					wsName = ws.Name
				}
			}
			m.Result = PickerResult{
				Action:    "template",
				Name:      name,
				Template:  m.selectedTemplate,
				Workspace: wsName,
			}
			m.Quitting = true
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
	}

	// ── Normal mode ──
	return m.handleNormalKey(msg)
}

// handleNormalKey handles keys in the normal flat-list mode.
func (m PickerModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Quitting = true
		return m, tea.Quit

	case "up":
		m.tree.MoveUp()
		m.buildOutline()
		return m, nil

	case "down":
		m.tree.MoveDown()
		m.buildOutline()
		return m, nil

	case "enter":
		return m.handleEnter()

	case "tab":
		// Let bubbles textinput accept its suggestion. We forward the tab
		// key so AcceptSuggestion fires.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.onInputChanged()
		return m, cmd

	case "ctrl+t":
		if len(m.templates) == 0 {
			return m, nil
		}
		m.mode = modeTemplateSelect
		m.templateCursor = 0
		m.input.Blur()
		return m, nil

	case "ctrl+h":
		// Toggle show-all: reveal every workspace past the default cap.
		m.state.showAll = !m.state.showAll
		m.applyFilter()
		return m, nil

	case "ctrl+x":
		// Delete the current row (workspace or session). Snapshot the
		// target so the two-step confirm flow (for attached workspaces)
		// operates on a stable reference; the actual kill runs in
		// handleKey after y/N.
		row := m.tree.CurrentSelectable()
		if row == nil {
			return m, nil
		}
		switch row.Kind {
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](row); ok && ws != nil && !ws.IsPseudo {
				m.confirm = &pickerConfirmTarget{
					kind:      "workspace",
					name:      ws.Name,
					attached:  ws.HasAttached,
					liveCount: len(ws.LiveSessions),
				}
				// Empty workspace (nothing live to kill) — delete outright,
				// no confirmation step.
				if len(ws.LiveSessions) == 0 {
					m.applyConfirmedDelete()
					m.confirm = nil
					return m, m.reloadWorkspaces()
				}
				m.mode = modeConfirmDelete
			}
		case outline.RowSession:
			if s, ok := outline.RowData[session.SessionInfo](row); ok && s != nil {
				m.confirm = &pickerConfirmTarget{
					kind: "session",
					name: s.Name,
				}
				m.mode = modeConfirmDelete
			}
		}
		return m, nil

	case "esc":
		if m.state.workspaceQuery != "" || m.state.sessionQuery != "" {
			m.input.SetValue("")
			m.onInputChanged()
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick-select named session by 1-based index across the whole list.
		if m.state.workspaceQuery == "" && m.state.sessionQuery == "" {
			idx := int(msg.String()[0] - '0')
			count := 0
			for i := range m.tree.Rows {
				r := &m.tree.Rows[i]
				if r.Kind != outline.RowSession {
					continue
				}
				s, ok := outline.RowData[session.SessionInfo](r)
				if !ok || s == nil || s.IsTmp {
					continue
				}
				count++
				if count == idx {
					m.Result = PickerResult{Action: "attach", Session: s.Name}
					m.Quitting = true
					return m, tea.Quit
				}
			}
			return m, nil
		}
	}

	// All other keys go to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.onInputChanged()
	return m, cmd
}
