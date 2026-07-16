package tabs

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/donjor/zmux/internal/tui/tkey"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
)

// ── Test helpers ──

// newTestThemesTab builds a ThemesTab backed by the sessionsMemFS from
// sessions_test.go (shared), a real theme.Resolver pointing at the
// bundled themes (empty user + iterm2 dirs), and a tmux mock. Activating
// the returned tab and feeding the Activate command back through Update
// populates theme list + config.
func newTestThemesTab(t *testing.T) (*ThemesTab, *tmux.MockRunner, *sessionsMemFS) {
	t.Helper()

	fs := newSessionsMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true

	// Seed a starter config so fetchData sees a loadable file.
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

	// Resolver with no user/iterm2 dirs — returns bundled themes only.
	resolver := theme.NewResolver(fs, "", "")

	tab := NewThemesTab(resolver, fs, mock, "zmux", styles.DefaultStyles())
	tab.Resize(120, 40)
	return tab, mock, fs
}

// activateTheme runs the Activate command and feeds the resulting
// themesDataMsg back through Update, returning the tab ready for
// interaction.
func activateTheme(t *testing.T, tab *ThemesTab) *ThemesTab {
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
	return out.(*ThemesTab)
}

// sendThemesKey sends a single-char key to the tab.
func sendThemesKey(tab *ThemesTab, k string) (*ThemesTab, tea.Cmd) {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tkey.Enter()
	case "esc":
		msg = tkey.Esc()
	case "up":
		msg = tkey.Up()
	case "down":
		msg = tkey.Down()
	case "left":
		msg = tkey.Left()
	case "right":
		msg = tkey.Right()
	case " ":
		msg = tkey.Space()
	default:
		msg = tkey.Type(k)
	}
	out, cmd := tab.Update(msg)
	return out.(*ThemesTab), cmd
}

// ── Init / activation ──

func TestNewThemesTabDefaults(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)

	if tab.ID() != dashboard.TabThemes {
		t.Errorf("ID = %v, want %v", tab.ID(), dashboard.TabThemes)
	}
	if tab.Title() != "Themes" {
		t.Errorf("Title = %q, want Themes", tab.Title())
	}
	if len(tab.editSlots) == 0 {
		t.Error("expected editor slots populated")
	}
	// All 20 editor slots: 4 named (bg/fg/cursor/selection) + 16 palette.
	if got, want := len(tab.editSlots), 20; got != want {
		t.Errorf("editor slots = %d, want %d", got, want)
	}
}

func TestThemesActivateLoadsData(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	if len(tab.themes) == 0 {
		t.Fatal("themes should be populated after activate")
	}
	if tab.currentTheme != "ayu-dark" {
		t.Errorf("currentTheme = %q, want ayu-dark", tab.currentTheme)
	}
	if len(tab.filtered) == 0 {
		t.Error("filtered should be populated after applyFilter")
	}
}

func TestThemesShortHelpChangesPerMode(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	// List mode.
	if h := tab.ShortHelp(); !strings.Contains(h, "enter:apply") {
		t.Errorf("list help missing enter:apply, got %q", h)
	}

	// Editing mode.
	tab.editing = true
	if h := tab.ShortHelp(); !strings.Contains(h, "enter:edit color") {
		t.Errorf("editing help missing enter:edit color, got %q", h)
	}
}

// ── Cursor navigation ──

// TestThemesColorsCursor pins j/k/g/G navigation over the outline.Tree. The
// test resolver exposes bundled themes only, so the tree is a single
// (non-selectable) "Bundled" header followed by the alphabetical theme rows —
// navigation walks that order and the highlighted theme is read back through
// currentThemeInfo (the cursor now lives on the tree, not a themeCursor int).
func TestThemesColorsCursor(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	names := make([]string, len(tab.filtered))
	for i, ti := range tab.filtered {
		names[i] = ti.Name
	}
	if len(names) < 2 {
		t.Fatalf("need at least 2 bundled themes to exercise nav, got %d", len(names))
	}
	cur := func() string {
		ti := tab.currentThemeInfo()
		if ti == nil {
			t.Fatal("currentThemeInfo is nil — cursor left the selectable rows")
		}
		return ti.Name
	}

	if got := cur(); got != names[0] {
		t.Fatalf("start highlighted = %q, want %q", got, names[0])
	}

	tab, _ = sendThemesKey(tab, "j")
	if got := cur(); got != names[1] {
		t.Errorf("after j: highlighted = %q, want %q", got, names[1])
	}

	tab, _ = sendThemesKey(tab, "k")
	if got := cur(); got != names[0] {
		t.Errorf("after k: highlighted = %q, want %q", got, names[0])
	}

	// G to jump to bottom.
	tab, _ = sendThemesKey(tab, "G")
	if got, want := cur(), names[len(names)-1]; got != want {
		t.Errorf("after G: highlighted = %q, want %q", got, want)
	}

	// g to jump back to top.
	tab, _ = sendThemesKey(tab, "g")
	if got := cur(); got != names[0] {
		t.Errorf("after g: highlighted = %q, want %q", got, names[0])
	}
}

