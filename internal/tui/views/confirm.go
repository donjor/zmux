package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmYesMsg signals that the user confirmed the action.
type ConfirmYesMsg struct{}

// ConfirmNoMsg signals that the user cancelled the action.
type ConfirmNoMsg struct{}

// ConfirmStyles holds the lipgloss styles needed by the confirm model.
type ConfirmStyles struct {
	Error  lipgloss.Style
	Normal lipgloss.Style
}

// ConfirmModel is a Y/N confirmation dialog for destructive actions.
type ConfirmModel struct {
	prompt string
	styles ConfirmStyles
	Active bool
}

// NewConfirmModel creates a new ConfirmModel.
func NewConfirmModel(styles ConfirmStyles) ConfirmModel {
	return ConfirmModel{
		styles: styles,
	}
}

// Show activates the confirmation dialog with the given prompt.
func (m *ConfirmModel) Show(prompt string) {
	m.prompt = prompt
	m.Active = true
}

// Hide deactivates the confirmation dialog.
func (m *ConfirmModel) Hide() {
	m.Active = false
	m.prompt = ""
}

// Update handles messages for the confirm model.
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.Hide()
			return m, func() tea.Msg { return ConfirmYesMsg{} }
		default:
			// Any other key cancels.
			m.Hide()
			return m, func() tea.Msg { return ConfirmNoMsg{} }
		}
	}

	return m, nil
}

// View renders the confirmation dialog.
func (m ConfirmModel) View() string {
	if !m.Active {
		return ""
	}
	return m.styles.Error.Render(m.prompt + " (y/N)")
}
