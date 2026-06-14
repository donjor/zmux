package tabs

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/donjor/zmux/internal/tui/tkey"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

// ── Test helpers ──

// newTestCurrentTab builds a CurrentTab backed by a memFS-backed workspace
// store and a tmux mock seeded with:
//
//   - current session: "dev" (workspace "dev") with 3 windows
//   - sibling session: "dev-2" (also in workspace "dev")
//   - sibling workspace: "api" (not rendered)
func newTestCurrentTab(t *testing.T) (*CurrentTab, *tmux.MockRunner, *workspace.Store) {
	t.Helper()

	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.DisplayMessageResult = "dev"

	now := time.Now()
	mock.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), Dir: "/home/user/work"},
		{Name: "dev-2", Windows: 1, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/work"},
		{Name: "api", Windows: 2, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/api"},
	}
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true, Dir: "/home/user/work"},
			{Index: 2, Name: "server", Active: false, Dir: "/home/user/work"},
			{Index: 3, Name: "git", Active: false, Dir: "/home/user/work"},
		},
		"dev-2": {
			{Index: 1, Name: "shell", Active: true, Dir: "/home/user/work"},
		},
		"api": {
			{Index: 1, Name: "main", Active: true, Dir: "/home/user/api"},
		},
	}
	mock.Panes = map[string][]tmux.Pane{
		"dev": {
			{ID: "%11", Index: 1, WindowIndex: 1, Active: true, Command: "nvim", PID: 1234, Dir: "/home/user/work", Width: 80, Height: 24, Title: "editor-pane"},
			{ID: "%12", Index: 1, WindowIndex: 2, Active: true, Command: "node", PID: 1235, Dir: "/home/user/work", Width: 80, Height: 24, Title: "server-pane"},
			{ID: "%13", Index: 1, WindowIndex: 3, Active: true, Command: "bash", PID: 1236, Dir: "/home/user/work", Width: 80, Height: 24, Title: "git-pane"},
		},
	}

	fs := newSessionsMemFS("/home/user")
	store := workspace.NewStore(fs)
	if err := store.CreateWorkspace("dev", ""); err != nil {
		t.Fatalf("create dev: %v", err)
	}
	if err := store.CreateWorkspace("api", ""); err != nil {
		t.Fatalf("create api: %v", err)
	}
	addLegacyWorkspaceSession(t, store, "dev", "dev")
	addLegacyWorkspaceSession(t, store, "dev", "dev-2")
	addLegacyWorkspaceSession(t, store, "api", "api")

	loader := func() []workspaceview.WorkspaceViewModel {
		ws, _ := store.ListWorkspaces()
		live, _ := session.ListSessions(mock)
		return workspaceview.BuildWorkspaceViewModels(ws, live)
	}

	tab := NewCurrentTab(mock, styles.DefaultStyles(), loader, store)
	tab.Resize(80, 40)
	return tab, mock, store
}

func simulateCurrentActivate(tab *CurrentTab) *CurrentTab {
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd == nil {
		return tab
	}
	msg := cmd()
	if msg == nil {
		return tab
	}
	result, _ := tab.Update(msg)
	return result.(*CurrentTab)
}

// runCurrentMutationCmd drives a returned command + follow-on refetch chain.
func runCurrentMutationCmd(tab *CurrentTab, cmd tea.Cmd) *CurrentTab {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return tab
		}
		result, next := tab.Update(msg)
		tab = result.(*CurrentTab)
		cmd = next
	}
	return tab
}

func sendCurrentKey(tab *CurrentTab, keyStr string) (*CurrentTab, tea.Cmd) {
	var msg tea.KeyMsg
	switch keyStr {
	case "enter":
		msg = tkey.Enter()
	case "esc":
		msg = tkey.Esc()
	case "up":
		msg = tkey.Up()
	case "down":
		msg = tkey.Down()
	default:
		msg = tkey.Type(keyStr)
	}
	result, cmd := tab.Update(msg)
	return result.(*CurrentTab), cmd
}