// ── Filter mode ──

func TestThemesFilterMode(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	originalCount := len(tab.filtered)

	// Enter filter mode.
	tab, _ = sendThemesKey(tab, "/")
	if tab.mode != themesModeFilter {
		t.Fatalf("after /: mode = %v, want Filter", tab.mode)
	}

	// Type a narrow query that should filter heavily.
	for _, r := range "ayu" {
		tab, _ = sendThemesKey(tab, string(r))
	}
	if len(tab.filtered) == 0 {
		t.Error("filter 'ayu' produced no matches")
	}
	if len(tab.filtered) >= originalCount {
		t.Errorf("filter 'ayu' did not narrow: %d → %d", originalCount, len(tab.filtered))
	}

	// Esc clears filter.
	tab, _ = sendThemesKey(tab, "esc")
	if tab.mode != themesModeList {
		t.Errorf("after esc: mode = %v, want List", tab.mode)
	}
	if len(tab.filtered) != originalCount {
		t.Errorf("after esc clear: filtered = %d, want %d", len(tab.filtered), originalCount)
	}
}

// TestThemesCommittedFilterEscClears guards the modal-esc routing contract:
// Enter commits a filter (stays in list mode, value retained); in that state
// CapturesEscape must be true so the dashboard routes Esc here to clear the
// committed filter — matching the "esc to clear" hint the list view shows.
func TestThemesCommittedFilterEscClears(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	originalCount := len(tab.filtered)

	// Filter, then commit with Enter (filter persists in list mode).
	tab, _ = sendThemesKey(tab, "/")
	for _, r := range "ayu" {
		tab, _ = sendThemesKey(tab, string(r))
	}
	tab, _ = sendThemesKey(tab, "enter")
	if tab.mode != themesModeList {
		t.Fatalf("after enter: mode = %v, want List", tab.mode)
	}
	if tab.filter.Value() == "" {
		t.Fatal("expected committed filter value retained after enter")
	}
	if !tab.CapturesEscape() {
		t.Error("expected CapturesEscape true with a committed filter in list mode")
	}

	// Esc in list mode clears the committed filter.
	tab, _ = sendThemesKey(tab, "esc")
	if tab.filter.Value() != "" {
		t.Errorf("expected committed filter cleared by esc, got %q", tab.filter.Value())
	}
	if len(tab.filtered) != originalCount {
		t.Errorf("after esc clear: filtered = %d, want %d", len(tab.filtered), originalCount)
	}
	if tab.CapturesEscape() {
		t.Error("expected CapturesEscape false after filter cleared (esc would now quit)")
	}
}

// ── Editor flow (Colors section 'e' → editor → esc) ──

func TestThemesEnterAndExitEditor(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	tab, _ = sendThemesKey(tab, "e")
	if !tab.editing {
		t.Fatal("'e' should enter editor")
	}
	if tab.editName == "" {
		t.Error("editName should be set")
	}
	if tab.editCursor != 0 {
		t.Errorf("editCursor = %d, want 0", tab.editCursor)
	}

	// j/k moves through slots.
	tab, _ = sendThemesKey(tab, "j")
	if tab.editCursor != 1 {
		t.Errorf("after j in editor: editCursor = %d, want 1", tab.editCursor)
	}

	// esc exits editor.
	tab, _ = sendThemesKey(tab, "esc")
	if tab.editing {
		t.Error("esc should exit editor")
	}
}

// ── Editor slot buildEditorSlots sanity ──

