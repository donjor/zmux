package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tmux"
)

func newTestDashboard() (DashboardModel, *tmux.MockRunner) {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	now := time.Now()
	mock.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Dir: "/home/user/work"},
		{Name: "api", Windows: 2, Attached: false, Activity: now, Dir: "/home/user/api"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Dir: "/tmp"},
	}
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true, Dir: "/home/user/work"},
			{Index: 2, Name: "server", Active: false, Dir: "/home/user/work"},
		},
	}

	styles := DefaultStyles()
	model := NewDashboardModel(mock, styles)
	return model, mock
}

// simulateDashboardInit runs Init and feeds results back to Update.
func simulateDashboardInit(model DashboardModel) DashboardModel {
	cmd := model.Init()
	if cmd == nil {
		return model
	}

	// Handle batch commands.
	cmds := extractBatchCmds(cmd)
	for _, c := range cmds {
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		result, _ := model.Update(msg)
		model = result.(DashboardModel)
	}

	return model
}

// extractBatchCmds unwraps a tea.Batch into individual commands.
// For non-batch commands, returns a single-element slice.
func extractBatchCmds(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}

	// Try to invoke the cmd - if it returns a tea.BatchMsg, extract sub-cmds.
	msg := cmd()
	if msg == nil {
		return nil
	}

	if batch, ok := msg.(tea.BatchMsg); ok {
		cmds := make([]tea.Cmd, 0, len(batch))
		for _, c := range batch {
			cmds = append(cmds, c)
		}
		return cmds
	}

	// Not a batch - wrap the msg in a cmd that returns it.
	return []tea.Cmd{func() tea.Msg { return msg }}
}

func sendDashboardKey(model DashboardModel, keyStr string) DashboardModel {
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
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	}

	result, _ := model.Update(msg)
	return result.(DashboardModel)
}

func TestDashboardInitInPaletteMode(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	if model.mode != ModePalette {
		t.Errorf("expected ModePalette on init, got %d", model.mode)
	}
}

func TestDashboardTabTogglesModes(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Start in palette mode.
	if model.mode != ModePalette {
		t.Fatalf("expected ModePalette initially, got %d", model.mode)
	}

	// Tab switches to dashboard.
	model = sendDashboardKey(model, "tab")
	if model.mode != ModeDashboard {
		t.Errorf("expected ModeDashboard after tab, got %d", model.mode)
	}

	// Tab switches back to palette.
	model = sendDashboardKey(model, "tab")
	if model.mode != ModePalette {
		t.Errorf("expected ModePalette after second tab, got %d", model.mode)
	}
}

func TestDashboardEscQuits(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// In palette mode, esc quits.
	model = sendDashboardKey(model, "esc")
	if !model.Quitting {
		t.Error("expected Quitting to be true after esc in palette mode")
	}
}

func TestDashboardEscQuitsFromDashboard(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard, then esc.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "esc")

	if !model.Quitting {
		t.Error("expected Quitting to be true after esc in dashboard mode")
	}
}

func TestDashboardQuitFromDashboard(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard, then q.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "q")

	if !model.Quitting {
		t.Error("expected Quitting to be true after q in dashboard mode")
	}
}

func TestDashboardSessionsLoaded(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	if len(model.sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(model.sessions))
	}
}

func TestDashboardNavigateInDashboard(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard mode.
	model = sendDashboardKey(model, "tab")

	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}

	model = sendDashboardKey(model, "j")
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", model.cursor)
	}

	model = sendDashboardKey(model, "j")
	if model.cursor != 2 {
		t.Errorf("expected cursor at 2 after j, got %d", model.cursor)
	}

	// Should not go past end.
	model = sendDashboardKey(model, "j")
	if model.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", model.cursor)
	}

	model = sendDashboardKey(model, "k")
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", model.cursor)
	}
}

func TestDashboardEnterSwitchesSession(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard mode and press enter.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "enter")

	if model.Action != "switch" {
		t.Errorf("expected action 'switch', got %q", model.Action)
	}
	if model.Chosen == "" {
		t.Error("expected a chosen session name")
	}
	if !model.Quitting {
		t.Error("expected Quitting to be true")
	}
}

func TestDashboardNewSession(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard mode and press n.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "n")

	if model.Action != "new" {
		t.Errorf("expected action 'new', got %q", model.Action)
	}
	if !model.Quitting {
		t.Error("expected Quitting to be true")
	}
}

func TestDashboardTemplateSession(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard mode and press t.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "t")

	if model.Action != "template" {
		t.Errorf("expected action 'template', got %q", model.Action)
	}
	if !model.Quitting {
		t.Error("expected Quitting to be true")
	}
}

func TestDashboardViewPaletteMode(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)
	model.width = 80
	model.height = 40

	view := model.View()

	if !strings.Contains(view, "command palette") {
		t.Error("expected palette view to contain 'command palette'")
	}
	if !strings.Contains(view, "Switch Session") {
		t.Error("expected palette view to contain 'Switch Session'")
	}
}

func TestDashboardViewDashboardMode(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)
	model.width = 80
	model.height = 40

	// Switch to dashboard.
	model = sendDashboardKey(model, "tab")

	view := model.View()

	if !strings.Contains(view, "Sessions") {
		t.Error("expected dashboard view to contain 'Sessions' title")
	}
	if !strings.Contains(view, "enter:switch") {
		t.Error("expected dashboard view to contain action hints")
	}
}

func TestDashboardViewQuittingEmpty(t *testing.T) {
	model, _ := newTestDashboard()
	model.Quitting = true

	view := model.View()
	if view != "" {
		t.Errorf("expected empty view when quitting, got %q", view)
	}
}

func TestDashboardWindowSizeMsg(t *testing.T) {
	model, _ := newTestDashboard()

	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = result.(DashboardModel)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

func TestDashboardCleanupTmp(t *testing.T) {
	model, mock := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard and press c.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "c")

	// Should have called KillSession for tmp-1.
	found := false
	for _, c := range mock.Calls {
		if c.Method == "KillSession" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected KillSession to be called for tmp session cleanup")
	}
}

func TestDashboardDeleteConfirm(t *testing.T) {
	model, _ := newTestDashboard()
	model = simulateDashboardInit(model)

	// Switch to dashboard and press d.
	model = sendDashboardKey(model, "tab")
	model = sendDashboardKey(model, "d")

	if model.subMode != dashSubConfirm {
		t.Errorf("expected dashSubConfirm mode, got %d", model.subMode)
	}
}