// findCurrentRowByID returns the row index for the given outline ID, or -1.
func findCurrentRowByID(tab *CurrentTab, id string) int {
	for i := range tab.tree.Rows {
		if tab.tree.Rows[i].ID == id {
			return i
		}
	}
	return -1
}

// ── Identity ──

func TestCurrentTabID(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	if tab.ID() != dashboard.TabSession {
		t.Errorf("expected TabSession, got %s", tab.ID())
	}
}

func TestCurrentTabTitle(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	if tab.Title() != "Session & Workspace" {
		t.Errorf("expected 'Session & Workspace', got %q", tab.Title())
	}
}

// ── Activation + row building ──

func TestCurrentTabActivateLoadsData(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	if tab.sessionName != "dev" {
		t.Errorf("expected session 'dev', got %q", tab.sessionName)
	}
	if tab.wsName != "dev" {
		t.Errorf("expected workspace 'dev', got %q", tab.wsName)
	}
	if len(tab.windows) != 3 {
		t.Errorf("expected 3 windows, got %d", len(tab.windows))
	}
	if len(tab.siblings) != 1 {
		t.Errorf("expected 1 sibling session, got %d", len(tab.siblings))
	}
}

func TestCurrentTabRowCount(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)
	// Layout (all tabs expanded):
	//   workspace banner + sep + current session + 3 windows + 3 panes + sep +
	//   sibling session + 1 sibling window = 12 rows
	if got := len(tab.tree.Rows); got != 12 {
		t.Errorf("expected 12 rows, got %d: %+v", got, tab.tree.Rows)
	}
}

func TestCurrentTabRowShape(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	if idx := findCurrentRowByID(tab, outline.WorkspaceID("dev")); idx < 0 {
		t.Error("expected workspace header row")
	}
	if idx := findCurrentRowByID(tab, outline.SessionID("dev")); idx < 0 {
		t.Error("expected current-session row")
	}
	if idx := findCurrentRowByID(tab, outline.SessionID("dev-2")); idx < 0 {
		t.Error("expected sibling session row")
	}
	for _, idx := range []int{1, 2, 3} {
		id := outline.WindowID("dev", idx)
		if findCurrentRowByID(tab, id) < 0 {
			t.Errorf("expected window row %s", id)
		}
	}
}

// ── Navigation ──

func TestCurrentTabNavigateSkipsNonSelectable(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Jump to top should land on the workspace header.
	tab, _ = sendCurrentKey(tab, "g")
	row := tab.tree.Current()
	if row == nil || row.ID != outline.WorkspaceID("dev") {
		t.Errorf("expected workspace header, got %+v", row)
	}

	// Down once → current session row.
	tab, _ = sendCurrentKey(tab, "j")
	row = tab.tree.Current()
	if row == nil || row.ID != outline.SessionID("dev") {
		t.Errorf("expected current session, got %+v", row)
	}
}

// ── Enter ──

func TestCurrentTabEnterOnWindowFocuses(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Cursor onto the first window row.
	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	if idx < 0 {
		t.Fatal("window:dev:1 row not found")
	}
	tab.tree.Cursor = idx

	_, cmd := sendCurrentKey(tab, "enter")
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	if intent.Action != "focus" {
		t.Errorf("expected 'focus', got %q", intent.Action)
	}
}

func TestCurrentTabEnterOnSiblingSwitches(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.SessionID("dev-2"))
	if idx < 0 {
		t.Fatal("dev-2 sibling row not found")
	}
	tab.tree.Cursor = idx

	_, cmd := sendCurrentKey(tab, "enter")
	if cmd == nil {
		t.Fatal("expected command from enter")
	}
	intent, ok := cmd().(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent")
	}
	if intent.Action != "switch" || intent.Chosen != "dev-2" {
		t.Errorf("expected switch to dev-2, got %+v", intent)
	}
}

