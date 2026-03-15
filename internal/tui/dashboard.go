package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/views"
)

// DashboardMode tracks whether we are in palette or dashboard view.
type DashboardMode int

const (
	// ModePalette is the default mode showing the command palette.
	ModePalette DashboardMode = iota
	// ModeDashboard shows the full session dashboard.
	ModeDashboard
)

// dashboardSubMode tracks sub-views within the dashboard.
type dashboardSubMode int

const (
	dashSubNone dashboardSubMode = iota
	dashSubInput
	dashSubConfirm
)

// switchSessionMsg signals that the user wants to switch to a session.
type switchSessionMsg struct{ name string }

// dashActionMsg signals a specific dashboard action result.
type dashActionMsg struct{ action string }

// DashboardModel is the bubbletea model for the full dashboard TUI.
type DashboardModel struct {
	mode     DashboardMode
	subMode  dashboardSubMode
	palette  PaletteModel
	runner   tmux.Runner
	sessions []session.SessionInfo
	windows  []tmux.Window
	cursor   int
	width    int
	height   int
	styles   Styles
	input    views.InputModel
	confirm  views.ConfirmModel

	// Current session info.
	currentSession string
	currentDir     string

	// Pending action context for input/confirm.
	pendingAction string

	// Result state (read after quit).
	Action   string
	Chosen   string
	Quitting bool
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(runner tmux.Runner, styles Styles) DashboardModel {
	actions := buildDashboardActions()

	inputStyles := views.InputStyles{
		Title:  styles.Accent,
		Normal: styles.Normal,
	}
	confirmStyles := views.ConfirmStyles{
		Error:  styles.Error,
		Normal: styles.Normal,
	}

	return DashboardModel{
		mode:    ModePalette,
		palette: NewPaletteModel(actions, styles),
		runner:  runner,
		styles:  styles,
		input:   views.NewInputModel("", inputStyles),
		confirm: views.NewConfirmModel(confirmStyles),
	}
}

func buildDashboardActions() []PaletteAction {
	return []PaletteAction{
		{Name: "Switch Session", Hotkey: "enter"},
		{Name: "New Session", Hotkey: "n"},
		{Name: "New from Template", Hotkey: "t"},
		{Name: "Rename Session", Hotkey: "r"},
		{Name: "Rename Tab", Hotkey: "R"},
		{Name: "Kill Session", Hotkey: "d"},
		{Name: "Kill Tab", Hotkey: "D"},
		{Name: "Theme Browser", Hotkey: "T"},
		{Name: "Cleanup Tmp", Hotkey: "c"},
	}
}

// Init loads sessions and initializes the palette.
func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		refreshSessions(m.runner),
		m.loadCurrentSession(),
		m.palette.Init(),
	)
}

func (m DashboardModel) loadCurrentSession() tea.Cmd {
	return func() tea.Msg {
		name, _ := m.runner.DisplayMessage("", "#{session_name}")
		dir, _ := m.runner.DisplayMessage("", "#{session_path}")
		windows, _ := m.runner.ListWindows(name)
		return currentSessionMsg{name: name, dir: dir, windows: windows}
	}
}

type currentSessionMsg struct {
	name    string
	dir     string
	windows []tmux.Window
}

// Update handles messages and user input.
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case refreshSessionsMsg:
		m.sessions = msg.sessions
		m.clampCursor()
		return m, nil

	case currentSessionMsg:
		m.currentSession = msg.name
		m.currentDir = msg.dir
		m.windows = msg.windows
		return m, nil

	case switchSessionMsg:
		m.Chosen = msg.name
		m.Action = "switch"
		m.Quitting = true
		return m, tea.Quit

	case dashActionMsg:
		m.Action = msg.action
		m.Quitting = true
		return m, tea.Quit

	case views.InputDoneMsg:
		return m.handleInputDone(msg.Value)

	case views.InputCancelMsg:
		m.subMode = dashSubNone
		return m, nil

	case views.ConfirmYesMsg:
		return m.handleConfirmYes()

	case views.ConfirmNoMsg:
		m.subMode = dashSubNone
		m.pendingAction = ""
		return m, nil

	case paletteActionMsg:
		return m.executePaletteAction(msg.action)
	}

	// Route to sub-views.
	if m.subMode == dashSubInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	if m.subMode == dashSubConfirm {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to palette in palette mode.
	if m.mode == ModePalette {
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		if m.palette.Quitting {
			m.Quitting = true
			return m, tea.Quit
		}
		return m, cmd
	}

	return m, nil
}

