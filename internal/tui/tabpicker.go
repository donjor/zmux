package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/tmux"
)

// tabPickerMode tracks the current mode.
type tabPickerMode int

const (
	tpModeList tabPickerMode = iota
	tpModeNew                // text input for new tab name
	tpModeRename             // text input for rename
)

// TabPickerResult holds the outcome.
type TabPickerResult struct {
	Action string // "select", "new", "rename", "close", "swap", ""
	Index  int    // window index
	Name   string // tab name (for new/rename)
	Delta  int    // swap direction (-1 or +1)
}

type tabEntry struct {
	tmux.Window
	Command string // pane command
}

// TabPickerModel is a lightweight tab switcher for the current session.
type TabPickerModel struct {
	runner  tmux.Runner
	session string
	tabs    []tabEntry
	filtered []tabEntry
	cursor  int
	input   textinput.Model
	mode    tabPickerMode
	width   int
	height  int
	styles  Styles

	// rename target
	renameIdx int

	Result   TabPickerResult
	Quitting bool
}

// NewTabPickerModel creates a new tab picker.
func NewTabPickerModel(runner tmux.Runner, session string, styles Styles) TabPickerModel {
	ti := textinput.New()
	ti.Placeholder = "search tabs..."
	ti.CharLimit = 64
	ti.Focus()

	return TabPickerModel{
		runner:  runner,
		session: session,
		styles:  styles,
		input:   ti,
	}
}

type tabsLoadedMsg struct {
	tabs []tabEntry
}

func (m TabPickerModel) Init() tea.Cmd {
	return tea.Batch(m.loadTabs(), textinput.Blink)
}

func (m TabPickerModel) loadTabs() tea.Cmd {
	runner := m.runner
	session := m.session
	return func() tea.Msg {
		windows, _ := runner.ListWindows(session)
		panes, _ := runner.ListPanes(session)

		paneCmd := make(map[int]string)
		for _, p := range panes {
			if p.Active {
				paneCmd[p.WindowIndex] = p.Command
			}
		}

		entries := make([]tabEntry, len(windows))
		for i, w := range windows {
			entries[i] = tabEntry{Window: w, Command: paneCmd[w.Index]}
		}
		return tabsLoadedMsg{tabs: entries}
	}
}

func (m TabPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tabsLoadedMsg:
		m.tabs = msg.tabs
		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.mode == tpModeList {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd
	}
	return m, nil
}

func (m TabPickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// New tab name input
	if m.mode == tpModeNew {
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			m.Result = TabPickerResult{Action: "new", Name: name}
			m.Quitting = true
			return m, tea.Quit
		case "esc":
			m.mode = tpModeList
			m.input.SetValue("")
			m.input.Placeholder = "search tabs..."
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	// Rename input
	if m.mode == tpModeRename {
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			m.Result = TabPickerResult{Action: "rename", Index: m.renameIdx, Name: name}
			m.Quitting = true
			return m, tea.Quit
		case "esc":
			m.mode = tpModeList
			m.input.SetValue("")
			m.input.Placeholder = "search tabs..."
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	// List mode
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.input.Value() != "" {
			m.input.SetValue("")
			m.applyFilter()
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit

	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case "enter":
		if m.cursor < len(m.filtered) {
			m.Result = TabPickerResult{Action: "select", Index: m.filtered[m.cursor].Index}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+n":
		m.mode = tpModeNew
		m.input.SetValue("")
		m.input.Placeholder = "new tab name (blank for default)..."
		return m, nil

	case "ctrl+r":
		if m.cursor < len(m.filtered) {
			m.renameIdx = m.filtered[m.cursor].Index
			m.mode = tpModeRename
			m.input.SetValue(m.filtered[m.cursor].Name)
			m.input.Placeholder = "rename..."
			return m, nil
		}
		return m, nil

	case "ctrl+x":
		if m.cursor < len(m.filtered) {
			m.Result = TabPickerResult{Action: "close", Index: m.filtered[m.cursor].Index}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+left", "<":
		if m.cursor < len(m.filtered) && m.input.Value() == "" {
			m.Result = TabPickerResult{Action: "swap", Index: m.filtered[m.cursor].Index, Delta: -1}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+right", ">":
		if m.cursor < len(m.filtered) && m.input.Value() == "" {
			m.Result = TabPickerResult{Action: "swap", Index: m.filtered[m.cursor].Index, Delta: 1}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// All other keys to input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m *TabPickerModel) applyFilter() {
	query := m.input.Value()
	if query == "" {
		m.filtered = m.tabs
	} else {
		names := make([]string, len(m.tabs))
		for i, t := range m.tabs {
			names[i] = t.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]tabEntry, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.tabs[match.Index]
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m TabPickerModel) View() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString("  " + m.styles.Title.Bold(true).Render(m.session) + m.styles.Muted.Render(" tabs") + "\n\n")

	// Input
	if m.mode == tpModeNew {
		b.WriteString("  " + m.styles.Accent.Render("new ▸ ") + m.input.View() + "\n\n")
	} else if m.mode == tpModeRename {
		b.WriteString("  " + m.styles.Accent.Render("rename ▸ ") + m.input.View() + "\n\n")
	} else {
		b.WriteString("  " + m.styles.Accent.Render("▸ ") + m.input.View() + "\n\n")
	}

	// Tab list
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.Muted.Render("  no tabs") + "\n")
	} else {
		for i, t := range m.filtered {
			selected := i == m.cursor

			cursor := "  "
			if selected {
				cursor = m.styles.Accent.Render("▸ ")
			}

			active := " "
			if t.Active {
				active = m.styles.Success.Render("●")
			}

			idx := m.styles.Dim.Render(fmt.Sprintf("%d", t.Index))

			nameStyle := m.styles.Normal.Bold(true)
			if selected {
				nameStyle = m.styles.Accent.Bold(true)
			}
			name := nameStyle.Render(fmt.Sprintf("%-14s", t.Name))

			cmd := m.styles.Dim.Render(t.Command)

			b.WriteString(fmt.Sprintf("  %s%s %s %s  %s\n", cursor, active, idx, name, cmd))
		}
	}

	// Help
	b.WriteString("\n")
	switch m.mode {
	case tpModeNew:
		b.WriteString(m.styles.Help.Render("  enter:create  esc:cancel"))
	case tpModeRename:
		b.WriteString(m.styles.Help.Render("  enter:rename  esc:cancel"))
	default:
		parts := []string{"enter:go", "ctrl+n:new", "ctrl+r:rename", "ctrl+x:close", "</>:reorder", "esc:quit"}
		b.WriteString(m.styles.Help.Render("  " + strings.Join(parts, "  ")))
	}
	b.WriteString("\n")

	return b.String()
}

// SetKey is used to ignore key conflicts
func (m TabPickerModel) SetKey(_ key.Binding) {}