// ── Rename ──

func TestCurrentTabRenameWindow(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 2))
	if idx < 0 {
		t.Fatal("window:dev:2 row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}
	if tab.rename == nil || tab.rename.kind != "window" {
		t.Fatalf("expected rename kind=window, got %+v", tab.rename)
	}
	tab.renameInput.SetValue("renamed")

	_, cmd := sendCurrentKey(tab, "enter")
	_ = runCurrentMutationCmd(tab, cmd)

	if !currentMockCalled(mock, "RenameWindow") {
		t.Error("expected RenameWindow to be called")
	}
}

func TestCurrentTabRenameSiblingSession(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.SessionID("dev-2"))
	if idx < 0 {
		t.Fatal("dev-2 row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename || tab.rename == nil || tab.rename.kind != "session" {
		t.Fatalf("expected session rename, got mode=%d rename=%+v", tab.mode, tab.rename)
	}
	tab.renameInput.SetValue("dev-renamed")

	prepareMockForRename(mock, "dev-2", workspace.RawSessionName("dev", "dev-renamed"), "dev-renamed")

	_, cmd := sendCurrentKey(tab, "enter")
	_ = runCurrentMutationCmd(tab, cmd)

	if !currentMockCalled(mock, "RenameSession") {
		t.Error("expected RenameSession to be called")
	}
}

func TestCurrentTabRenameWorkspaceJumpsToNewID(t *testing.T) {
	tab, _, store := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WorkspaceID("dev"))
	if idx < 0 {
		t.Fatal("workspace row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename || tab.rename == nil || tab.rename.kind != "workspace" {
		t.Fatalf("expected workspace rename, got mode=%d rename=%+v", tab.mode, tab.rename)
	}
	tab.renameInput.SetValue("devv2")

	tab, cmd := sendCurrentKey(tab, "enter")
	tab = runCurrentMutationCmd(tab, cmd)

	if _, err := store.GetWorkspace("devv2"); err != nil {
		t.Fatalf("expected workspace devv2: %v", err)
	}
	// Cursor should land on the new workspace header after refetch.
	if row := tab.tree.Current(); row == nil || row.ID != outline.WorkspaceID("devv2") {
		t.Errorf("expected cursor on devv2 header, got %+v", row)
	}
}

// Renaming a workspace to a name that already exists must surface an
// error flash instead of silently no-op'ing. Regression for the "rename
// feels fragile" report.
func TestCurrentTabRenameWorkspaceConflictFlashesError(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}
	tab.renameInput.SetValue("api") // already exists

	_, cmd := sendCurrentKey(tab, "enter")
	if cmd == nil {
		t.Fatal("expected a command from the rename")
	}
	msg := cmd()
	intent, ok := msg.(dashboard.SetStatusIntent)
	if !ok {
		t.Fatalf("expected SetStatusIntent, got %T (%+v)", msg, msg)
	}
	if !intent.IsError {
		t.Error("expected error flash for name conflict")
	}
	if !strings.Contains(intent.Text, "rename workspace failed") {
		t.Errorf("expected 'rename workspace failed' in flash, got %q", intent.Text)
	}
}

func TestCurrentTabRenameCancel(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	tab, _ = sendCurrentKey(tab, "esc")

	if tab.mode != currentModeList {
		t.Errorf("expected list mode after esc, got %d", tab.mode)
	}
}

// ── Kill ──

func TestCurrentTabKillWindow(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 3))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "x")
	if tab.mode != currentModeConfirmKill {
		t.Fatalf("expected confirm mode, got %d", tab.mode)
	}
	if tab.confirm == nil || tab.confirm.kind != "window" {
		t.Fatalf("expected window confirm, got %+v", tab.confirm)
	}

	tab, cmd := sendCurrentKey(tab, "y")
	_ = runCurrentMutationCmd(tab, cmd)

	if !currentMockCalled(mock, "KillWindow") {
		t.Error("expected KillWindow to be called")
	}
}