func (m DashboardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tab toggles between palette and dashboard.
	if msg.String() == "tab" {
		if m.mode == ModePalette {
			m.mode = ModeDashboard
		} else {
			m.mode = ModePalette
			m.palette.Reset()
		}
		return m, nil
	}

	if m.mode == ModePalette {
		return m.handlePaletteKey(msg)
	}
	return m.handleDashboardKey(msg)
}

func (m DashboardModel) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.palette, cmd = m.palette.Update(msg)

	// Check if palette dismissed itself.
	if m.palette.Quitting {
		m.Quitting = true
		return m, tea.Quit
	}

	// Check if an action was chosen.
	if m.palette.Chosen != nil {
		action := *m.palette.Chosen
		m.palette.Reset()
		return m.executePaletteAction(action)
	}

	return m, cmd
}

func (m DashboardModel) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, Keys.Quit):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, Keys.Back):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, Keys.Down):
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, Keys.Enter):
		if m.cursor < len(m.sessions) {
			m.Chosen = m.sessions[m.cursor].Name
			m.Action = "switch"
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case key.Matches(msg, Keys.New):
		m.Action = "new"
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, Keys.Template):
		m.Action = "template"
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, Keys.Delete):
		if m.cursor < len(m.sessions) {
			m.pendingAction = "kill"
			m.subMode = dashSubConfirm
			m.confirm.Show(fmt.Sprintf("  Kill session %q?", m.sessions[m.cursor].Name))
		}
		return m, nil

	case key.Matches(msg, Keys.Rename):
		if m.cursor < len(m.sessions) {
			m.pendingAction = "rename"
			m.subMode = dashSubInput
			return m, m.input.Focus(m.sessions[m.cursor].Name)
		}
		return m, nil

	case key.Matches(msg, Keys.Cleanup):
		_, _ = session.CleanupTmp(m.runner)
		return m, refreshSessions(m.runner)
	}

	return m, nil
}

func (m DashboardModel) executePaletteAction(action PaletteAction) (tea.Model, tea.Cmd) {
	switch action.Name {
	case "Switch Session":
		// Switch to dashboard mode to pick a session.
		m.mode = ModeDashboard
		return m, nil

	case "New Session":
		m.Action = "new"
		m.Quitting = true
		return m, tea.Quit

	case "New from Template":
		m.Action = "template"
		m.Quitting = true
		return m, tea.Quit

	case "Rename Session":
		m.mode = ModeDashboard
		m.pendingAction = "rename"
		m.subMode = dashSubInput
		initialName := m.currentSession
		if m.cursor < len(m.sessions) {
			initialName = m.sessions[m.cursor].Name
		}
		return m, m.input.Focus(initialName)

	case "Rename Tab":
		m.pendingAction = "rename-tab"
		m.subMode = dashSubInput
		initialName := ""
		if len(m.windows) > 0 {
			for _, w := range m.windows {
				if w.Active {
					initialName = w.Name
					break
				}
			}
		}
		return m, m.input.Focus(initialName)

	case "Kill Session":
		m.mode = ModeDashboard
		if m.cursor < len(m.sessions) {
			m.pendingAction = "kill"
			m.subMode = dashSubConfirm
			m.confirm.Show(fmt.Sprintf("  Kill session %q?", m.sessions[m.cursor].Name))
		}
		return m, nil

	case "Kill Tab":
		m.Action = "kill-tab"
		m.Quitting = true
		return m, tea.Quit

	case "Theme Browser":
		m.Action = "theme"
		m.Quitting = true
		return m, tea.Quit

	case "Cleanup Tmp":
		_, _ = session.CleanupTmp(m.runner)
		return m, refreshSessions(m.runner)
	}

	return m, nil
}

