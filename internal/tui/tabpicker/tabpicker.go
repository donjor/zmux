package tabpicker

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
)

// tabPickerMode tracks the current mode.
type tabPickerMode int

const (
	tpModeList   tabPickerMode = iota
	tpModeNew                  // text input for new tab name
	tpModeRename               // text input for rename
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

// tabRowID builds the stable outline row ID for a tab. Keyed by name
// so the cursor tracks the same logical tab across rename / refetch.
func tabRowID(name string) string { return "tab:" + name }

// TabPickerModel is a lightweight tab switcher for the current session.
// Cursor and selectable navigation run through the shared outline.Tree
// so the cursor lands on the same tab by name across refetches,
// rename, and filter narrowing.
type TabPickerModel struct {
	runner   tmux.Runner
	session  string
	tabs     []tabEntry
	filtered []tabEntry
	tree     *outline.Tree
	input    textinput.Model
	mode     tabPickerMode
	width    int
	height   int
	styles   styles.Styles

	// rename target
	renameIdx int

	Result   TabPickerResult
	Quitting bool
}

// NewTabPickerModel creates a new tab picker.
func NewTabPickerModel(runner tmux.Runner, session string, styles styles.Styles) TabPickerModel {
	ti := textinput.New()
	ti.Placeholder = "search tabs..."
	ti.CharLimit = 64
	ti.Focus()

	return TabPickerModel{
		runner:  runner,
		session: session,
		styles:  styles,
		input:   ti,
		tree:    outline.NewTree(),
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

// buildRows rebuilds the outline rows from the filtered tab entries.
// Stable IDs are keyed by tab name so rename operations keep the
// cursor tracking the same logical tab.
func (m *TabPickerModel) buildRows() []outline.Row {
	rows := make([]outline.Row, len(m.filtered))
	for i := range m.filtered {
		t := m.filtered[i]
		rows[i] = outline.Row{
			ID:         tabRowID(t.Name),
			Kind:       outline.RowWindow,
			Label:      t.Name,
			Selectable: true,
			Data:       &m.filtered[i],
		}
	}
	return rows
}

// currentTab returns the tab currently under the cursor, or nil.
func (m TabPickerModel) currentTab() *tabEntry {
	row := m.tree.CurrentSelectable()
	if row == nil {
		return nil
	}
	te, _ := outline.RowData[tabEntry](row)
	return te
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
		m.tree.MoveUp()
		return m, nil

	case "down":
		m.tree.MoveDown()
		return m, nil

	case "enter":
		if t := m.currentTab(); t != nil {
			m.Result = TabPickerResult{Action: "select", Index: t.Index}
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
		if t := m.currentTab(); t != nil {
			m.renameIdx = t.Index
			m.mode = tpModeRename
			m.input.SetValue(t.Name)
			m.input.Placeholder = "rename..."
		}
		return m, nil

	case "ctrl+x":
		if t := m.currentTab(); t != nil {
			m.Result = TabPickerResult{Action: "close", Index: t.Index}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+left", "<":
		if m.input.Value() == "" {
			if t := m.currentTab(); t != nil {
				m.Result = TabPickerResult{Action: "swap", Index: t.Index, Delta: -1}
				m.Quitting = true
				return m, tea.Quit
			}
		}
		return m, nil

	case "ctrl+right", ">":
		if m.input.Value() == "" {
			if t := m.currentTab(); t != nil {
				m.Result = TabPickerResult{Action: "swap", Index: t.Index, Delta: 1}
				m.Quitting = true
				return m, tea.Quit
			}
		}
		return m, nil
	}

	// All other keys to input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

// applyFilter rebuilds the filtered slice based on the current input
// value, then pushes the new rows into the tree (stable-ID restore).
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
	m.tree.SetRows(m.buildRows())
}

func (m TabPickerModel) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m TabPickerModel) view() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString("  " + m.styles.Title.Bold(true).Render(m.session) + m.styles.Muted.Render(" tabs") + "\n\n")

	// Input
	switch m.mode {
	case tpModeNew:
		b.WriteString("  " + m.styles.Accent.Render("new ▸ ") + m.input.View() + "\n\n")
	case tpModeRename:
		b.WriteString("  " + m.styles.Accent.Render("rename ▸ ") + m.input.View() + "\n\n")
	default:
		b.WriteString("  " + m.styles.Accent.Render("▸ ") + m.input.View() + "\n\n")
	}

	// Tab list
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.Muted.Render("  no tabs") + "\n")
	} else {
		for i, t := range m.filtered {
			selected := i == m.tree.Cursor

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

			fmt.Fprintf(&b, "  %s%s %s %s  %s\n", cursor, active, idx, name, cmd)
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