func TestBuildEditorSlotsRoundTrip(t *testing.T) {
	slots := buildEditorSlots()
	if len(slots) != 20 {
		t.Fatalf("expected 20 slots, got %d", len(slots))
	}

	// Construct a base theme and pin a distinct hex per palette index via
	// Set, then read it back via Get. This guards the off-by-one risk in
	// the buildEditorSlots palette-loop closures.
	var th theme.Theme
	for i := 0; i < 16; i++ {
		th.Palette[i] = theme.Color{R: 1, G: uint8(i), B: uint8(i * 2)}
	}

	// The 4 named slots come first (BACKGROUND, FOREGROUND, CURSOR, SELECTION)
	// then 16 palette slots. Set each palette slot via its Set closure to a
	// sentinel value and verify Get returns the same value.
	for i := 4; i < 20; i++ {
		idx := i - 4
		sentinel := theme.Color{R: 200, G: uint8(idx), B: 100}
		slots[i].Set(&th, sentinel)
		got := slots[i].Get(th)
		if got != sentinel {
			t.Errorf("slot %d (%s): Get after Set = %v, want %v",
				i, slots[i].Label, got, sentinel)
		}
	}

	// Named slots:
	slots[0].Set(&th, theme.Color{R: 10, G: 20, B: 30})
	if th.Background != (theme.Color{R: 10, G: 20, B: 30}) {
		t.Errorf("BACKGROUND slot Set did not reach Background: %v", th.Background)
	}
	slots[1].Set(&th, theme.Color{R: 40, G: 50, B: 60})
	if th.Foreground != (theme.Color{R: 40, G: 50, B: 60}) {
		t.Errorf("FOREGROUND slot Set did not reach Foreground: %v", th.Foreground)
	}
}

// ── Message handlers ──

func TestThemesStaleDataMsgIgnored(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	before := tab.currentTheme

	// Send a themesDataMsg with a stale reqID.
	stale := themesDataMsg{
		reqID:        tab.reqID - 999,
		currentTheme: "not-the-real-theme",
	}
	out, _ := tab.Update(stale)
	tab = out.(*ThemesTab)

	if tab.currentTheme != before {
		t.Errorf("stale data msg was applied: currentTheme=%q want %q", tab.currentTheme, before)
	}
}

func TestThemesDataMsgErrorFlashesStatus(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab.reqID = 42

	errMsg := themesDataMsg{reqID: 42, err: &testError{"boom"}}
	_, cmd := tab.Update(errMsg)
	if cmd == nil {
		t.Fatal("error data msg should emit a status flash cmd")
	}
	msg := cmd()
	intent, ok := msg.(dashboard.SetStatusIntent)
	if !ok {
		t.Fatalf("expected SetStatusIntent, got %T", msg)
	}
	if !intent.IsError {
		t.Error("expected IsError=true")
	}
}

func TestThemesApplyMsgUpdatesCurrent(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	tab.reqID = 10

	// Simulate a successful apply.
	out, _ := tab.Update(themesApplyMsg{reqID: 10, themeName: "catppuccin-mocha"})
	tab = out.(*ThemesTab)

	if tab.currentTheme != "catppuccin-mocha" {
		t.Errorf("currentTheme after apply = %q, want catppuccin-mocha", tab.currentTheme)
	}
	if tab.savedTheme != "catppuccin-mocha" {
		t.Errorf("savedTheme after apply = %q, want catppuccin-mocha", tab.savedTheme)
	}
	if tab.previewing {
		t.Error("previewing should be false after apply")
	}
}

func TestThemesApplyMsgStaleIgnored(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	before := tab.currentTheme
	tab.reqID = 10

	out, _ := tab.Update(themesApplyMsg{reqID: 1, themeName: "zzz-fake"})
	tab = out.(*ThemesTab)
	if tab.currentTheme != before {
		t.Errorf("stale apply msg was applied: currentTheme=%q, want %q", tab.currentTheme, before)
	}
}

func TestThemesSaveThemeMsgExitsEditing(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	tab.editing = true
	tab.pickerActive = true
	tab.reqID = 99

	out, _ := tab.Update(themesSaveThemeMsg{reqID: 99, themeName: "my-custom"})
	tab = out.(*ThemesTab)

	if tab.editing {
		t.Error("editing should be false after successful save")
	}
	if tab.pickerActive {
		t.Error("pickerActive should be false after save")
	}
}

// ── View rendering smoke tests ──

func TestThemesViewRendersContent(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	view := tab.View()
	if !strings.Contains(view, "Current") {
		t.Error("view should contain 'Current' label")
	}
}

// ── Deactivate cleans up overlays ──

func TestThemesDeactivateClearsTransientState(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	tab.mode = themesModeFilter
	tab.editing = true
	tab.pickerActive = true
	tab.namingActive = true
	tab.previewing = false // revertPreview no-ops if not previewing

	tab.Deactivate()

	if tab.mode != themesModeList {
		t.Error("Deactivate should reset mode to List")
	}
	if tab.editing || tab.pickerActive || tab.namingActive {
		t.Errorf("Deactivate should clear editing flags, got editing=%v picker=%v naming=%v",
			tab.editing, tab.pickerActive, tab.namingActive)
	}
}

// testError is a tiny error type used in the stale/error message tests.
type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