func TestCurrentTabKillWorkspaceDoubleConfirm(t *testing.T) {
	tab, mock, store := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "x")
	if tab.mode != currentModeConfirmKill {
		t.Fatalf("expected first confirm, got %d", tab.mode)
	}

	// First 'y' → escalate (dev workspace has an attached session).
	tab, _ = sendCurrentKey(tab, "y")
	if tab.mode != currentModeConfirmKillAttached {
		t.Fatalf("expected attached confirm, got %d", tab.mode)
	}

	tab, cmd := sendCurrentKey(tab, "y")
	_ = runCurrentMutationCmd(tab, cmd)

	if !currentMockCalled(mock, "KillSession") {
		t.Error("expected KillSession to be called")
	}
	if ws, _ := store.GetWorkspace("dev"); ws != nil {
		t.Error("expected dev workspace to be deleted")
	}
}

// Killing the currently-attached session must switch the client to a
// sibling first; otherwise tmux drops the client and the dashboard popup
// dies mid-action.
func TestCurrentTabKillCurrentSessionSwitchesToSibling(t *testing.T) {
	tab, mock, store := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.SessionID("dev"))
	if idx < 0 {
		t.Fatalf("dev session row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "x")
	if tab.confirm == nil || tab.confirm.kind != "session" || tab.confirm.name != "dev" {
		t.Fatalf("expected session confirm for dev, got %+v", tab.confirm)
	}
	tab, cmd := sendCurrentKey(tab, "y")
	_ = runCurrentMutationCmd(tab, cmd)

	switchedFirst, killed := false, false
	for _, c := range mock.Calls {
		if c.Method == "SwitchClient" && len(c.Args) > 0 && c.Args[0] == "dev-2" && !killed {
			switchedFirst = true
		}
		if c.Method == "KillSession" && len(c.Args) > 0 && c.Args[0] == "dev" {
			killed = true
		}
	}
	if !switchedFirst {
		t.Errorf("expected SwitchClient(dev-2) before KillSession(dev), calls: %v", mock.Calls)
	}
	if !killed {
		t.Errorf("expected KillSession(dev), calls: %v", mock.Calls)
	}

	ws, _ := store.GetWorkspace("dev")
	if ws == nil || ws.LastActiveSession != "dev-2" {
		got := ""
		if ws != nil {
			got = ws.LastActiveSession
		}
		t.Errorf("expected last-active=dev-2 after fallback switch, got %q", got)
	}
}

// Killing a sibling (non-current) session must not switch the client.
func TestCurrentTabKillSiblingSessionDoesNotSwitch(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.SessionID("dev-2"))
	if idx < 0 {
		t.Fatalf("dev-2 sibling session row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "x")
	tab, cmd := sendCurrentKey(tab, "y")
	_ = runCurrentMutationCmd(tab, cmd)

	if currentMockCalled(mock, "SwitchClient") {
		t.Error("sibling kill should not switch the client")
	}
	if !currentMockCalled(mock, "KillSession") {
		t.Error("expected KillSession to be called")
	}
}

// A current session with no siblings cannot be killed from the dashboard —
// it would drop the client. Expect a status flash and no KillSession.
func TestCurrentTabKillOnlySessionBlocked(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	// Strip dev-2 so dev is the only session in the workspace.
	tab.siblings = nil
	tab.wsName = "dev"
	tab.sessionName = "dev"

	tab.confirm = &confirmState{kind: "session", name: "dev"}
	tab.mode = currentModeConfirmKill

	_, cmd := sendCurrentKey(tab, "y")
	if cmd == nil {
		t.Fatal("expected a command from the confirm")
	}
	msg := cmd()

	intent, ok := msg.(dashboard.SetStatusIntent)
	if !ok {
		t.Fatalf("expected SetStatusIntent flash, got %T", msg)
	}
	if !intent.IsError {
		t.Error("expected error flash")
	}
	if !strings.Contains(intent.Text, "only session") {
		t.Errorf("expected 'only session' in flash, got %q", intent.Text)
	}
	if currentMockCalled(mock, "KillSession") {
		t.Error("only-session kill must not call KillSession")
	}
}

// ── New ──

func TestCurrentTabNewWindowOnWindowRow(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	tab.tree.Cursor = idx

	_, cmd := sendCurrentKey(tab, "n")
	if cmd == nil {
		t.Fatal("expected command from n")
	}
	msg := cmd()
	if msg != nil {
		tab.Update(msg)
	}

	if !currentMockCalled(mock, "NewWindow") {
		t.Error("expected NewWindow to be called")
	}
}

func TestCurrentTabNewSessionInWorkspace(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "n")
	if tab.mode != currentModeCreate {
		t.Fatalf("expected create mode, got %d", tab.mode)
	}
	tab.createInput.SetValue("brand-new")

	tab, cmd := sendCurrentKey(tab, "enter")
	_ = runCurrentMutationCmd(tab, cmd)

	// session.Create calls NewSession under the hood via the runner.
	found := false
	for _, c := range mock.Calls {
		if strings.Contains(c.Method, "NewSession") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected NewSession to be called")
	}
}

// ── Move window ──

func TestCurrentTabMoveWindowEntersMoveMode(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 2))
	tab.tree.Cursor = idx

	_, cmd := sendCurrentKey(tab, "m")
	if cmd == nil {
		t.Fatal("expected command from m")
	}
	msg := cmd()
	result, _ := tab.Update(msg)
	tab = result.(*CurrentTab)

	if tab.mode != currentModeMoveWindow {
		t.Errorf("expected move mode, got %d", tab.mode)
	}
}

