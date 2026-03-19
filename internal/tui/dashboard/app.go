package dashboard

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tmux"
)

// Services bundles the dependencies tabs need.
type Services struct {
	Runner   tmux.Runner
	FS       config.FS
	Styles   tui.Styles
}

// DashboardApp is the top-level bubbletea model for the tabbed dashboard.
// It owns the tab bar, chrome rendering, and message routing.
// All domain logic lives in individual tabs.
type DashboardApp struct {
	tabs      map[TabID]Tab
	tabOrder  []TabID
	activeTab TabID
	services  Services

	width  int
	height int
	rect   ContentRect

	// Status flash.
	statusText    string
	statusIsError bool

	// Result state for the caller.
	Action   string
	Chosen   string
	Quitting bool
}

// NewDashboardApp creates a new dashboard with the given tabs and initial tab.
func NewDashboardApp(services Services, tabImpls []Tab, initialTab TabID) *DashboardApp {
	tabs := make(map[TabID]Tab, len(tabImpls))
	order := make([]TabID, 0, len(tabImpls))
	for _, t := range tabImpls {
		tabs[t.ID()] = t
		order = append(order, t.ID())
	}

	if _, ok := tabs[initialTab]; !ok && len(order) > 0 {
		initialTab = order[0]
	}

	return &DashboardApp{
		tabs:      tabs,
		tabOrder:  order,
		activeTab: initialTab,
		services:  services,
	}
}

// Init initializes all tabs and activates the initial one.
func (m *DashboardApp) Init() tea.Cmd {
	var cmds []tea.Cmd

	for _, t := range m.tabs {
		if cmd := t.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if tab, ok := m.tabs[m.activeTab]; ok {
		if cmd := tab.Activate(ActivateInit); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// Update routes messages per the contract:
// 1. Global messages (resize, tab switch keys)
// 2. TargetedMsg -> specific tab
// 3. AppIntentMsg -> handle at parent
// 4. Everything else -> active tab
func (m *DashboardApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rect = ComputeContentRect(m.width, m.height)
		for id, tab := range m.tabs {
			tab.Resize(m.rect.Width, m.rect.Height)
			m.tabs[id] = tab
		}
		return m, nil

	case tea.KeyMsg:
		// Global key handling (always processed).
		if cmd := m.handleGlobalKey(msg); cmd != nil {
			return m, cmd
		}
		// Tab switch keys take priority over tab-specific handling.
		if handled, cmd := m.handleTabSwitchKey(msg); handled {
			return m, cmd
		}
	}

	// Route TargetedMsg to the specific tab.
	if targeted, ok := msg.(TargetedMsg); ok {
		targetID := targeted.TargetTab()
		if tab, exists := m.tabs[targetID]; exists {
			updated, cmd := tab.Update(msg)
			m.tabs[targetID] = updated
			return m, cmd
		}
		return m, nil
	}

	// Handle AppIntentMsg at parent level.
	if intent, ok := msg.(AppIntentMsg); ok {
		return m.handleIntent(intent)
	}

	// Forward everything else to active tab.
	if tab, ok := m.tabs[m.activeTab]; ok {
		updated, cmd := tab.Update(msg)
		m.tabs[m.activeTab] = updated
		return m, cmd
	}

	return m, nil
}

// View renders chrome + active tab content.
func (m *DashboardApp) View() string {
	if m.Quitting {
		return ""
	}

	if IsTooSmall(m.width, m.height) {
		return RenderTooSmall(m.services.Styles)
	}

	var b strings.Builder

	// Tab bar.
	tabBar := RenderTabBar(m.tabOrder, m.activeTab, m.services.Styles, m.width)

	// Status flash (appended to tab bar line).
	if m.statusText != "" {
		tabBar += RenderStatusFlash(m.statusText, m.statusIsError, m.services.Styles)
	}

	b.WriteString(tabBar)
	b.WriteString("\n")

	// Active tab content.
	if tab, ok := m.tabs[m.activeTab]; ok {
		content := tab.View()

		// Pad the content area.
		padded := lipgloss.NewStyle().
			Padding(0, 2).
			Render(content)

		b.WriteString(padded)
	}

	// Help bar at the bottom.
	b.WriteString("\n")
	tabHelp := ""
	if tab, ok := m.tabs[m.activeTab]; ok {
		tabHelp = tab.ShortHelp()
	}
	helpBar := RenderHelpBar(tabHelp, m.services.Styles, m.width)
	padHelp := lipgloss.NewStyle().
		Padding(0, 2).
		Render(helpBar)
	b.WriteString(padHelp)

	return b.String()
}

// handleGlobalKey handles keys that always work regardless of active tab.
func (m *DashboardApp) handleGlobalKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
		m.Quitting = true
		return tea.Quit
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.Quitting = true
		return tea.Quit
	}
	return nil
}

// handleTabSwitchKey handles tab navigation keys.
// Returns (true, cmd) if the key was a tab switch, (false, nil) otherwise.
func (m *DashboardApp) handleTabSwitchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "1":
		return true, m.switchToIndex(0)
	case "2":
		return true, m.switchToIndex(1)
	case "3":
		return true, m.switchToIndex(2)
	case "4":
		return true, m.switchToIndex(3)
	case "tab":
		return true, m.cycleTab(1)
	case "shift+tab":
		return true, m.cycleTab(-1)
	}
	return false, nil
}

func (m *DashboardApp) switchToIndex(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.tabOrder) {
		return nil
	}
	return m.switchTab(m.tabOrder[idx])
}

func (m *DashboardApp) cycleTab(delta int) tea.Cmd {
	idx := m.activeIndex()
	if idx < 0 {
		return nil
	}
	next := (idx + delta + len(m.tabOrder)) % len(m.tabOrder)
	return m.switchTab(m.tabOrder[next])
}

func (m *DashboardApp) switchTab(target TabID) tea.Cmd {
	if target == m.activeTab {
		return nil
	}

	// Deactivate current tab.
	if tab, ok := m.tabs[m.activeTab]; ok {
		tab.Deactivate()
		m.tabs[m.activeTab] = tab
	}

	m.activeTab = target
	m.statusText = "" // Clear status on tab switch.

	// Activate new tab.
	if tab, ok := m.tabs[m.activeTab]; ok {
		cmd := tab.Activate(ActivateTabSwitch)
		return cmd
	}
	return nil
}

func (m *DashboardApp) activeIndex() int {
	for i, id := range m.tabOrder {
		if id == m.activeTab {
			return i
		}
	}
	return -1
}

func (m *DashboardApp) handleIntent(intent AppIntentMsg) (tea.Model, tea.Cmd) {
	switch it := intent.(type) {
	case SwitchTabIntent:
		return m, m.switchTab(it.Tab)

	case SetStatusIntent:
		m.statusText = it.Text
		m.statusIsError = it.IsError
		return m, nil

	case QuitIntent:
		m.Action = it.Action
		m.Chosen = it.Chosen
		m.Quitting = true
		return m, tea.Quit
	}

	return m, nil
}
