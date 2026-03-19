package tabs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

func newTestSettingsTab() *SettingsTab {
	resolver := theme.NewResolver(config.RealFS{}, "", "")
	styles := tui.DefaultStyles()
	tab := NewSettingsTab(resolver, config.RealFS{}, styles)
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

func TestSettingsTabActivateLoadsData(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	if len(tab.themes) == 0 {
		t.Error("expected themes to be loaded")
	}
	if len(tab.filtered) == 0 {
		t.Error("expected filtered themes to be populated")
	}
}

func TestSettingsTabSectionSwitch(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	if tab.section != sectionTheme {
		t.Fatalf("expected initial section Theme, got %d", tab.section)
	}

	// Switch to Bar.
	tab, _ = sendSettingsKey(tab, "right")
	if tab.section != sectionBar {
		t.Errorf("expected section Bar, got %d", tab.section)
	}

	// Switch to General.
	tab, _ = sendSettingsKey(tab, "right")
	if tab.section != sectionGeneral {
		t.Errorf("expected section General, got %d", tab.section)
	}

	// Should not go past end.
	tab, _ = sendSettingsKey(tab, "right")
	if tab.section != sectionGeneral {
		t.Errorf("expected section still General, got %d", tab.section)
	}

	// Switch back.
	tab, _ = sendSettingsKey(tab, "left")
	if tab.section != sectionBar {
		t.Errorf("expected section Bar, got %d", tab.section)
	}

	tab, _ = sendSettingsKey(tab, "left")
	if tab.section != sectionTheme {
		t.Errorf("expected section Theme, got %d", tab.section)
	}

	// Should not go below 0.
	tab, _ = sendSettingsKey(tab, "left")
	if tab.section != sectionTheme {
		t.Errorf("expected section still Theme, got %d", tab.section)
	}
}

func TestSettingsTabThemeNavigate(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	if tab.themeCursor != 0 {
		t.Fatalf("expected theme cursor at 0, got %d", tab.themeCursor)
	}

	tab, _ = sendSettingsKey(tab, "j")
	if tab.themeCursor != 1 {
		t.Errorf("expected theme cursor at 1, got %d", tab.themeCursor)
	}

	tab, _ = sendSettingsKey(tab, "k")
	if tab.themeCursor != 0 {
		t.Errorf("expected theme cursor at 0, got %d", tab.themeCursor)
	}
}

func TestSettingsTabThemeFilterMode(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	tab, _ = sendSettingsKey(tab, "/")
	if tab.mode != settingsModeFilter {
		t.Errorf("expected settingsModeFilter, got %d", tab.mode)
	}
}

func TestSettingsTabThemeFilterCancel(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	tab, _ = sendSettingsKey(tab, "/")
	tab, _ = sendSettingsKey(tab, "esc")

	if tab.mode != settingsModeList {
		t.Errorf("expected settingsModeList after esc, got %d", tab.mode)
	}
}

func TestSettingsTabBarNavigate(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	// Switch to bar section.
	tab, _ = sendSettingsKey(tab, "right")
	if tab.section != sectionBar {
		t.Fatalf("expected bar section, got %d", tab.section)
	}

	// Reset cursor to 0 for deterministic testing (actual position depends on config on disk).
	tab.barCursor = 0

	tab, _ = sendSettingsKey(tab, "j")
	if tab.barCursor != 1 {
		t.Errorf("expected bar cursor at 1, got %d", tab.barCursor)
	}

	tab, _ = sendSettingsKey(tab, "k")
	if tab.barCursor != 0 {
		t.Errorf("expected bar cursor at 0, got %d", tab.barCursor)
	}
}

func TestSettingsTabGeneralNavigate(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	// Switch to general section.
	tab, _ = sendSettingsKey(tab, "right")
	tab, _ = sendSettingsKey(tab, "right")
	if tab.section != sectionGeneral {
		t.Fatalf("expected general section, got %d", tab.section)
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

func TestSettingsTabViewContainsSectionHeaders(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "Theme") {
		t.Error("expected view to contain 'Theme' section header")
	}
	if !strings.Contains(view, "Bar") {
		t.Error("expected view to contain 'Bar' section header")
	}
	if !strings.Contains(view, "General") {
		t.Error("expected view to contain 'General' section header")
	}
}

func TestSettingsTabViewThemeSectionShowsCurrent(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)
	tab.currentTheme = "ayu-dark"

	view := tab.View()

	if !strings.Contains(view, "ayu-dark") {
		t.Error("expected view to contain current theme 'ayu-dark'")
	}
}

func TestSettingsTabDeactivateBlursInputs(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	tab.mode = settingsModeFilter
	tab.Deactivate()

	if tab.mode != settingsModeList {
		t.Errorf("expected mode reset to list, got %d", tab.mode)
	}
}

func TestSettingsTabShortHelp(t *testing.T) {
	tab := newTestSettingsTab()

	// Theme section.
	tab.section = sectionTheme
	help := tab.ShortHelp()
	if !strings.Contains(help, "enter:apply") {
		t.Error("expected help to contain 'enter:apply' for theme section")
	}
	if !strings.Contains(help, "/:filter") {
		t.Error("expected help to contain '/:filter' for theme section")
	}

	// Bar section.
	tab.section = sectionBar
	help = tab.ShortHelp()
	if !strings.Contains(help, "enter:apply") {
		t.Error("expected help to contain 'enter:apply' for bar section")
	}

	// General section.
	tab.section = sectionGeneral
	help = tab.ShortHelp()
	if !strings.Contains(help, "s:save") {
		t.Error("expected help to contain 's:save' for general section")
	}
}

func TestSettingsTabApplyFilter(t *testing.T) {
	tab := newTestSettingsTab()
	tab = simulateSettingsActivate(tab)

	initialCount := len(tab.filtered)
	if initialCount == 0 {
		t.Skip("no themes to filter")
	}

	tab.filter.SetValue("xxxxxxnonexistent")
	tab.applyFilter()

	if len(tab.filtered) != 0 {
		t.Errorf("expected 0 filtered results, got %d", len(tab.filtered))
	}

	tab.filter.SetValue("")
	tab.applyFilter()

	if len(tab.filtered) != initialCount {
		t.Errorf("expected %d results after clearing filter, got %d", initialCount, len(tab.filtered))
	}
}