func TestCurrentTabMoveCancelOnEsc(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)
	tab.mode = currentModeMoveWindow
	tab.moveTargets = []currentMoveTarget{{Name: "other", Windows: 1}}

	tab, _ = sendCurrentKey(tab, "esc")
	if tab.mode != currentModeList {
		t.Errorf("expected list mode after esc, got %d", tab.mode)
	}
}

// ── Reorder ──

func TestCurrentTabSwapWindow(t *testing.T) {
	tab, mock, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	tab.tree.Cursor = idx

	_, cmd := sendCurrentKey(tab, ">")
	if cmd == nil {
		t.Fatal("expected command from >")
	}
	_ = cmd()

	if !currentMockCalled(mock, "SwapWindow") {
		t.Error("expected SwapWindow to be called")
	}
}

// ── Modal staging (Codex #4) ──

func TestCurrentTabStagesDataDuringRename(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	tab.tree.Cursor = idx

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}

	rowsBefore := len(tab.tree.Rows)

	// Inject a new data msg while in rename mode — must be staged.
	staged := currentDataMsg{
		reqID:       tab.reqID,
		wsName:      "dev",
		sessionName: "dev",
		windows:     []windowDetail{},
	}
	result, _ := tab.Update(staged)
	tab = result.(*CurrentTab)

	if tab.pending == nil {
		t.Error("expected staged data while in modal")
	}
	if len(tab.tree.Rows) != rowsBefore {
		t.Error("expected tree rows unchanged during modal")
	}

	// Exit rename and staged data should apply.
	tab, _ = sendCurrentKey(tab, "esc")
	if tab.pending != nil {
		t.Error("expected pending to clear after exit")
	}
}

// ── Stale reqID dropping (Codex #9) ──

func TestCurrentTabDropsStaleAfterDeactivate(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	staleID := tab.reqID
	tab.Deactivate()

	rowsBefore := len(tab.tree.Rows)
	stale := currentDataMsg{
		reqID:       staleID,
		wsName:      "ghost",
		sessionName: "ghost",
	}
	result, _ := tab.Update(stale)
	tab = result.(*CurrentTab)

	if len(tab.tree.Rows) != rowsBefore {
		t.Errorf("expected unchanged rows after stale msg, got %d (was %d)", len(tab.tree.Rows), rowsBefore)
	}
}