func (m DashboardModel) handleInputDone(value string) (tea.Model, tea.Cmd) {
	m.subMode = dashSubNone

	switch m.pendingAction {
	case "rename":
		if m.cursor < len(m.sessions) && value != "" {
			old := m.sessions[m.cursor].Name
			_ = session.Rename(m.runner, old, value)
		}
		m.pendingAction = ""
		return m, refreshSessions(m.runner)

	case "rename-tab":
		if value != "" && m.currentSession != "" {
			// Find the active window name.
			oldName := ""
			for _, w := range m.windows {
				if w.Active {
					oldName = w.Name
					break
				}
			}
			if oldName != "" {
				_ = m.runner.RenameWindow(m.currentSession, oldName, value)
			}
		}
		m.pendingAction = ""
		return m, m.loadCurrentSession()
	}

	m.pendingAction = ""
	return m, nil
}

func (m DashboardModel) handleConfirmYes() (tea.Model, tea.Cmd) {
	m.subMode = dashSubNone

	switch m.pendingAction {
	case "kill":
		if m.cursor < len(m.sessions) {
			name := m.sessions[m.cursor].Name
			_ = session.Kill(m.runner, name)
		}
		m.pendingAction = ""
		return m, refreshSessions(m.runner)
	}

	m.pendingAction = ""
	return m, nil
}

func (m *DashboardModel) clampCursor() {
	if m.cursor >= len(m.sessions) {
		m.cursor = max(0, len(m.sessions)-1)
	}
}

// View renders the dashboard UI.
func (m DashboardModel) View() string {
	if m.Quitting {
		return ""
	}

	if m.mode == ModePalette {
		return m.palette.View()
	}

	return m.renderDashboard()
}

func (m DashboardModel) renderDashboard() string {
	var b strings.Builder

	// Header.
	headerStyles := views.HeaderStyles{
		Accent:   m.styles.Accent,
		Normal:   m.styles.Normal,
		Muted:    m.styles.Muted,
		Dim:      m.styles.Dim,
		Title:    m.styles.Title,
		Selected: m.styles.Selected,
	}
	header := views.RenderHeader(m.currentSession, m.currentDir, m.windows, headerStyles, m.width)
	b.WriteString(header)
	b.WriteString("\n")

	// Sub-view overlays.
	if m.subMode == dashSubInput {
		b.WriteString("  " + m.input.View() + "\n\n")
	}
	if m.subMode == dashSubConfirm {
		b.WriteString("  " + m.confirm.View() + "\n\n")
	}

	// Session list title.
	b.WriteString(m.styles.Title.Render("Sessions") + "\n\n")

	// Session list.
	listStyles := views.SessionListStyles{
		Normal:   m.styles.Normal,
		Selected: m.styles.Selected,
		Accent:   m.styles.Accent,
		Dim:      m.styles.Dim,
		Muted:    m.styles.Muted,
		Success:  m.styles.Success,
	}
	listHeight := m.height - 12 // Reserve space for header, actions, etc.
	if listHeight < 5 {
		listHeight = 10
	}
	b.WriteString(views.RenderSessionList(m.sessions, m.cursor, listStyles, m.width, listHeight))

	// Action hints.
	b.WriteString("\n")
	actionHints := []views.ActionHint{
		{Key: "enter", Name: "switch"},
		{Key: "n", Name: "new"},
		{Key: "t", Name: "template"},
		{Key: "r", Name: "rename"},
		{Key: "d", Name: "kill"},
		{Key: "c", Name: "cleanup"},
		{Key: "tab", Name: "palette"},
		{Key: "esc", Name: "quit"},
	}
	actionStyles := views.ActionBarStyles{Help: m.styles.Help}
	b.WriteString(views.RenderActions(actionHints, actionStyles, m.width))

	return b.String()
}
