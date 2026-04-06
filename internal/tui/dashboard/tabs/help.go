package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

// HelpTab displays a static, scrollable keybinding reference.
type HelpTab struct {
	styles tui.Styles
	width  int
	height int
	scroll int
}

// NewHelpTab creates a new help tab.
func NewHelpTab(styles tui.Styles) *HelpTab {
	return &HelpTab{styles: styles}
}

func (t *HelpTab) ID() dashboard.TabID  { return dashboard.TabHelp }
func (t *HelpTab) Title() string         { return "Help" }
func (t *HelpTab) Init() tea.Cmd         { return nil }

func (t *HelpTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	if tc, ok := msg.(dashboard.ThemeChangedMsg); ok {
		t.styles = tc.Styles
		return t, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
			if t.scroll > 0 {
				t.scroll--
			}
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
			t.scroll++
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("g"))):
			t.scroll = 0
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("G"))):
			t.scroll = len(t.helpLines()) - t.height + 5
			if t.scroll < 0 {
				t.scroll = 0
			}
		}
	}
	return t, nil
}

func (t *HelpTab) View() string {
	lines := t.helpLines()

	// Apply scroll.
	start := t.scroll
	if start >= len(lines) {
		start = max(0, len(lines)-1)
	}
	end := start + t.height - 2
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString("\n")
	for _, line := range lines[start:end] {
		b.WriteString(line + "\n")
	}

	if end < len(lines) {
		b.WriteString(t.styles.Dim.Render("  ↓ scroll down for more") + "\n")
	}

	return b.String()
}

func (t *HelpTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

func (t *HelpTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	return nil
}

func (t *HelpTab) Deactivate() {}

func (t *HelpTab) ShortHelp() string {
	return "j/k:scroll  g/G:top/bottom"
}

func (t *HelpTab) helpLines() []string {
	accent := t.styles.Accent
	dim := t.styles.Dim
	normal := t.styles.Normal

	section := func(title string) string {
		return accent.Bold(true).Render(title)
	}
	binding := func(keys, desc string) string {
		return "  " + normal.Bold(true).Render(keys) + dim.Render("  "+desc)
	}

	return []string{
		section("Dashboard"),
		binding("1-5", "Switch to tab by number"),
		binding("Tab / Shift+Tab", "Cycle through tabs"),
		binding("Esc", "Close dashboard"),
		"",
		section("This Session Tab"),
		binding("j / k / Up / Down", "Navigate window list"),
		binding("Enter", "Focus selected window"),
		binding("n", "Create new window"),
		binding("r", "Rename selected window"),
		binding("x", "Close selected window (with confirmation)"),
		binding("m", "Move window to another session"),
		binding("< / >", "Reorder windows (swap left/right)"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Sessions Tab"),
		binding("j / k / Up / Down", "Navigate session list"),
		binding("Enter", "Switch to selected session"),
		binding("n", "Create new tmp session"),
		binding("r", "Rename selected session"),
		binding("d", "Kill selected session (with confirmation)"),
		binding("m", "Move current window to another session"),
		binding("p", "Preview selected session's pane"),
		binding("c", "Clean up unattached tmp sessions"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Settings Tab"),
		binding("h / l / Left / Right", "Switch section (Theme, Bar, General)"),
		binding("j / k / Up / Down", "Navigate within section"),
		binding("Enter", "Apply / edit / cycle / toggle"),
		binding("/", "Filter themes (Theme section)"),
		binding("s", "Save config changes (General section)"),
		binding("Esc", "Cancel editing"),
		"",
		section("Command Palette"),
		binding("prefix + p", "Open command palette"),
		binding("Type", "Fuzzy search actions"),
		binding("Enter", "Execute selected action"),
		binding("Esc", "Close palette"),
		binding("j / k / Up / Down", "Navigate action list"),
		"",
		section("tmux Prefix Bindings"),
		binding("prefix + Space", "Open dashboard"),
		binding("prefix + p", "Open command palette"),
		binding("prefix + d", "Detach from session"),
		binding("prefix + ?", "Open help popup"),
		binding("prefix + r", "Reload zmux config (zmux apply)"),
		binding("prefix + c", "New window (tab)"),
		binding("prefix + n", "Next window"),
		binding("prefix + .", "Rename window"),
		binding("prefix + ,", "Rename session"),
		binding("Alt+1-5", "Switch to window 1-5 (no prefix)"),
		"",
		section("Copy Mode (vi keys)"),
		binding("prefix + v", "Enter copy mode"),
		binding("v", "Begin selection"),
		binding("Ctrl+v", "Toggle rectangle selection"),
		binding("y", "Yank selection to clipboard"),
		binding("Escape", "Cancel copy mode"),
		binding("/", "Search forward"),
		binding("?", "Search backward"),
		"",
		section("Session Behavior"),
		"  " + dim.Render("Attaching to an already-attached session creates a"),
		"  " + dim.Render("grouped session (shared windows, independent viewport)."),
		"  " + dim.Render("The grouped session is named <session>-b, -c, etc."),
		"  " + dim.Render("It is automatically cleaned up on detach."),
		"",
		section("Tmp Sessions"),
		"  " + dim.Render("Sessions named tmp-N are temporary and auto-cleaned"),
		"  " + dim.Render("when unattached (if auto_cleanup_tmp is enabled)."),
	}
}