// ── Deactivate resets overlays ──

func TestCurrentTabDeactivateDropsOverlays(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	tab.mode = currentModeConfirmKill
	tab.confirm = &confirmState{kind: "window", name: "editor", windowIndex: 1}

	tab.Deactivate()

	if tab.mode != currentModeList {
		t.Errorf("expected list mode, got %d", tab.mode)
	}
	if tab.confirm != nil {
		t.Error("expected confirm state cleared")
	}
}

// ── ShortHelp ──

func TestCurrentTabShortHelp(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Cursor onto a window row → window-flavoured help.
	idx := findCurrentRowByID(tab, outline.WindowID("dev", 1))
	tab.tree.Cursor = idx
	help := tab.ShortHelp()
	if !strings.Contains(help, "enter:focus") {
		t.Errorf("expected 'enter:focus' in window help, got %q", help)
	}
	if !strings.Contains(help, "</>:reorder") {
		t.Errorf("expected '</>:reorder' in window help, got %q", help)
	}

	// Cursor onto workspace header → workspace-flavoured help.
	idx = findCurrentRowByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = idx
	help = tab.ShortHelp()
	if !strings.Contains(help, "r:rename") {
		t.Errorf("expected 'r:rename' in workspace help, got %q", help)
	}
}

func TestCurrentTabNoActiveSessionState(t *testing.T) {
	tab := NewCurrentTab(tmux.NewMockRunner(), styles.DefaultStyles(), nil, nil)
	tab.Resize(80, 24)

	view := tab.View()
	if !strings.Contains(view, "No active session") {
		t.Fatalf("expected no-active-session view, got:\n%s", view)
	}
	if !strings.Contains(view, "Press n") {
		t.Fatalf("expected create affordance in no-session view, got:\n%s", view)
	}
	if help := tab.ShortHelp(); !strings.Contains(help, "n:new tmp") || !strings.Contains(help, "esc:exit") {
		t.Fatalf("unexpected no-session help: %q", help)
	}
}

func TestCurrentTabNoActiveSessionNewQuitsWithIntent(t *testing.T) {
	tab := NewCurrentTab(tmux.NewMockRunner(), styles.DefaultStyles(), nil, nil)

	_, cmd := sendCurrentKey(tab, "n")
	if cmd == nil {
		t.Fatal("expected n to emit a dashboard quit intent")
	}
	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	if intent.Action != "new" {
		t.Fatalf("QuitIntent action = %q, want new", intent.Action)
	}
}

// ── View smoke ──

func TestCurrentTabViewRendersTree(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	view := tab.View()
	for _, want := range []string{"dev", "editor", "server", "git", "dev-2"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestCurrentTabViewShowsWindowCount(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	view := tab.View()
	if !strings.Contains(view, "3 tabs") {
		t.Error("expected view to contain '3 tabs'")
	}
}

func TestCurrentTabViewShowsPaneRows(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)
	_, _ = tab.enterWindowLevel()

	view := tab.View()
	for _, want := range []string{"%11", "editor-pane", "80x24"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected pane row to contain %q, got:\n%s", want, view)
		}
	}
}

// ── isIdleWindow ──

func TestIsIdleWindow(t *testing.T) {
	w := windowDetail{
		Panes: []tmux.Pane{{Active: true, Command: "bash"}},
		Stats: tmux.ProcessStats{CPU: 0.0},
	}
	if !isIdleWindow(w) {
		t.Error("expected bash with 0 CPU to be idle")
	}

	w.Stats.CPU = 2.0
	if isIdleWindow(w) {
		t.Error("expected bash with CPU to not be idle")
	}

	w2 := windowDetail{
		Panes: []tmux.Pane{{Active: true, Command: "nvim"}},
		Stats: tmux.ProcessStats{CPU: 0.0},
	}
	if isIdleWindow(w2) {
		t.Error("expected nvim to not be idle")
	}
}

