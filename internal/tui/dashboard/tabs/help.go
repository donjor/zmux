package tabs

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/keys"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
)

// HelpTab displays a static, scrollable keybinding reference.
type HelpTab struct {
	styles styles.Styles
	vp     viewport.Model
	width  int
	height int
}

// NewHelpTab creates a new help tab.
func NewHelpTab(styles styles.Styles) *HelpTab {
	return &HelpTab{styles: styles}
}

func (t *HelpTab) ID() dashboard.TabID { return dashboard.TabHelp }
func (t *HelpTab) Title() string       { return "Help" }
func (t *HelpTab) Init() tea.Cmd       { return nil }

// CapturesEscape is always false — the Help tab is static, so Esc closes the
// dashboard.
func (t *HelpTab) CapturesEscape() bool { return false }

func (t *HelpTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	if tc, ok := msg.(dashboard.ThemeChangedMsg); ok {
		t.styles = tc.Styles
		return t, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
			t.vp.ScrollUp(1)
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
			t.vp.ScrollDown(1)
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
	t.vp.SetWidth(width)
	t.vp.SetHeight(height)
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
	binding := func(keyLabel, desc string) string {
		return "  " + normal.Bold(true).Render(keyLabel) + dim.Render("  "+desc)
	}
	// registryLines renders a keys-registry slice into a single newline-joined
	// block so the tmux/no-prefix/copy-mode sections share one source of truth
	// with conf.go and the generated docs.
	registryLines := func(bindings []keys.Binding) string {
		out := make([]string, 0, len(bindings))
		for _, kb := range bindings {
			out = append(out, binding(kb.Humanize(), kb.Help))
		}
		return strings.Join(out, "\n")
	}

	return []string{
		section("Dashboard"),
		binding("Alt+1-9", "Switch to tab by number"),
		binding("Tab / Shift+Tab", "Cycle through tabs"),
		binding("Esc", "Close dashboard"),
		"",
		section("Session Tab"),
		binding("j / k", "Navigate workspace / session / window tree"),
		binding("Enter", "Focus selected window, switch to sibling session"),
		binding("c", "Create window / session at cursor scope"),
		binding("r", "Rename selected item (workspace, session, or window)"),
		binding("x", "Kill selected item (with confirmation)"),
		binding("m", "Move window to another session"),
		binding("< / >", "Reorder windows (swap left/right)"),
		binding("g / G", "Jump to top / bottom"),
		"",
		section("Workspaces Tab"),
		binding("j / k", "Navigate workspace / session tree"),
		binding("Enter", "Switch to selected session"),
		binding("c", "Create session in workspace at cursor"),
		binding("C", "Create new workspace"),
		binding("r", "Rename selected workspace or session"),
		binding("x", "Kill workspace or session (with confirmation)"),
		binding("m", "Move session to another workspace"),
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
		registryLines(keys.PrefixBindings),
		"",
		section("Inherited tmux Defaults (Ctrl+Space)"),
		registryLines(keys.InheritedBindings),
		"",
		section("No-Prefix Keys"),
		registryLines(keys.NoPrefixBindings),
		"",
		section("Tab Picker (Alt+`)"),
		binding("Enter", "Go to tab (riders focus their pane; hidden tabs return)"),
		binding("Ctrl+N", "Create new tab"),
		binding("Ctrl+R", "Rename selected tab (windows only)"),
		binding("Ctrl+X", "Close selected tab (riders/hidden close their pane)"),
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
		registryLines(keys.CopyModeBindings),
		"",
		section("Logical Tabs & Placement (CLI)"),
		"  " + dim.Render("zmux-managed tabs keep a stable id, label, and state"),
		"  " + dim.Render("glyph across placement moves: full window, pane inside"),
		"  " + dim.Render("another tab (zmux tab pane / tab full), or parked in the"),
		"  " + dim.Render("hidden dock (zmux tab hide / tab show). send/type/watch/"),
		"  " + dim.Render("run -n reach a tab by name in every placement. Placement"),
		"  " + dim.Render("verbs refuse while grouped viewports are attached."),
		"",
		section("QA Walkthroughs (./qa, repo tool)"),
		"  " + dim.Render("./qa (in the zmux repo) drives committed checklists"),
		"  " + dim.Render("(checklists/*.toml): a human picker (./qa) and agent verbs"),
		"  " + dim.Render("(./qa run / mark / status / reset / lint) share one"),
		"  " + dim.Render("scorecard (.qa/) with by-attribution. cmd+check steps"),
		"  " + dim.Render("verify automatically; cmd-only steps await a human"),
		"  " + dim.Render("verdict; bare steps are instructions."),
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
