package dashboard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tkey"
)

// stubTab is a minimal Tab implementation for testing the app shell.
type stubTab struct {
	id          TabID
	title       string
	activated   bool
	deactivated bool
	resized     bool
	lastWidth   int
	lastHeight  int
	viewText    string
	helpText    string
	initCalled  bool
	capturesEsc bool // when true, the tab claims Esc (modal-esc routing)
	sawEsc      bool // recorded when an Esc key reaches Update
}

func newStubTab(id TabID, title string) *stubTab {
	return &stubTab{id: id, title: title, viewText: "content:" + string(id), helpText: "help:" + string(id)}
}

func (t *stubTab) ID() TabID            { return t.id }
func (t *stubTab) Title() string        { return t.title }
func (t *stubTab) Init() tea.Cmd        { t.initCalled = true; return nil }
func (t *stubTab) View() string         { return t.viewText }
func (t *stubTab) ShortHelp() string    { return t.helpText }
func (t *stubTab) CapturesEscape() bool { return t.capturesEsc }

func (t *stubTab) Update(msg tea.Msg) (Tab, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		t.sawEsc = true
	}
	return t, nil
}

func (t *stubTab) Resize(width, height int) {
	t.resized = true
	t.lastWidth = width
	t.lastHeight = height
}

func (t *stubTab) Activate(reason ActivateReason) tea.Cmd {
	t.activated = true
	return nil
}

func (t *stubTab) Deactivate() {
	t.deactivated = true
}

func newTestApp() (*DashboardApp, []*stubTab) {
	stubs := []*stubTab{
		newStubTab(TabSession, "Session"),
		newStubTab(TabWorkspaces, "Workspaces"),
		newStubTab(TabSettings, "Settings"),
		newStubTab(TabHelp, "Help"),
	}
	tabImpls := make([]Tab, len(stubs))
	for i, s := range stubs {
		tabImpls[i] = s
	}

	services := Services{
		Styles: styles.DefaultStyles(),
	}
	app := NewDashboardApp(services, tabImpls, TabSession)
	return app, stubs
}

func sendKey(app *DashboardApp, keyStr string) *DashboardApp {
	msg := tkey.Type(keyStr)
	switch keyStr {
	case "esc":
		msg = tkey.Esc()
	case "ctrl+c":
		msg = tkey.Ctrl('c')
	case "tab":
		msg = tkey.Tab()
	case "shift+tab":
		msg = tkey.ShiftTab()
	}

	result, _ := app.Update(msg)
	return result.(*DashboardApp)
}

func TestNewDashboardApp(t *testing.T) {
	app, stubs := newTestApp()

	if app.activeTab != TabSession {
		t.Errorf("expected active tab Current, got %s", app.activeTab)
	}
	if len(app.tabs) != 4 {
		t.Errorf("expected 4 tabs, got %d", len(app.tabs))
	}
	if len(app.tabOrder) != 4 {
		t.Errorf("expected 4 tab order, got %d", len(app.tabOrder))
	}
	_ = stubs
}

func TestDashboardInit(t *testing.T) {
	app, stubs := newTestApp()
	app.Init()

	for _, s := range stubs {
		if !s.initCalled {
			t.Errorf("expected Init called on tab %s", s.id)
		}
	}

	// First tab should be activated.
	if !stubs[0].activated {
		t.Error("expected current tab to be activated on init")
	}
}

func TestDashboardResize(t *testing.T) {
	app, stubs := newTestApp()

	result, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = result.(*DashboardApp)

	if app.width != 120 {
		t.Errorf("expected width 120, got %d", app.width)
	}
	if app.height != 40 {
		t.Errorf("expected height 40, got %d", app.height)
	}

	// All tabs should be resized.
	for _, s := range stubs {
		if !s.resized {
			t.Errorf("expected tab %s to be resized", s.id)
		}
	}
}