// ── Search / filter (P5) ──

// typeCurrentSearch enters search mode via "/" (when not already there) and
// types each rune of query as a separate keystroke, driving the live-filter
// path through handleSearchKey exactly as a real user would.
func typeCurrentSearch(tab *CurrentTab, query string) *CurrentTab {
	if tab.mode != currentModeSearch {
		tab, _ = sendCurrentKey(tab, "/")
	}
	for _, r := range query {
		tab, _ = sendCurrentKey(tab, string(r))
	}
	return tab
}

func TestCurrentTabSearchEntersMode(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "/")
	if tab.mode != currentModeSearch {
		t.Fatalf("expected search mode, got %d", tab.mode)
	}
	if !tab.CapturesEscape() {
		t.Error("expected CapturesEscape true while in search mode")
	}
}

func TestCurrentTabSearchFiltersBySessionName(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// "dev-2" matches only the sibling session, not the current "dev".
	tab = typeCurrentSearch(tab, "dev-2")

	if findCurrentRowByID(tab, outline.SessionID("dev-2")) < 0 {
		t.Error("expected matching session dev-2 to show")
	}
	if findCurrentRowByID(tab, outline.SessionID("dev")) >= 0 {
		t.Error("expected non-matching current session dev to be hidden")
	}
}

func TestCurrentTabSearchFiltersByWindowName(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// "editor" is a window of the current session "dev"; "dev-2" only has a
	// "shell" window, so it drops out.
	tab = typeCurrentSearch(tab, "editor")

	if findCurrentRowByID(tab, outline.SessionID("dev")) < 0 {
		t.Error("expected current session dev kept for its matching window")
	}
	if findCurrentRowByID(tab, outline.SessionID("dev-2")) >= 0 {
		t.Error("expected dev-2 filtered out by window-name query 'editor'")
	}
}

func TestCurrentTabSearchEscCancelsFilter(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	tab = typeCurrentSearch(tab, "dev-2")
	tab, _ = sendCurrentKey(tab, "esc")

	if tab.mode != currentModeList {
		t.Errorf("expected list mode after esc, got %d", tab.mode)
	}
	if tab.searchQuery != "" {
		t.Errorf("expected filter cleared, got %q", tab.searchQuery)
	}
	if findCurrentRowByID(tab, outline.SessionID("dev")) < 0 {
		t.Error("expected current session restored after clearing filter")
	}
}

func TestCurrentTabSearchEnterCommitsFilter(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	tab = typeCurrentSearch(tab, "dev-2")
	tab, _ = sendCurrentKey(tab, "enter")

	if tab.mode != currentModeList {
		t.Errorf("expected list mode after enter, got %d", tab.mode)
	}
	if tab.searchQuery != "dev-2" {
		t.Errorf("expected committed filter dev-2, got %q", tab.searchQuery)
	}
	if !tab.CapturesEscape() {
		t.Error("expected CapturesEscape true while a committed filter is active")
	}

	// A second esc in list mode clears the committed filter.
	tab, _ = sendCurrentKey(tab, "esc")
	if tab.searchQuery != "" {
		t.Errorf("expected filter cleared by list-mode esc, got %q", tab.searchQuery)
	}
}

// ── Numbered quick-jump (P5) ──

func TestCurrentTabDigitJumpsToNthSession(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// #1 = current "dev", #2 = sibling "dev-2". Pressing 2 switches to dev-2.
	_, cmd := sendCurrentKey(tab, "2")
	intent := currentQuitIntent(t, cmd)
	if intent.Action != "switch" || intent.Chosen != "dev-2" {
		t.Fatalf("digit 2: got action=%q chosen=%q, want switch/dev-2", intent.Action, intent.Chosen)
	}
}

