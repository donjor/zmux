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

// ── Test helpers ──

func newTestBarTab(t *testing.T) (*BarTab, *tmux.MockRunner, *sessionsMemFS) {
	t.Helper()

	fs := newSessionsMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true

	cfg := config.DefaultConfig()
	cfg.Theme = "ayu-dark"
	cfg.Bar.Preset = "default"
	cfg.Bar.Segments = config.BarSegments{
		Git:       true,
		Workspace: true,
		Clock:     true,
		Lang:      true,
		Directory: true,
		Process:   true,
		Group:     true,
	}
	if err := config.Save(fs, "/home/user/.zmux.toml", cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	resolver := theme.NewResolver(fs, "", "")

	tab := NewBarTab(resolver, fs, mock, tui.DefaultStyles())
	tab.Resize(120, 40)
	return tab, mock, fs
}

func activateBar(t *testing.T, tab *BarTab) *BarTab {
	t.Helper()
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd == nil {
		t.Fatal("Activate returned nil cmd")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("Activate cmd returned nil msg")
	}
	out, _ := tab.Update(msg)
	return out.(*BarTab)
}

func sendBarKey(tab *BarTab, k string) (*BarTab, tea.Cmd) {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	out, cmd := tab.Update(msg)
	return out.(*BarTab), cmd
}

// ── Init / activation ──

func TestNewBarTabDefaults(t *testing.T) {
	tab, _, _ := newTestBarTab(t)

	if tab.ID() != dashboard.TabBar {
		t.Errorf("ID = %v, want %v", tab.ID(), dashboard.TabBar)
	}
	if tab.Title() != "Bar" {
		t.Errorf("Title = %q, want Bar", tab.Title())
	}
	if len(tab.presets) == 0 {
		t.Error("expected bar presets populated")
	}
}

func TestBarActivateLoadsData(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	if tab.currentBar != "default" {
		t.Errorf("currentBar = %q, want default", tab.currentBar)
	}
}

// ── Cursor navigation ──

func TestBarCursor(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	if tab.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", tab.cursor)
	}

	tab, _ = sendBarKey(tab, "j")
	if tab.cursor != 1 || tab.currentSection() != barPresets {
		t.Errorf("j: cursor=%d section=%v, want cursor=1 section=presets", tab.cursor, tab.currentSection())
	}

	tab, _ = sendBarKey(tab, "k")
	if tab.cursor != 0 {
		t.Errorf("k back to top: cursor=%d", tab.cursor)
	}

	// Walk past presets into layout section.
	for range tab.presets {
		tab, _ = sendBarKey(tab, "j")
	}
	if tab.currentSection() != barLayout {
		t.Errorf("walking past presets should enter layout, section=%v cursor=%d",
			tab.currentSection(), tab.cursor)
	}

	// Walk past layout into segments.
	for range barLayoutOptions {
		tab, _ = sendBarKey(tab, "j")
	}
	if tab.currentSection() != barSegments {
		t.Errorf("walking past layout should enter segments, section=%v cursor=%d",
			tab.currentSection(), tab.cursor)
	}
}

func TestBarPresetApplyReturnsCmd(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	tab.cursor = 1

	_, cmd := sendBarKey(tab, "enter")
	if cmd == nil {
		t.Fatal("enter on preset should return a cmd (saveConfig)")
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("saveConfig cmd produced nil msg")
	}
	if _, ok := msg.(barConfigSaveMsg); !ok {
		t.Errorf("expected barConfigSaveMsg, got %T", msg)
	}
}

func TestBarSegmentToggle(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	tab.cursor = len(tab.presets) + len(barLayoutOptions) // first segment

	before := tab.segments.Git

	_, cmd := sendBarKey(tab, " ")
	if cmd == nil {
		t.Fatal("space on segment should return a cmd")
	}

	if tab.segments.Git == before {
		t.Errorf("segment git did not toggle: before=%v after=%v", before, tab.segments.Git)
	}
}

func TestBarToggleSegmentFields(t *testing.T) {
	tab, _, _ := newTestBarTab(t)

	fields := []string{"git", "workspace", "clock", "lang", "directory", "process", "group"}
	tab.segments = config.BarSegments{}
	for _, f := range fields {
		tab.toggleSegment(f)
	}
	if !tab.segments.Git || !tab.segments.Workspace || !tab.segments.Clock ||
		!tab.segments.Lang || !tab.segments.Directory || !tab.segments.Process ||
		!tab.segments.Group {
		t.Errorf("toggleSegment did not flip all fields: %+v", tab.segments)
	}
}

// ── Layout cycling ──

func TestBarLayoutCycle(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	// Move to first layout option (Layout field).
	tab.cursor = len(tab.presets)
	if tab.currentSection() != barLayout {
		t.Fatalf("expected layout section, got %v", tab.currentSection())
	}

	// Default layout from config is "two-line".
	if tab.layout != "two-line" {
		t.Fatalf("layout = %q, want two-line", tab.layout)
	}

	// Cycle forward: two-line → split.
	_, cmd := sendBarKey(tab, "l")
	if cmd == nil {
		t.Fatal("l on layout option should return a cmd (saveConfig)")
	}
	if tab.layout != "split" {
		t.Errorf("after l: layout = %q, want split", tab.layout)
	}

	// Cycle forward again: split → single.
	sendBarKey(tab, "l")
	if tab.layout != "single" {
		t.Errorf("after l: layout = %q, want single", tab.layout)
	}

	// Cycle backward: single → split.
	sendBarKey(tab, "h")
	if tab.layout != "split" {
		t.Errorf("after h: layout = %q, want split", tab.layout)
	}
}

// ── View rendering ──

func TestBarViewShowsCurrent(t *testing.T) {
	tab, _, _ := newTestBarTab(t)
	tab = activateBar(t, tab)

	view := tab.View()
	if !strings.Contains(view, "Current") {
		t.Error("bar view should show 'Current' label")
	}
}
