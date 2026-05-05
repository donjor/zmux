package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/theme"
)

func newTestThemeResolver() *theme.Resolver {
	// Use bundled themes only (no user/iterm2 dirs).
	fs := &noopFS{}
	return theme.NewResolver(fs, "", "")
}

// noopFS satisfies config.FS but returns errors/nil for all file ops.
// This is fine since we only use bundled themes in tests.
type noopFS struct{}

func (noopFS) ReadFile(path string) ([]byte, error)                       { return nil, os.ErrNotExist }
func (noopFS) WriteFile(path string, data []byte, perm os.FileMode) error { return nil }
func (noopFS) MkdirAll(path string, perm os.FileMode) error               { return nil }
func (noopFS) Stat(path string) (os.FileInfo, error)                      { return nil, os.ErrNotExist }
func (noopFS) UserHomeDir() (string, error)                               { return "/tmp", nil }
func (noopFS) Glob(pattern string) ([]string, error)                      { return nil, nil }

func newTestThemePicker() ThemePickerModel {
	resolver := newTestThemeResolver()
	styles := DefaultStyles()
	return NewThemePickerModel(resolver, styles)
}

func sendThemeKey(model ThemePickerModel, keyStr string) ThemePickerModel {
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
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	}

	result, _ := model.Update(msg)
	return result.(ThemePickerModel)
}

func TestThemePickerLoadsBundledThemes(t *testing.T) {
	model := newTestThemePicker()

	if len(model.themes) == 0 {
		t.Fatal("expected bundled themes to be loaded")
	}

	// Check for ayu-dark specifically.
	found := false
	for _, ti := range model.themes {
		if ti.Name == "ayu-dark" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ayu-dark in theme list")
	}
}

func TestThemePickerNavigateUpDown(t *testing.T) {
	model := newTestThemePicker()

	if model.tree.Cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.tree.Cursor)
	}

	model = sendThemeKey(model, "j")
	if model.tree.Cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", model.tree.Cursor)
	}

	model = sendThemeKey(model, "k")
	if model.tree.Cursor != 0 {
		t.Errorf("expected cursor at 0 after k, got %d", model.tree.Cursor)
	}

	// Should not go below 0.
	model = sendThemeKey(model, "k")
	if model.tree.Cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", model.tree.Cursor)
	}
}

func TestThemePickerQuit(t *testing.T) {
	model := newTestThemePicker()

	model = sendThemeKey(model, "q")
	if !model.Quitting {
		t.Error("expected Quitting to be true after q")
	}
	if model.Chosen != "" {
		t.Errorf("expected empty Chosen after quit, got %q", model.Chosen)
	}
}

func TestThemePickerEscCancels(t *testing.T) {
	model := newTestThemePicker()

	model = sendThemeKey(model, "esc")
	if !model.Quitting {
		t.Error("expected Quitting to be true after esc")
	}
	if model.Chosen != "" {
		t.Errorf("expected empty Chosen after esc, got %q", model.Chosen)
	}
}

func TestThemePickerEnterChooses(t *testing.T) {
	model := newTestThemePicker()

	model = sendThemeKey(model, "enter")
	if model.Chosen == "" {
		t.Error("expected a chosen theme name after enter")
	}
}

func TestThemePickerFilterMode(t *testing.T) {
	model := newTestThemePicker()

	// Enter filter mode.
	model = sendThemeKey(model, "/")
	if model.mode != themeFilter {
		t.Error("expected themeFilter mode after pressing /")
	}

	// Press esc to exit filter mode.
	model = sendThemeKey(model, "esc")
	if model.mode != themeList {
		t.Error("expected themeList mode after pressing esc in filter mode")
	}
}

func TestThemePickerViewRendersContent(t *testing.T) {
	model := newTestThemePicker()
	model.width = 80
	model.height = 40

	view := model.View()

	if !strings.Contains(view, "zmux") {
		t.Error("expected view to contain 'zmux' title")
	}
	if !strings.Contains(view, "theme picker") {
		t.Error("expected view to contain 'theme picker' subtitle")
	}
	if !strings.Contains(view, "enter:apply") {
		t.Error("expected view to contain help text")
	}
}

func TestThemePickerViewEmpty(t *testing.T) {
	model := newTestThemePicker()
	model.Quitting = true

	view := model.View()
	if view != "" {
		t.Errorf("expected empty view when quitting without choice, got %q", view)
	}
}

func TestThemePickerWindowSizeMsg(t *testing.T) {
	model := newTestThemePicker()

	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = result.(ThemePickerModel)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

func TestThemePickerThemeInfo(t *testing.T) {
	model := newTestThemePicker()
	model.width = 80
	model.height = 40

	view := model.View()

	// Should show source tags.
	if !strings.Contains(view, "bundled") {
		t.Error("expected view to contain 'bundled' source tag")
	}

	// Should show dark/light tags.
	if !strings.Contains(view, "dark") {
		t.Error("expected view to contain 'dark' tag")
	}
}

// TestThemePickerCursorStableAcrossFilterClear covers the behavior gain
// from adopting outline.Tree: typing a filter, landing the cursor on a
// theme, then clearing the filter should leave the cursor on the same
// theme — not jump back to row 0.
func TestThemePickerCursorStableAcrossFilterClear(t *testing.T) {
	model := newTestThemePicker()

	// Find a theme index that isn't row 0 in the unfiltered list, so
	// we can tell the difference between "restored to ID" and
	// "snapped to 0".
	if len(model.themes) < 3 {
		t.Skip("need at least 3 bundled themes")
	}
	targetName := model.themes[2].Name

	// Enter filter mode and type enough of the target's name to narrow
	// hard enough that its row is at index 0 in the filtered view.
	model = sendThemeKey(model, "/")
	for _, r := range targetName {
		model.filter.SetValue(model.filter.Value() + string(r))
	}
	model.applyFilter()

	// Verify cursor is now pointing at the target.
	cur := model.currentTheme()
	if cur == nil || cur.Name != targetName {
		t.Fatalf("after filter, cursor = %v, want %q",
			nameOrNil(cur), targetName)
	}

	// Clear the filter. The list is back to the full set, but the
	// cursor should still land on targetName via stable-ID restore.
	model.filter.SetValue("")
	model.applyFilter()

	cur = model.currentTheme()
	if cur == nil || cur.Name != targetName {
		t.Errorf("after filter clear, cursor = %v, want %q (stable ID restore)",
			nameOrNil(cur), targetName)
	}
}

func nameOrNil(ti *theme.ThemeInfo) string {
	if ti == nil {
		return "<nil>"
	}
	return ti.Name
}