func TestCurrentTabDigitJumpsToCurrentSession(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// #1 is the current session → focus (no switch).
	_, cmd := sendCurrentKey(tab, "1")
	intent := currentQuitIntent(t, cmd)
	if intent.Action != "focus" || intent.Chosen != "dev" {
		t.Fatalf("digit 1: got action=%q chosen=%q, want focus/dev", intent.Action, intent.Chosen)
	}
}

func TestCurrentTabDigitRespectsFilter(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Filter to dev-2 only, then commit. It becomes the sole visible session,
	// so digit 1 targets it.
	tab = typeCurrentSearch(tab, "dev-2")
	tab, _ = sendCurrentKey(tab, "enter")

	_, cmd := sendCurrentKey(tab, "1")
	intent := currentQuitIntent(t, cmd)
	if intent.Action != "switch" || intent.Chosen != "dev-2" {
		t.Fatalf("digit 1 under filter: got action=%q chosen=%q, want switch/dev-2", intent.Action, intent.Chosen)
	}
}

func TestCurrentTabDigitOutOfRangeNoOp(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Only 2 sessions exist; digit 9 is a no-op (no quit intent, no panic).
	_, cmd := sendCurrentKey(tab, "9")
	if cmd != nil {
		t.Fatalf("expected no command for out-of-range digit, got one yielding %T", cmd())
	}
}

// Clearing a committed filter from window-level nav must leave a sane state:
// the full session list returns and the cursor stays on a selectable row
// (regression guard for the filter + window-level + Esc transition).
func TestCurrentTabClearFilterFromWindowLevel(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	// Filter to keep the current session (matches its "editor" window), commit.
	tab = typeCurrentSearch(tab, "editor")
	tab, _ = sendCurrentKey(tab, "enter")

	// Descend into window-level nav on the current session.
	tab.tree.JumpToID(outline.SessionID("dev"))
	_, _ = tab.enterWindowLevel()
	if tab.navLevel != navLevelWindow {
		t.Fatal("expected to descend to window level")
	}

	// Esc clears the committed filter from window level.
	tab, _ = sendCurrentKey(tab, "esc")
	if tab.searchQuery != "" {
		t.Errorf("expected filter cleared, got %q", tab.searchQuery)
	}
	if findCurrentRowByID(tab, outline.SessionID("dev-2")) < 0 {
		t.Error("expected dev-2 restored after clearing the filter")
	}
	if cur := tab.tree.Current(); cur == nil || !cur.Selectable {
		t.Error("expected cursor on a selectable row after clearing the filter")
	}
}

// ── Scope cue (P5) ──

func TestCurrentTabScopeCueAboveRows(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	view := tab.View()
	cue := "2 sessions in dev"
	idxCue := strings.Index(view, cue)
	if idxCue < 0 {
		t.Fatalf("expected scope cue %q in view, got:\n%s", cue, view)
	}
	// The cue must sit above the banner's horizontal rule (i.e. in the pinned
	// chrome, not in the scrolling rows).
	idxRule := strings.Index(view, "───")
	if idxRule < 0 || idxCue >= idxRule {
		t.Errorf("expected scope cue above the banner rule (cue@%d, rule@%d)", idxCue, idxRule)
	}
}

func TestCurrentTabScopeCueShowsFilterChip(t *testing.T) {
	tab, _, _ := newTestCurrentTab(t)
	tab = simulateCurrentActivate(tab)

	tab = typeCurrentSearch(tab, "dev-2")
	tab, _ = sendCurrentKey(tab, "enter")

	view := tab.View()
	if !strings.Contains(view, "filter: dev-2") {
		t.Errorf("expected committed filter chip in view, got:\n%s", view)
	}
}

// ── Helpers ──

func currentQuitIntent(t *testing.T, cmd tea.Cmd) dashboard.QuitIntent {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	return intent
}

func currentMockCalled(mock *tmux.MockRunner, method string) bool {
	for _, c := range mock.Calls {
		if c.Method == method {
			return true
		}
	}
	return false
}
