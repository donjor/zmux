package palette

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/sahilm/fuzzy"
)

// PaletteModel is the bubbletea model for the spotlight-style command palette.
type PaletteModel struct {
	registry *Registry
	actions  []Action
	filtered []Action
	cursor   int
	filter   textinput.Model
	styles   styles.Styles
	width    int
	height   int

	// Result state (read after the program exits).
	Chosen   *Action
	Quitting bool
}

// NewPaletteModel creates a new command palette model.
func NewPaletteModel(registry *Registry, styles styles.Styles) *PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "type to search actions..."
	ti.CharLimit = 128
	ti.Focus()

	actions := registry.All()

	return &PaletteModel{
		registry: registry,
		actions:  actions,
		filtered: actions,
		filter:   ti,
		styles:   styles,
	}
}

// Init starts the text input cursor blinking.
func (m *PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and user input.
func (m *PaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *PaletteModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "ctrl+k"))):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "ctrl+j"))):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if m.cursor < len(m.filtered) {
			chosen := m.filtered[m.cursor]
			m.Chosen = &chosen
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Forward to text input for typing.
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *PaletteModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.actions
	} else {
		// Build searchable text for fuzzy matching.
		texts := make([]string, len(m.actions))
		for i, a := range m.actions {
			texts[i] = a.searchText()
		}
		matches := fuzzy.Find(query, texts)
		m.filtered = make([]Action, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.actions[match.Index]
		}
	}

	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// View renders the command palette UI.
func (m *PaletteModel) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m *PaletteModel) view() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Title bar.
	title := m.styles.Accent.Bold(true).Render("zmux")
	subtitle := m.styles.Dim.Render(" command palette")
	b.WriteString("\n  " + title + subtitle + "\n\n")

	// Search input.
	prompt := m.styles.Accent.Render("  > ")
	b.WriteString(prompt + m.filter.View() + "\n")
	b.WriteString("\n")

	// Action list.
	if len(m.filtered) == 0 {
		b.WriteString("  " + m.styles.Dim.Render("No matching actions.") + "\n")
	} else {
		// Scrolling window.
		maxVisible := m.height - 10
		if maxVisible < 5 {
			maxVisible = 15
		}

		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		if start > 0 {
			b.WriteString("  " + m.styles.Dim.Render(strings.Repeat("↑", 1)+" more") + "\n")
		}

		lastGroup := ""
		for i := start; i < end; i++ {
			a := m.filtered[i]

			// Group separator.
			if a.Group != lastGroup {
				if lastGroup != "" {
					b.WriteString("\n")
				}
				b.WriteString("  " + m.styles.Dim.Render(a.Group) + "\n")
				lastGroup = a.Group
			}

			b.WriteString(m.renderAction(i, a))
		}

		if end < len(m.filtered) {
			b.WriteString("\n  " + m.styles.Dim.Render(strings.Repeat("↓", 1)+" more") + "\n")
		}
	}

	// Help bar.
	b.WriteString("\n")
	b.WriteString("  " + m.styles.Dim.Render("enter:execute  esc:close  up/down:navigate"))
	b.WriteString("\n")

	return b.String()
}

func (m *PaletteModel) renderAction(idx int, a Action) string {
	selected := idx == m.cursor

	// Cursor indicator.
	cursor := "  "
	if selected {
		cursor = m.styles.Accent.Render("▸ ")
	}

	// Title.
	titleStyle := m.styles.Normal
	if selected {
		titleStyle = m.styles.Accent.Bold(true)
	}
	title := titleStyle.Render(a.Title)

	// Subtitle.
	subtitle := ""
	if a.Subtitle != "" {
		subtitle = "  " + m.styles.Dim.Render(a.Subtitle)
	}

	// Hint (right-aligned).
	hint := ""
	if a.Hint != "" {
		hint = "  " + m.styles.Dim.Render(a.Hint)
	}

	return "    " + cursor + title + subtitle + hint + "\n"
}
