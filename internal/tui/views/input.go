package views

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputDoneMsg signals that the user has confirmed text input.
type InputDoneMsg struct {
	Value string
}

// InputCancelMsg signals that the user has cancelled text input.
type InputCancelMsg struct{}

// InputStyles holds the lipgloss styles needed by the input model.
type InputStyles struct {
	Title  lipgloss.Style
	Normal lipgloss.Style
}

// InputModel is a reusable text input sub-view for rename operations and similar.
type InputModel struct {
	input  textinput.Model
	prompt string
	styles InputStyles
	Active bool
}

// NewInputModel creates a new InputModel with the given prompt.
func NewInputModel(prompt string, styles InputStyles) InputModel {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 128
	return InputModel{
		input:  ti,
		prompt: prompt,
		styles: styles,
	}
}

// Focus activates the input with an optional initial value.
func (m *InputModel) Focus(initialValue string) tea.Cmd {
	m.Active = true
	m.input.SetValue(initialValue)
	m.input.Focus()
	return textinput.Blink
}

// Blur deactivates the input.
func (m *InputModel) Blur() {
	m.Active = false
	m.input.Blur()
	m.input.SetValue("")
}

// Value returns the current input value.
func (m InputModel) Value() string {
	return m.input.Value()
}

// Update handles messages for the input model.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := m.input.Value()
			m.Blur()
			return m, func() tea.Msg { return InputDoneMsg{Value: val} }
		case tea.KeyEscape:
			m.Blur()
			return m, func() tea.Msg { return InputCancelMsg{} }
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the input.
func (m InputModel) View() string {
	if !m.Active {
		return ""
	}
	return m.styles.Title.Render(m.prompt) + " " + m.input.View()
}