func TestDashboardTabSwitchByNumber(t *testing.T) {
	app, stubs := newTestApp()

	// Switch to sessions (tab 2) via Alt+2.
	app = sendKey(app, "alt+2")
	if app.activeTab != TabWorkspaces {
		t.Errorf("expected active tab Sessions, got %s", app.activeTab)
	}
	if !stubs[0].deactivated {
		t.Error("expected current tab to be deactivated")
	}
	if !stubs[1].activated {
		t.Error("expected sessions tab to be activated")
	}

	// Switch to help (tab 4) via Alt+4.
	app = sendKey(app, "alt+4")
	if app.activeTab != TabHelp {
		t.Errorf("expected active tab Help, got %s", app.activeTab)
	}

	// Switch to current (tab 1) via Alt+1.
	app = sendKey(app, "alt+1")
	if app.activeTab != TabSession {
		t.Errorf("expected active tab Current, got %s", app.activeTab)
	}
}

func TestDashboardTabCycle(t *testing.T) {
	app, _ := newTestApp()

	// Cycle forward.
	app = sendKey(app, "tab")
	if app.activeTab != TabWorkspaces {
		t.Errorf("expected Sessions after tab, got %s", app.activeTab)
	}

	app = sendKey(app, "tab")
	if app.activeTab != TabSettings {
		t.Errorf("expected Settings after tab, got %s", app.activeTab)
	}

	app = sendKey(app, "tab")
	if app.activeTab != TabHelp {
		t.Errorf("expected Help after tab, got %s", app.activeTab)
	}

	// Should wrap around.
	app = sendKey(app, "tab")
	if app.activeTab != TabSession {
		t.Errorf("expected Current after tab wrap, got %s", app.activeTab)
	}
}

func TestDashboardTabCycleBackward(t *testing.T) {
	app, _ := newTestApp()

	// Cycle backward from sessions should wrap to help.
	app = sendKey(app, "shift+tab")
	if app.activeTab != TabHelp {
		t.Errorf("expected Help after shift+tab, got %s", app.activeTab)
	}
}

func TestDashboardEscQuits(t *testing.T) {
	app, _ := newTestApp()

	app = sendKey(app, "esc")
	if !app.Quitting {
		t.Error("expected Quitting after esc")
	}
}

func TestDashboardCtrlCQuits(t *testing.T) {
	app, _ := newTestApp()

	app = sendKey(app, "ctrl+c")
	if !app.Quitting {
		t.Error("expected Quitting after ctrl+c")
	}
}

// Modal-esc routing: when the active tab captures Esc (inline rename/search/
// confirm), Esc must reach the tab to cancel that mode, NOT close the dashboard.
func TestDashboardEscRoutedToCapturingTab(t *testing.T) {
	app, stubs := newTestApp()
	stubs[0].capturesEsc = true // active tab (Session) is in a capturing mode

	app = sendKey(app, "esc")

	if app.Quitting {
		t.Error("esc must not quit while the active tab captures escape")
	}
	if !stubs[0].sawEsc {
		t.Error("esc must be routed to the capturing active tab so it can cancel its mode")
	}
}

// Ctrl+C is a hard quit even when the active tab captures Esc.
func TestDashboardCtrlCQuitsEvenWhenTabCapturesEsc(t *testing.T) {
	app, stubs := newTestApp()
	stubs[0].capturesEsc = true

	app = sendKey(app, "ctrl+c")
	if !app.Quitting {
		t.Error("expected Quitting after ctrl+c regardless of capture state")
	}
}

func TestDashboardViewContainsTabBar(t *testing.T) {
	app, _ := newTestApp()
	app.width = 80
	app.height = 40
	app.rect = ComputeContentRect(80, 40)

	view := ansi.Strip(app.view())

	if !strings.Contains(view, "Workspaces") {
		t.Error("expected view to contain Workspaces tab label")
	}
	if !strings.Contains(view, "Settings") {
		t.Error("expected view to contain Settings tab label")
	}
	if !strings.Contains(view, "Session") {
		t.Error("expected view to contain Session tab label")
	}
}

func TestDashboardViewContainsActiveTabContent(t *testing.T) {
	app, _ := newTestApp()
	app.width = 80
	app.height = 40
	app.rect = ComputeContentRect(80, 40)

	view := app.view()

	if !strings.Contains(view, "content:session") {
		t.Error("expected view to contain session tab content")
	}
}

func TestDashboardViewContainsHelpBar(t *testing.T) {
	app, _ := newTestApp()
	app.width = 80
	app.height = 40
	app.rect = ComputeContentRect(80, 40)

	view := app.view()

	if !strings.Contains(view, "help:session") {
		t.Error("expected view to contain session help text")
	}
}

