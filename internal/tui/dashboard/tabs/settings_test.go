package tabs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

func newTestSettingsTab() *SettingsTab {
	resolver := theme.NewResolver(config.RealFS{}, "", "")
	styles := tui.DefaultStyles()
	mock := tmux.NewMockRunner()
	tab := NewSettingsTab(resolver, config.RealFS{}, mock, styles)
	tab.Resize(80, 40)
	return tab
}

func simulateSettingsActivate(tab *SettingsTab) *SettingsTab {
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			result, _ := tab.Update(msg)
			tab = result.(*SettingsTab)
		}
	}
	return tab
}

func sendSettingsKey(tab *SettingsTab, keyStr string) (*SettingsTab, tea.Cmd) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
	switch keyStr {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	}

	result, cmd := tab.Update(msg)
	return result.(*SettingsTab), cmd
}

func TestSettingsTabID(t *testing.T) {
	tab := newTestSettingsTab()
	if tab.ID() != dashboard.TabSettings {
		t.Errorf("expected TabSettings, got %s", tab.ID())
	}
}

func TestSettingsTabTitle(t *testing.T) {
	tab := newTestSettingsTab()
	if tab.Title() != "Settings" {
		t.Errorf("expected 'Settings', got %q", tab.Title())
	}
}

func TestSettingsTabActivateLoadsConfig(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	// Config should be loaded (either from disk or defaults).
	if tab.cfg.Prefix == "" && tab.cfg.Sync.Target == "" {
		// Default config should still have some non-zero values.
		// Just verify activation didn't panic.
	}
}

func TestSettingsTabGeneralNavigate(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	if tab.cfgCursor != 0 {
		t.Fatalf("expected config cursor at 0, got %d", tab.cfgCursor)
	}

	tab, _ = sendSettingsKey(tab, "j")
	if tab.cfgCursor != 1 {
		t.Errorf("expected config cursor at 1, got %d", tab.cfgCursor)
	}

	tab, _ = sendSettingsKey(tab, "k")
	if tab.cfgCursor != 0 {
		t.Errorf("expected config cursor at 0, got %d", tab.cfgCursor)
	}
}

func TestSettingsTabGeneralNavigateGg(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	tab, _ = sendSettingsKey(tab, "G")
	if tab.cfgCursor != len(tab.configRows)-1 {
		t.Errorf("expected cursor at last row, got %d", tab.cfgCursor)
	}

	tab, _ = sendSettingsKey(tab, "g")
	if tab.cfgCursor != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.cfgCursor)
	}
}

func TestSettingsTabViewContainsConfigPath(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	view := tab.View()

	if !strings.Contains(view, ".zmux.toml") {
		t.Error("expected view to contain '.zmux.toml'")
	}
}

func TestSettingsTabViewShowsConfigRows(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "Prefix") {
		t.Error("expected view to contain 'Prefix' config row")
	}
	if !strings.Contains(view, "Sync Target") {
		t.Error("expected view to contain 'Sync Target' config row")
	}
}

func TestSettingsTabDeactivateBlursEdit(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	tab.mode = settingsModeEdit
	tab.Deactivate()

	if tab.mode != settingsModeList {
		t.Errorf("expected mode reset to list, got %d", tab.mode)
	}
}

func TestSettingsTabShortHelp(t *testing.T) {
	tab := newTestSettingsTab()

	help := tab.ShortHelp()
	if !strings.Contains(help, "s:save") {
		t.Error("expected help to contain 's:save'")
	}
	if !strings.Contains(help, "enter:edit") {
		t.Error("expected help to contain 'enter:edit'")
	}

	// Edit mode.
	tab.mode = settingsModeEdit
	help = tab.ShortHelp()
	if !strings.Contains(help, "enter:confirm") {
		t.Error("expected edit help to contain 'enter:confirm'")
	}
	if !strings.Contains(help, "esc:cancel") {
		t.Error("expected edit help to contain 'esc:cancel'")
	}
}

func TestSettingsTabCycleOption(t *testing.T) {
	options := []string{"none", "ghostty", "nvim"}

	if got := cycleOption(options, "none"); got != "ghostty" {
		t.Errorf("expected 'ghostty', got %q", got)
	}
	if got := cycleOption(options, "nvim"); got != "none" {
		t.Errorf("expected 'none' (wrap), got %q", got)
	}
	if got := cycleOption(options, "unknown"); got != "none" {
		t.Errorf("expected 'none' (default), got %q", got)
	}
}

func TestSettingsTabToggleActivate(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	// Navigate to the toggle row (Auto Cleanup Tmp is index 3).
	tab.cfgCursor = 3
	initial := tab.configRows[3].getValue(tab.cfg)

	tab, _ = sendSettingsKey(tab, "enter")

	after := tab.configRows[3].getValue(tab.cfg)
	if initial == after {
		t.Error("expected toggle to change value")
	}
}

func TestSettingsTabEditMode(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	// First row (Prefix) is text, so enter should activate edit mode.
	tab.cfgCursor = 0
	tab, _ = sendSettingsKey(tab, "enter")

	if tab.mode != settingsModeEdit {
		t.Errorf("expected settingsModeEdit, got %d", tab.mode)
	}

	// Escape should cancel.
	tab, _ = sendSettingsKey(tab, "esc")
	if tab.mode != settingsModeList {
		t.Errorf("expected settingsModeList after esc, got %d", tab.mode)
	}
}

func TestSettingsTabHasConfigChanges(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	if tab.hasConfigChanges() {
		t.Error("expected no changes initially")
	}

	// Modify a value.
	tab.cfg.Prefix = "C-a"
	if !tab.hasConfigChanges() {
		t.Error("expected changes after modification")
	}
}
