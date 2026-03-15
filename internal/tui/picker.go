package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
)

// pickerMode tracks the current mode of the picker.
type pickerMode int

const (
	modeList pickerMode = iota
	modeFilter
	modeConfirmDelete
)

// attachMsg signals that the user wants to attach to a session.
type attachMsg struct{ name string }

// createMsg signals that the user wants to create a new session.
type createMsg struct{}

// templateMsg signals that the user wants to create from a template.
type templateMsg struct{}

// PickerModel is the bubbletea model for the outside-tmux session picker.
type PickerModel struct {
	runner    tmux.Runner
	sessions  []session.SessionInfo
	filtered  []session.SessionInfo
	cursor    int
	width     int
	height    int
	styles    Styles
	mode      pickerMode
	filter    textinput.Model
	err       error

	// Result state (read after quit).
	Chosen    string // session name to attach
	Action    string // "attach", "new", "template", or ""
	Quitting  bool
}

// NewPickerModel creates a new session picker model.
func NewPickerModel(runner tmux.Runner, styles Styles) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "filter sessions..."
	ti.CharLimit = 64

	return PickerModel{
		runner: runner,
		styles: styles,
		filter: ti,
	}
}

// refreshSessionsMsg carries refreshed session data.
type refreshSessionsMsg struct {
	sessions []session.SessionInfo
	err      error
}

func refreshSessions(runner tmux.Runner) tea.Cmd {
	return func() tea.Msg {
		sessions, err := session.ListSessions(runner)
		return refreshSessionsMsg{sessions: sessions, err: err}
	}
}

// Init loads sessions on startup.
func (m PickerModel) Init() tea.Cmd {
	return refreshSessions(m.runner)
}

// Update handles messages and user input.
func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case refreshSessionsMsg:
		m.sessions = msg.sessions
		m.err = msg.err
		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to text input if filtering.
	if m.mode == modeFilter {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilter()
		return m, cmd
	}

	return m, nil
}

func (m PickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirm-delete mode.
	if m.mode == modeConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			if m.cursor < len(m.filtered) {
				name := m.filtered[m.cursor].Name
				_ = session.Kill(m.runner, name)
			}
			m.mode = modeList
			return m, refreshSessions(m.runner)
		default:
			m.mode = modeList
			return m, nil
		}
	}

	// Handle filter mode.
	if m.mode == modeFilter {
		switch {
		case key.Matches(msg, Keys.Back):
			m.mode = modeList
			m.filter.SetValue("")
			m.filter.Blur()
			m.applyFilter()
			return m, nil
		case key.Matches(msg, Keys.Enter):
			m.mode = modeList
			m.filter.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}

	// Normal list mode.
	switch {
	case key.Matches(msg, Keys.Quit):
		m.Quitting = true
		return m, tea.Quit

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
			m.Chosen = m.filtered[m.cursor].Name
			m.Action = "attach"
			return m, tea.Quit
		}
		return m, nil

	case key.Matches(msg, Keys.New):
		m.Action = "new"
		return m, tea.Quit

	case key.Matches(msg, Keys.Template):
		m.Action = "template"
		return m, tea.Quit

	case key.Matches(msg, Keys.Delete):
		if m.cursor < len(m.filtered) {
			m.mode = modeConfirmDelete
		}
		return m, nil

	case key.Matches(msg, Keys.Filter):
		m.mode = modeFilter
		m.filter.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m *PickerModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.sessions
	} else {
		names := make([]string, len(m.sessions))
		for i, s := range m.sessions {
			names[i] = s.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]session.SessionInfo, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.sessions[match.Index]
		}
	}

	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// View renders the picker UI.
func (m PickerModel) View() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Header.
	title := m.styles.Title.Render("zmux")
	subtitle := m.styles.Muted.Render(" session picker")
	b.WriteString(title + subtitle + "\n\n")

	// Filter bar.
	if m.mode == modeFilter {
		b.WriteString(m.filter.View() + "\n\n")
	} else if m.filter.Value() != "" {
		b.WriteString(m.styles.Dim.Render("filter: "+m.filter.Value()) + "\n\n")
	}

	// Error message.
	if m.err != nil {
		b.WriteString(m.styles.Error.Render("Error: "+m.err.Error()) + "\n\n")
	}

	// Session list.
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.Muted.Render("  No sessions found.") + "\n")
	} else {
		for i, s := range m.filtered {
			b.WriteString(m.renderSessionEntry(i, s))
		}
	}

	// Delete confirmation.
	if m.mode == modeConfirmDelete && m.cursor < len(m.filtered) {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render(
			fmt.Sprintf("  Delete %q? (y/N)", m.filtered[m.cursor].Name),
		))
		b.WriteString("\n")
	}

	// Help bar.
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m PickerModel) renderSessionEntry(idx int, s session.SessionInfo) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = m.styles.Accent.Render("> ")
	}

	// Name.
	nameStyle := m.styles.Normal
	if idx == m.cursor {
		nameStyle = m.styles.Selected
	}
	if s.IsTmp {
		nameStyle = nameStyle.Foreground(m.styles.Dim.GetForeground())
	}
	name := nameStyle.Render(s.Name)

	// Metadata.
	meta := m.styles.Dim.Render(fmt.Sprintf(
		" %dw",
		s.Windows,
	))

	if !s.Activity.IsZero() {
		meta += m.styles.Dim.Render(" " + session.HumanAge(s.Activity))
	}

	// Dir (shortened).
	if s.Dir != "" {
		dir := shortenPath(s.Dir)
		meta += m.styles.Dim.Render(" " + dir)
	}

	// Attached indicator.
	attached := ""
	if s.Attached {
		attached = m.styles.Success.Render(" *")
	}

	return cursor + name + meta + attached + "\n"
}

func (m PickerModel) renderHelp() string {
	parts := []string{
		"enter:attach",
		"n:new",
		"t:template",
		"d:delete",
		"/:filter",
		"q:quit",
	}
	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}

// shortenPath replaces the home directory with ~ and truncates long paths.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	// If the path is still long, just show the last two components.
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}
