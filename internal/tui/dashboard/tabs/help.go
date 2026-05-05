package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

// HelpTab displays a static, scrollable keybinding reference.
type HelpTab struct {
	styles tui.Styles
	vp     viewport.Model
	width  int
	height int
}

// NewHelpTab creates a new help tab.
func NewHelpTab(styles tui.Styles) *HelpTab {
	return &HelpTab{styles: styles}
}

func (t *HelpTab) ID() dashboard.TabID { return dashboard.TabHelp }
func (t *HelpTab) Title() string       { return "Help" }
func (t *HelpTab) Init() tea.Cmd       { return nil }

func (t *HelpTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	if tc, ok := msg.(dashboard.ThemeChangedMsg); ok {
		t.styles = tc.Styles
		return t, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
			t.vp.LineUp(1)
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
			t.vp.LineDown(1)
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("g"))):
			t.vp.GotoTop()
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("G"))):
			t.vp.GotoBottom()
		}
	}
	return t, nil
}

func (t *HelpTab) View() string {
	lines := t.helpLines()
	content := "\n" + strings.Join(lines, "\n")
	t.vp.SetContent(content)
	return renderScrollable(t.vp, t.styles)
}

func (t *HelpTab) Resize(width, height int) {
	t.width = width
	t.height = height
	t.vp.Width = width
	t.vp.Height = height
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
		binding("Alt+1-5", "Switch to tab by number"),
		binding("Tab / Shift+Tab", "Cycle through tabs"),
		binding("Esc", "Close dashboard"),
		"",
		section("Session Tab"),
		binding("j / k", "Navigate workspace / session / window tree"),
		binding("Enter", "Focus selected window, switch to sibling session"),
		binding("n", "Create new window in current session"),
		binding("r", "Rename selected item (workspace, session, or window)"),
		binding("x", "Kill selected item (with confirmation)"),
		binding("m", "Move window to another session"),
		binding("< / >", "Reorder windows (swap left/right)"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Workspaces Tab"),
		binding("j / k", "Navigate workspace / session tree"),
		binding("Enter", "Switch to selected session"),
		binding("n", "Create new workspace"),
		binding("r", "Rename selected workspace or session"),
		binding("x", "Kill workspace or session (with confirmation)"),
		binding("m", "Move session to another workspace"),
		binding("p", "Preview selected session"),
		binding("c", "Clean up unattached tmp sessions"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Themes Tab"),
		binding("j / k", "Navigate themes"),
		binding("Enter", "Apply theme"),
		binding("/", "Filter themes"),
		binding("e", "Edit theme colors inline"),
		binding("c", "Clone theme to custom"),
		binding("q", "Revert preview"),
		"",
		section("Bar Tab"),
		binding("j / k", "Navigate presets and segments"),
		binding("Enter", "Apply preset"),
		binding("Space", "Toggle segment"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Settings Tab"),
		binding("j / k", "Navigate config fields"),
		binding("Enter", "Edit / cycle / toggle value"),
		binding("s", "Save config changes"),
		binding("Esc", "Cancel editing"),
		"",
		section("tmux Prefix Keys (Ctrl+Space)"),
		binding("Space", "Open dashboard"),
		binding("p", "Open command palette"),
		binding("d", "Detach from session"),
		binding("?", "Help popup"),
		binding("w", "Workspace session picker"),
		binding("[ / ]", "Previous / next session in workspace"),
		binding("c", "New tab"),
		binding("n / N", "Next / previous tab"),
		binding("< / >", "Move tab left / right"),
		binding("x", "Close tab (with confirm)"),
		binding(".", "Label tab (blank clears)"),
		binding(",", "Rename session"),
		binding("r", "Reload zmux config (zmux apply)"),
		binding("v", "Enter vi copy mode"),
		binding("P", "Paste buffer"),
		"",
		section("No-Prefix Keys"),
		binding("Alt+1-9", "Switch to tab N directly"),
		binding("Shift+Alt+1-9", "Switch to session N in workspace"),
		binding("Alt+`", "Tab switcher popup"),
		"",
		section("Tab Picker (Alt+`)"),
		binding("Enter", "Switch to selected tab"),
		binding("Ctrl+N", "Create new tab"),
		binding("Ctrl+R", "Rename selected tab"),
		binding("Ctrl+X", "Close selected tab"),
		binding("< / >", "Reorder tabs"),
		binding("Esc", "Close"),
		"",
		section("Command Palette (prefix+p)"),
		binding("Type", "Fuzzy search all actions"),
		binding("Enter", "Execute selected action"),
		binding("Esc", "Close palette"),
		binding("j / k", "Navigate action list"),
		"",
		section("Copy Mode (vi keys)"),
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
