package dashboard

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/styles"
)

// Services bundles the dependencies tabs need.
type Services struct {
	Runner   tmux.Runner
	FS       config.FS
	Styles   styles.Styles
	Palette  *theme.Palette  // Theme palette (nil if unresolved).
	Resolver *theme.Resolver // Theme resolver for lookups.
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

	// Theme changes broadcast to ALL tabs + update services.
	if tc, ok := msg.(ThemeChangedMsg); ok {
		m.services.Styles = tc.Styles
		m.services.Palette = &tc.Palette
		var cmds []tea.Cmd
		for id, tab := range m.tabs {
			updated, cmd := tab.Update(tc)
			m.tabs[id] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
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
func (m *DashboardApp) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m *DashboardApp) view() string {
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

	// Active tab content — clamped to rect.Height as a safety net
	// so no tab can push the dashboard taller than the terminal.
	if tab, ok := m.tabs[m.activeTab]; ok {
		content := tab.View()
		content = clampHeight(content, m.rect.Height)
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
		// A tab in a capturing mode (inline rename/create/search/edit input,
		// a y/N confirm) owns Esc — let it cancel that mode rather than
		// closing the dashboard. Returning nil here lets Update fall through
		// to forwarding the key to the active tab.
		if tab, ok := m.tabs[m.activeTab]; ok && tab.CapturesEscape() {
			return nil
		}
		m.Quitting = true
		return tea.Quit
	}
	return nil
}

// handleTabSwitchKey handles tab navigation keys.
// Alt+1–9 map to m.tabOrder[0..8] (consistent with tmux Alt+N tab switching,
// and avoids stealing plain digit keys from text inputs).
// Returns (true, cmd) if the key was a tab switch, (false, nil) otherwise.
func (m *DashboardApp) handleTabSwitchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	s := msg.String()
	// Alt+1 through Alt+9.
	if len(s) == 5 && s[:4] == "alt+" && s[4] >= '1' && s[4] <= '9' {
		idx := int(s[4] - '1')
		if idx < len(m.tabOrder) {
			return true, m.switchToIndex(idx)
		}
	}
	switch s {
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

// clampHeight truncates content to at most h lines. Safety net so no tab
// can accidentally push the dashboard taller than the terminal.
func clampHeight(content string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > h {
		return strings.Join(lines[:h], "\n")
	}
	return content
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

	case ThemeChangeIntent:
		// Broadcast via a tea.Cmd rather than a recursive m.Update call.
		// The recursive call worked only because the two structs happened
		// to share the same field shape — any future drift on either
		// side would silently miscompile at runtime.
		//nolint:staticcheck // S1016: intentional explicit-field copy, not a type
		// conversion — guards against silent breakage if the two structs drift.
		changed := ThemeChangedMsg{Palette: it.Palette, Styles: it.Styles}
		return m, func() tea.Msg { return changed }
	}

	return m, nil
}