func TestDashboardViewQuittingEmpty(t *testing.T) {
	app, _ := newTestApp()
	app.Quitting = true

	view := app.view()
	if view != "" {
		t.Errorf("expected empty view when quitting, got %q", view)
	}
}

func TestDashboardViewTooSmall(t *testing.T) {
	app, _ := newTestApp()
	app.width = 40
	app.height = 10

	view := app.view()
	if !strings.Contains(view, "too small") {
		t.Error("expected too-small warning")
	}
}

func TestDashboardHandlesSwitchTabIntent(t *testing.T) {
	app, _ := newTestApp()

	result, _ := app.Update(SwitchTabIntent{Tab: TabSettings})
	app = result.(*DashboardApp)

	if app.activeTab != TabSettings {
		t.Errorf("expected Settings after SwitchTabIntent, got %s", app.activeTab)
	}
}

func TestDashboardHandlesSetStatusIntent(t *testing.T) {
	app, _ := newTestApp()

	result, _ := app.Update(SetStatusIntent{Text: "hello", IsError: false})
	app = result.(*DashboardApp)

	if app.statusText != "hello" {
		t.Errorf("expected status 'hello', got %q", app.statusText)
	}
	if app.statusIsError {
		t.Error("expected statusIsError false")
	}
}

// The flash must actually appear in View() output, not just set statusText.
// Regression for the bug-#4 live-test where the SetStatusIntent reached the
// dashboard but didn't render visibly.
func TestDashboardSetStatusIntentRendersInView(t *testing.T) {
	app, _ := newTestApp()
	app.width = 80
	app.height = 24
	app.rect = ComputeContentRect(80, 24)

	result, _ := app.Update(SetStatusIntent{Text: "rename failed: name taken", IsError: true})
	app = result.(*DashboardApp)

	view := ansi.Strip(app.view())
	if !strings.Contains(view, "rename failed: name taken") {
		t.Errorf("expected flash text in view, got:\n%s", view)
	}
}

func TestDashboardHandlesQuitIntent(t *testing.T) {
	app, _ := newTestApp()

	result, cmd := app.Update(QuitIntent{Action: "switch", Chosen: "dev"})
	app = result.(*DashboardApp)

	if !app.Quitting {
		t.Error("expected Quitting after QuitIntent")
	}
	if app.Action != "switch" {
		t.Errorf("expected action 'switch', got %q", app.Action)
	}
	if app.Chosen != "dev" {
		t.Errorf("expected chosen 'dev', got %q", app.Chosen)
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestDashboardSwitchTabClearsStatus(t *testing.T) {
	app, _ := newTestApp()
	app.statusText = "some status"

	app = sendKey(app, "alt+2")

	if app.statusText != "" {
		t.Errorf("expected status cleared on tab switch, got %q", app.statusText)
	}
}

func TestDashboardInvalidInitialTab(t *testing.T) {
	stubs := []*stubTab{
		newStubTab(TabWorkspaces, "Workspaces"),
	}
	tabImpls := []Tab{stubs[0]}

	services := Services{Styles: styles.DefaultStyles()}
	app := NewDashboardApp(services, tabImpls, "nonexistent")

	if app.activeTab != TabWorkspaces {
		t.Errorf("expected fallback to Sessions, got %s", app.activeTab)
	}
}

func TestContentRect(t *testing.T) {
	rect := ComputeContentRect(80, 40)
	if rect.Width != 76 {
		t.Errorf("expected width 76, got %d", rect.Width)
	}
	if rect.Height <= 0 {
		t.Errorf("expected positive height, got %d", rect.Height)
	}
}

func TestIsTooSmall(t *testing.T) {
	if !IsTooSmall(50, 10) {
		t.Error("expected 50x10 to be too small")
	}
	if IsTooSmall(80, 24) {
		t.Error("expected 80x24 to not be too small")
	}
}

func TestIsCompact(t *testing.T) {
	if !IsCompact(70, 20) {
		t.Error("expected 70x20 to be compact")
	}
	if IsCompact(100, 30) {
		t.Error("expected 100x30 to not be compact")
	}
}
