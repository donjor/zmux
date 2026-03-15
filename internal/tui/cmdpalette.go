package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// PaletteAction defines an action available in the command palette.
type PaletteAction struct {
	Name    string
	Hotkey  string
	Handler func() tea.Cmd
}

// paletteActionMsg wraps a PaletteAction that was chosen for execution.
type paletteActionMsg struct {
	action PaletteAction
}

// PaletteModel is the bubbletea model for the spotlight-style command palette.
type PaletteModel struct {
	actions  []PaletteAction
	filtered []PaletteAction
	cursor   int
	filter   textinput.Model
	styles   Styles
	width    int
	height   int

	// Result state.
	Chosen   *PaletteAction
	Quitting bool
}

// NewPaletteModel creates a new command palette model.
func NewPaletteModel(actions []PaletteAction, styles Styles) PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "type to search..."
	ti.CharLimit = 64
	ti.Focus()

	return PaletteModel{
		actions:  actions,
		filtered: actions,
		filter:   ti,
		styles:   styles,
	}
}

// Init starts the text input cursor blinking.
func (m PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and user input.
func (m PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to text input.
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m PaletteModel) handleKey(msg tea.KeyMsg) (PaletteModel, tea.Cmd) {
	switch {
	case key.Matches(msg, Keys.Back):
		m.Quitting = true
		return m, nil

	case key.Matches(msg, Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, Keys.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, Keys.Enter):
		if m.cursor < len(m.filtered) {
			chosen := m.filtered[m.cursor]
			m.Chosen = &chosen
			if chosen.Handler != nil {
				return m, chosen.Handler()
			}
		}
		return m, nil

	default:
		// Forward to text input for typing.
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

func (m *PaletteModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.actions
	} else {
		names := make([]string, len(m.actions))
		for i, a := range m.actions {
			names[i] = a.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]PaletteAction, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.actions[match.Index]
		}
	}

	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// Reset clears the filter and resets the cursor.
func (m *PaletteModel) Reset() {
	m.filter.SetValue("")
	m.filter.Focus()
	m.Chosen = nil
	m.Quitting = false
	m.applyFilter()
	m.cursor = 0
}

// View renders the command palette UI.
func (m PaletteModel) View() string {
	var b strings.Builder

	// Title.
	title := m.styles.Title.Render("zmux")
	subtitle := m.styles.Muted.Render(" command palette")
	b.WriteString(title + subtitle + "\n\n")

	// Search input.
	b.WriteString("  " + m.filter.View() + "\n\n")

	// Action list.
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.Muted.Render("  No matching actions.") + "\n")
	} else {
		for i, a := range m.filtered {
			b.WriteString(m.renderAction(i, a))
		}
	}

	// Help bar.
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("  enter:execute  esc:dismiss  tab:dashboard"))

	return b.String()
}

func (m PaletteModel) renderAction(idx int, a PaletteAction) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = m.styles.Accent.Render("> ")
	}

	nameStyle := m.styles.Normal
	if idx == m.cursor {
		nameStyle = m.styles.Selected
	}
	name := nameStyle.Render(a.Name)

	hotkey := ""
	if a.Hotkey != "" {
		hotkey = m.styles.Dim.Render("  " + a.Hotkey)
	}

	return cursor + name + hotkey + "\n"
}
