package tabs

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/donjor/zmux/internal/tui/tkey"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

// noopOvermind is an inert overmind.Client for tab tests that don't exercise
// the overmind restart/stop paths.
type noopOvermind struct{}

func (noopOvermind) Connect(string, string) error        { return nil }
func (noopOvermind) Restart(string, string) error        { return nil }
func (noopOvermind) Stop(string, string) error           { return nil }
func (noopOvermind) StopAll(string) error                { return nil }
func (noopOvermind) Logs(string, string) (string, error) { return "", nil }

// ── memFS for an in-process workspace store ──

type sessionsMemFS struct {
	files   map[string][]byte
	homeDir string
}

func newSessionsMemFS(home string) *sessionsMemFS {
	return &sessionsMemFS{files: make(map[string][]byte), homeDir: home}
}

func (m *sessionsMemFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *sessionsMemFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *sessionsMemFS) MkdirAll(_ string, _ os.FileMode) error { return nil }
func (m *sessionsMemFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return sessionsFakeInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}
func (m *sessionsMemFS) UserHomeDir() (string, error) { return m.homeDir, nil }
func (m *sessionsMemFS) Glob(_ string) ([]string, error) {
	return nil, nil
}

type sessionsFakeInfo struct{ name string }

func (f sessionsFakeInfo) Name() string       { return f.name }
func (f sessionsFakeInfo) Size() int64        { return 0 }
func (f sessionsFakeInfo) Mode() os.FileMode  { return 0o644 }
func (f sessionsFakeInfo) ModTime() time.Time { return time.Time{} }
func (f sessionsFakeInfo) IsDir() bool        { return false }
func (f sessionsFakeInfo) Sys() any           { return nil }

// ── Test helpers ──

// newTestSessionsTab builds a tab backed by a real workspace.Store on memFS
// and a tmux mock seeded with three workspaces:
//
//   - dev    : 2 live sessions, one attached
//   - api    : 1 live session, no attached
//   - empty  : 0 live sessions
func newTestSessionsTab(t *testing.T) (*SessionsTab, *tmux.MockRunner, *workspace.Store) {
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
		"dev":   {{Index: 1, Name: "editor", Active: true}},
		"dev-2": {{Index: 1, Name: "shell", Active: true}},
		"api":   {{Index: 1, Name: "main", Active: true}},
	}

	fs := newSessionsMemFS("/home/user")
	store := workspace.NewStore(fs)
	if err := store.CreateWorkspace("dev", "/home/user/dev"); err != nil {
		t.Fatalf("create dev: %v", err)
	}
	if err := store.CreateWorkspace("api", ""); err != nil {
		t.Fatalf("create api: %v", err)
	}
	if err := store.CreateWorkspace("empty", ""); err != nil {
		t.Fatalf("create empty: %v", err)
	}
	addLegacyWorkspaceSession(t, store, "dev", "dev")
	addLegacyWorkspaceSession(t, store, "dev", "dev-2")
	addLegacyWorkspaceSession(t, store, "api", "api")

	loader := func() []workspaceview.WorkspaceViewModel {
		ws, _ := store.ListWorkspaces()
		live, _ := session.ListSessions(mock)
		return workspaceview.BuildWorkspaceViewModels(ws, live)
	}

	tab := NewSessionsTab(mock, styles.DefaultStyles(), loader, store, noopOvermind{})
	tab.Resize(80, 40)
	return tab, mock, store
}

func simulateActivate(tab *SessionsTab) *SessionsTab {
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd == nil {
		return tab
	}
	msg := cmd()
	if msg == nil {
		return tab
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if subMsg := sub(); subMsg != nil {
				result, _ := tab.Update(subMsg)
				tab = result.(*SessionsTab)
			}
		}
		// Tests don't depend on real external sources.
		tab.catalog = nil
		tab.tree.SetRows(tab.buildRows())
		return tab
	}
	result, _ := tab.Update(msg)
	tab = result.(*SessionsTab)
	tab.catalog = nil
	tab.tree.SetRows(tab.buildRows())
	return tab
}

// runMutationCmd runs a returned tea.Cmd, feeds the result back into the
// tab, and recursively drives any follow-on commands. Used for the rename /
// kill / move flows that produce a refetch chain.
func runMutationCmd(tab *SessionsTab, cmd tea.Cmd) *SessionsTab {
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return tab
		}
		result, next := tab.Update(msg)
		tab = result.(*SessionsTab)
		cmd = next
	}
	return tab
}

func sendKey(tab *SessionsTab, keyStr string) (*SessionsTab, tea.Cmd) {
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
	return result.(*SessionsTab), cmd
}

// findRowIndexByID returns the row index for the given outline ID, or -1.
func findRowIndexByID(tab *SessionsTab, id string) int {
	for i := range tab.tree.Rows {
		if tab.tree.Rows[i].ID == id {
			return i
		}
	}
	return -1
}

// ── Identity ──

func TestSessionsTabIDAndTitle(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	if tab.ID() != dashboard.TabWorkspaces {
		t.Errorf("expected TabWorkspaces, got %s", tab.ID())
	}
	if tab.Title() != "Workspaces" {
		t.Errorf("expected 'Workspaces', got %q", tab.Title())
	}
}

// ── Activation + row building ──

func TestSessionsTabActivateLoadsWorkspaces(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	if len(tab.workspaces) == 0 {
		t.Fatal("expected workspaces to load after activate")
	}
}

func TestSessionsTabCollapsedRowCount(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	// 3 workspaces, none expanded → 3 rows.
	if got := len(tab.tree.Rows); got != 3 {
		t.Errorf("expected 3 collapsed rows, got %d: %+v", got, tab.tree.Rows)
	}
}

func TestSessionsTabExpandShowsSessions(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	// Cursor must land on the dev workspace.
	devIdx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	if devIdx < 0 {
		t.Fatal("dev workspace row not found")
	}
	tab.tree.Cursor = devIdx
	tab, _ = sendKey(tab, "enter")
	// 3 workspaces + 2 sessions under dev = 5 rows.
	if got := len(tab.tree.Rows); got != 5 {
		t.Errorf("expected 5 rows after expand, got %d", got)
	}
}

func TestSessionsTabEmptyWorkspacePlaceholder(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	emptyIdx := findRowIndexByID(tab, outline.WorkspaceID("empty"))
	if emptyIdx < 0 {
		t.Fatal("empty workspace row not found")
	}
	tab.tree.Cursor = emptyIdx
	tab, _ = sendKey(tab, "enter")
	// 3 workspaces + 1 placeholder = 4 rows.
	if got := len(tab.tree.Rows); got != 4 {
		t.Errorf("expected 4 rows with empty placeholder, got %d", got)
	}
}

// ── Create workspace ──

func TestSessionsTabCreateWorkspace(t *testing.T) {
	tab, _, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	tab, _ = sendKey(tab, "C")
	if tab.mode != sessionsModeCreate {
		t.Fatalf("expected create mode, got %d", tab.mode)
	}
	tab.createInput.SetValue("brand-new")

	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	ws, _ := store.GetWorkspace("brand-new")
	if ws == nil {
		t.Fatal("expected workspace 'brand-new' to be created")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("brand-new")) < 0 {
		t.Error("expected new workspace row to appear after refetch")
	}
}

// TestSessionsTabCreateOnNoContextRowMakesWorkspace covers the empty/sessionless
// path: on a no-context row (the "no workspaces yet" placeholder, an external
// entry, a pseudo workspace) there is nothing to nest a session under, so c
// escalates to create-WORKSPACE exactly like C — matching the placeholder's
// "press C to create one" hint.
func TestSessionsTabCreateOnNoContextRowMakesWorkspace(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	placeholder := &outline.Row{ID: "placeholder:noworkspaces", Kind: outline.RowPlaceholder}
	if _, _ = tab.enterCreateSessionMode(placeholder); tab.mode != sessionsModeCreate {
		t.Fatalf("expected create mode, got %d", tab.mode)
	}
	if tab.createWsTarget != "" {
		t.Errorf("createWsTarget = %q, want empty (workspace-create, not session)", tab.createWsTarget)
	}
	if tab.createInput.Placeholder != "workspace name..." {
		t.Errorf("placeholder = %q, want %q", tab.createInput.Placeholder, "workspace name...")
	}
}

// ── Create session in workspace (Facet B) ──

// TestSessionsTabCreateSessionInWorkspace exercises the c=create-session flow on
// the Workspaces tab end to end: the ghost-prompt (mode + target + placeholder),
// the action (canonical tmux session + identity stamp, never a raw-label
// session), and the result (store record + force-expand reveals the new row).
func TestSessionsTabCreateSessionInWorkspace(t *testing.T) {
	tab, mock, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Collapse "dev" so force-expand-on-create is observable: its child session
	// rows must reappear only because create re-expands the workspace.
	tab.tree.SetExpanded(outline.WorkspaceID("dev"), false)
	tab.tree.SetRows(tab.buildRows())

	idx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	if idx < 0 {
		t.Fatal("dev workspace header not found")
	}
	tab.tree.Cursor = idx

	// Ghost-prompt: c opens the create-SESSION input targeting dev.
	tab, _ = sendKey(tab, "c")
	if tab.mode != sessionsModeCreate {
		t.Fatalf("expected create mode, got %d", tab.mode)
	}
	if tab.createWsTarget != "dev" {
		t.Fatalf("createWsTarget = %q, want dev", tab.createWsTarget)
	}
	if tab.createInput.Placeholder != "session name..." {
		t.Fatalf("placeholder = %q, want %q", tab.createInput.Placeholder, "session name...")
	}

	// Action: type a label and confirm.
	tab.createInput.SetValue("worker")
	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	canonical := workspace.RawSessionName("dev", "worker")

	// Action result 1: a canonically-named tmux session was created — NOT the
	// bare label (the old dashboard bug the picker/bar could not resolve) — in
	// the workspace's RootDir (the primary create-dir precedence).
	if !mockHasCall(mock, "NewSession", canonical, "/home/user/dev") {
		t.Errorf("expected NewSession(%s, \"/home/user/dev\"), got: %v", canonical, mock.Calls)
	}
	if mockHasCall(mock, "NewSession", "worker", "") {
		t.Error("created a raw-label session 'worker' — canonical identity regressed")
	}
	// Action result 2: identity metadata stamped on the canonical session.
	if !mockHasCall(mock, "SetSessionOption", canonical, workspace.OptionManaged, "1") {
		t.Errorf("expected managed-option stamp on %s, got: %v", canonical, mock.Calls)
	}

	// Result 3: the store records the session under dev.
	ws, _ := store.GetWorkspace("dev")
	if ws == nil {
		t.Fatal("dev workspace missing from store")
	}
	found := false
	for _, s := range ws.Sessions {
		if s.Label == "worker" && s.TmuxName == canonical {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session 'worker' (%s) recorded under dev, got %+v", canonical, ws.Sessions)
	}

	// Result 4: force-expand re-revealed dev's children — the new session row is
	// visible (and the cursor landed on it via the canonical jump target).
	if findRowIndexByID(tab, outline.SessionID(canonical)) < 0 {
		t.Error("new session row not visible — force-expand-on-create regressed")
	}
}

// ── Rename workspace ──

func TestSessionsTabRenameWorkspaceJumpsToNewID(t *testing.T) {
	tab, _, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	idx := findRowIndexByID(tab, outline.WorkspaceID("api"))
	if idx < 0 {
		t.Fatal("api workspace row not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendKey(tab, "r")
	if tab.mode != sessionsModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}
	tab.renameInput.SetValue("apiv2")

	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	if _, err := store.GetWorkspace("apiv2"); err != nil {
		t.Fatalf("expected workspace 'apiv2' to exist: %v", err)
	}
	row := tab.tree.Current()
	if row == nil || row.ID != outline.WorkspaceID("apiv2") {
		t.Errorf("expected cursor on apiv2, got %+v", row)
	}
}

// ── Rename session ──

func TestSessionsTabRenameSessionJumpsToNewID(t *testing.T) {
	tab, mock, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Expand dev so the session row is in the tree.
	devIdx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = devIdx
	tab, _ = sendKey(tab, "enter")

	// Cursor onto a dev session.
	sIdx := findRowIndexByID(tab, outline.SessionID("dev"))
	if sIdx < 0 {
		t.Fatal("dev session row not found")
	}
	tab.tree.Cursor = sIdx

	tab, _ = sendKey(tab, "r")
	if tab.mode != sessionsModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}
	tab.renameInput.SetValue("dev-renamed")

	// Update the mock so the post-mutation refetch reflects the rename.
	renamedRaw := workspace.RawSessionName("dev", "dev-renamed")
	prepareMockForRename(mock, "dev", renamedRaw, "dev-renamed")

	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	row := tab.tree.Current()
	if row == nil || row.ID != outline.SessionID(renamedRaw) {
		t.Errorf("expected cursor on dev-renamed, got %+v", row)
	}
}

func prepareMockForRename(mock *tmux.MockRunner, oldName, newName, label string) {
	for i := range mock.Sessions {
		if mock.Sessions[i].Name == oldName {
			mock.Sessions[i].Name = newName
			mock.Sessions[i].Managed = true
			mock.Sessions[i].Workspace = "dev"
			mock.Sessions[i].SessionLabel = label
		}
	}
	if wins, ok := mock.Windows[oldName]; ok {
		mock.Windows[newName] = wins
		delete(mock.Windows, oldName)
	}
}

// ── Kill session (single confirm) ──

func TestSessionsTabKillSession(t *testing.T) {
	tab, mock, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	devIdx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = devIdx
	tab, _ = sendKey(tab, "enter")

	sIdx := findRowIndexByID(tab, outline.SessionID("dev-2"))
	if sIdx < 0 {
		t.Fatal("dev-2 row not found")
	}
	tab.tree.Cursor = sIdx

	tab, _ = sendKey(tab, "x")
	if tab.mode != sessionsModeConfirmKill {
		t.Fatalf("expected confirm mode, got %d", tab.mode)
	}
	_, cmd := sendKey(tab, "y")
	_ = runMutationCmd(tab, cmd)

	if !mockCalled(mock, "KillSession") {
		t.Error("expected KillSession to be called")
	}
}

// ── Kill workspace (double confirm for attached) ──

func TestSessionsTabKillAttachedWorkspaceDoubleConfirm(t *testing.T) {
	tab, mock, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	idx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	if idx < 0 {
		t.Fatal("dev workspace not found")
	}
	tab.tree.Cursor = idx

	tab, _ = sendKey(tab, "x")
	if tab.mode != sessionsModeConfirmKill {
		t.Fatalf("expected first confirm, got %d", tab.mode)
	}

	// First "y" should escalate to the attached confirmation.
	tab, _ = sendKey(tab, "y")
	if tab.mode != sessionsModeConfirmKillAttached {
		t.Fatalf("expected attached confirm, got %d", tab.mode)
	}

	_, cmd := sendKey(tab, "y")
	_ = runMutationCmd(tab, cmd)

	if !mockCalled(mock, "KillSession") {
		t.Error("expected KillSession to be called for workspace sessions")
	}
	if ws, _ := store.GetWorkspace("dev"); ws != nil {
		t.Error("expected dev workspace to be deleted")
	}
}

// ── Move session inline ──

func TestSessionsTabMoveSession(t *testing.T) {
	tab, _, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Expand dev and place cursor on the dev-2 session.
	devIdx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = devIdx
	tab, _ = sendKey(tab, "enter")

	sIdx := findRowIndexByID(tab, outline.SessionID("dev-2"))
	if sIdx < 0 {
		t.Fatal("dev-2 row not found")
	}
	tab.tree.Cursor = sIdx

	tab, _ = sendKey(tab, "m")
	if tab.mode != sessionsModeMove {
		t.Fatalf("expected move mode, got %d", tab.mode)
	}

	// Snap should land on the dev workspace header.
	if row := tab.tree.Current(); row == nil || row.Kind != outline.RowWorkspaceHeader {
		t.Fatalf("expected cursor on workspace header, got %+v", row)
	}

	// Navigate to the api workspace header. Up to N tries in either direction
	// since the workspace order depends on activity sort.
	apiID := outline.WorkspaceID("api")
	if !navigateToWorkspaceInMoveMode(tab, apiID) {
		t.Fatal("could not navigate to api workspace")
	}

	_, cmd := sendKey(tab, "enter")
	_ = runMutationCmd(tab, cmd)

	apiWS, _ := store.GetWorkspace("api")
	if apiWS == nil {
		t.Fatal("api workspace missing")
	}
	found := false
	for _, s := range apiWS.Sessions {
		if s.Label == "dev-2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dev-2 to be in api workspace, got %v", apiWS.Sessions)
	}
}

// ── Modal staging (Codex #4) ──

func TestSessionsTabStagesDataDuringRename(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	idx := findRowIndexByID(tab, outline.WorkspaceID("dev"))
	tab.tree.Cursor = idx
	tab, _ = sendKey(tab, "r")
	if tab.mode != sessionsModeRename {
		t.Fatalf("expected rename mode, got %d", tab.mode)
	}

	rowsBefore := len(tab.tree.Rows)

	// Inject a new data message while in rename mode.
	staged := sessionsDataMsg{
		reqID:      tab.reqID,
		workspaces: []workspaceview.WorkspaceViewModel{{Workspace: workspace.Workspace{Name: "fresh"}}},
	}
	result, _ := tab.Update(staged)
	tab = result.(*SessionsTab)

	if tab.pending == nil {
		t.Error("expected staged data while in modal")
	}
	if len(tab.tree.Rows) != rowsBefore {
		t.Error("expected tree rows unchanged during modal")
	}

	// Exit rename and the staged data should apply.
	tab, _ = sendKey(tab, "esc")
	if tab.pending != nil {
		t.Error("expected pending to clear after exit")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("fresh")) < 0 {
		t.Error("expected staged workspace to be applied after exit")
	}
}

// ── Stale reqID dropping (Codex #9) ──

func TestSessionsTabDropsStaleAfterDeactivate(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	staleID := tab.reqID
	tab.Deactivate()

	// Stale message must not panic and must not mutate state.
	rowsBefore := len(tab.tree.Rows)
	stale := sessionsDataMsg{
		reqID: staleID,
		workspaces: []workspaceview.WorkspaceViewModel{
			{Workspace: workspace.Workspace{Name: "ghost"}},
		},
	}
	result, _ := tab.Update(stale)
	tab = result.(*SessionsTab)

	if len(tab.tree.Rows) != rowsBefore {
		t.Errorf("expected unchanged rows after stale msg, got %d (was %d)", len(tab.tree.Rows), rowsBefore)
	}
}

// ── Cursor preservation ──

func TestSessionsTabCursorAfterDelete(t *testing.T) {
	tab, _, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	idx := findRowIndexByID(tab, outline.WorkspaceID("empty"))
	tab.tree.Cursor = idx

	// Delete via the store directly, then refetch.
	if err := store.DeleteWorkspace("empty"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	tab.reqID = dashboard.NextReqID()
	tab = runMutationCmd(tab, tab.fetchData(tab.reqID))

	row := tab.tree.Current()
	if row == nil {
		t.Fatal("expected cursor on a row after delete")
	}
	if !row.Selectable && row.Kind != outline.RowPlaceholder {
		t.Errorf("expected cursor on a sensible row, got %+v", row)
	}
}

// ── ShortHelp ──

func TestSessionsTabShortHelpListMode(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	help := tab.ShortHelp()
	// The footer advertises the relettered create keys (c = session in the
	// workspace at the cursor, C = new workspace) — never the retired n:new.
	if !strings.Contains(help, "c:session") || !strings.Contains(help, "C:workspace") {
		t.Errorf("expected 'c:session' and 'C:workspace' in help, got %q", help)
	}
	if strings.Contains(help, "n:new") {
		t.Errorf("retired 'n:new' still present in help: %q", help)
	}
}

// ── View smoke test ──

func TestSessionsTabViewRendersWorkspaces(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)
	view := tab.View()
	if !strings.Contains(view, "dev") {
		t.Error("expected view to contain 'dev'")
	}
	if !strings.Contains(view, "api") {
		t.Error("expected view to contain 'api'")
	}
}

// ── Search / filter ──

// typeSearch enters search mode via "/" (when not already there) and types
// each rune of query as a separate keystroke, driving the live-filter path
// through handleSearchKey exactly as a real user would.
func typeSearch(tab *SessionsTab, query string) *SessionsTab {
	if tab.mode != sessionsModeSearch {
		tab, _ = sendKey(tab, "/")
	}
	for _, r := range query {
		tab, _ = sendKey(tab, string(r))
	}
	return tab
}

func TestSessionsTabSearchEntersMode(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	tab, _ = sendKey(tab, "/")
	if tab.mode != sessionsModeSearch {
		t.Fatalf("expected search mode, got %d", tab.mode)
	}
	if !tab.CapturesEscape() {
		t.Error("expected CapturesEscape true while in search mode")
	}
}

func TestSessionsTabSearchFiltersByWorkspaceName(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	tab = typeSearch(tab, "api")

	if findRowIndexByID(tab, outline.WorkspaceID("api")) < 0 {
		t.Error("expected api workspace to survive the filter")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("dev")) >= 0 {
		t.Error("expected dev workspace to be filtered out")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("empty")) >= 0 {
		t.Error("expected empty workspace to be filtered out")
	}
}

func TestSessionsTabSearchFiltersBySessionName(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// "dev-2" matches the session, not the workspace name "dev" — the header
	// is kept (force-expanded) but only the matching session shows.
	tab = typeSearch(tab, "dev-2")

	if findRowIndexByID(tab, outline.WorkspaceID("dev")) < 0 {
		t.Error("expected dev header kept for matching session")
	}
	if findRowIndexByID(tab, outline.SessionID("dev-2")) < 0 {
		t.Error("expected matching session dev-2 to show")
	}
	if findRowIndexByID(tab, outline.SessionID("dev")) >= 0 {
		t.Error("expected non-matching sibling session dev to be hidden")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("api")) >= 0 {
		t.Error("expected api workspace filtered out")
	}
}

func TestSessionsTabSearchEscCancelsFilter(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	tab = typeSearch(tab, "api")
	tab, _ = sendKey(tab, "esc")

	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode after esc, got %d", tab.mode)
	}
	if tab.searchQuery != "" {
		t.Errorf("expected filter cleared, got %q", tab.searchQuery)
	}
	// All three workspaces back, collapsed.
	if got := len(tab.tree.Rows); got != 3 {
		t.Errorf("expected 3 rows after cancel, got %d", got)
	}
}

func TestSessionsTabSearchEnterCommitsFilter(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	tab = typeSearch(tab, "api")
	tab, _ = sendKey(tab, "enter")

	// Back to list browsing but the filter stays active.
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode after enter, got %d", tab.mode)
	}
	if tab.searchQuery != "api" {
		t.Errorf("expected filter retained as %q, got %q", "api", tab.searchQuery)
	}
	if !tab.CapturesEscape() {
		t.Error("expected CapturesEscape true while a committed filter is active")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("dev")) >= 0 {
		t.Error("expected dev still filtered out after commit")
	}

	// A second Esc (now in list mode) clears the committed filter.
	tab, _ = sendKey(tab, "esc")
	if tab.searchQuery != "" {
		t.Errorf("expected filter cleared by list-mode esc, got %q", tab.searchQuery)
	}
	if findRowIndexByID(tab, outline.WorkspaceID("dev")) < 0 {
		t.Error("expected dev to return after clearing filter")
	}
}

func TestSessionsTabSearchFiltersExternalGroups(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Seed an external source group (simulateActivate clears the catalog).
	tab.catalog = &source.Catalog{
		External: []source.SourceGroup{{
			Source: source.Source{
				ID:     "/tmp/remote.sock",
				Kind:   source.SourceExternal,
				Label:  "remote-box",
				Health: source.HealthOK,
			},
			Entries: []source.CatalogEntry{{Session: "rsess"}},
		}},
	}
	tab.tree.SetRows(tab.buildRows())

	tab = typeSearch(tab, "remote")

	groupID := outline.ExternalGroupID("external", "/tmp/remote.sock")
	if findRowIndexByID(tab, groupID) < 0 {
		t.Error("expected external group matching label to survive filter")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("dev")) >= 0 {
		t.Error("expected workspaces filtered out by external-only query")
	}
}

// TestSessionsTabSearchMutationUnderFilterClearsAndJumps covers the latent
// jump-miss guard in applyData: a mutation whose target row is hidden by the
// active filter must drop the filter and land the cursor on the new row.
func TestSessionsTabSearchMutationUnderFilterClearsAndJumps(t *testing.T) {
	tab, _, store := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Commit a filter that hides everything but api.
	tab = typeSearch(tab, "api")
	tab, _ = sendKey(tab, "enter")
	if tab.searchQuery != "api" {
		t.Fatalf("expected committed filter, got %q", tab.searchQuery)
	}

	// Create a workspace that the filter would hide.
	tab, _ = sendKey(tab, "C")
	if tab.mode != sessionsModeCreate {
		t.Fatalf("expected create mode, got %d", tab.mode)
	}
	tab.createInput.SetValue("zebra")
	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	if ws, _ := store.GetWorkspace("zebra"); ws == nil {
		t.Fatal("expected zebra workspace created")
	}
	if tab.searchQuery != "" {
		t.Errorf("expected filter dropped after hidden-target mutation, got %q", tab.searchQuery)
	}
	row := tab.tree.Current()
	if row == nil || row.ID != outline.WorkspaceID("zebra") {
		t.Errorf("expected cursor jumped to zebra, got %+v", row)
	}
}

// TestSessionsTabSearchForceExpandPreservesSavedState verifies the filter's
// force-expand is view-only: clearing the filter restores the workspace's
// original collapsed state rather than leaving it expanded.
func TestSessionsTabSearchForceExpandPreservesSavedState(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	devID := outline.WorkspaceID("dev")
	if tab.tree.IsExpanded(devID) {
		t.Fatal("expected dev collapsed at start")
	}

	// Filter to dev — it force-expands in the view.
	tab = typeSearch(tab, "dev")
	if findRowIndexByID(tab, outline.SessionID("dev")) < 0 {
		t.Error("expected dev sessions visible under filter (force-expand)")
	}

	// Cancel: saved expansion state must be untouched (still collapsed).
	tab, _ = sendKey(tab, "esc")
	if tab.tree.IsExpanded(devID) {
		t.Error("expected dev expansion state untouched by force-expand")
	}
	if got := len(tab.tree.Rows); got != 3 {
		t.Errorf("expected 3 collapsed rows after clearing filter, got %d", got)
	}
}

// TestSessionsTabFilteredEnterDoesNotToggleExpansion guards the buddy-flagged
// bug: Enter on a force-expanded (filtered) header must not mutate the saved
// expansion state, since the toggle would be invisible under the filter and
// surface as a surprise flip once the filter clears.
func TestSessionsTabFilteredEnterDoesNotToggleExpansion(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	devID := outline.WorkspaceID("dev")
	tab = typeSearch(tab, "dev")
	tab, _ = sendKey(tab, "enter") // commit filter; dev force-expanded

	// Cursor onto the dev header and press Enter — should be a no-op.
	devIdx := findRowIndexByID(tab, devID)
	if devIdx < 0 {
		t.Fatal("dev header not found under filter")
	}
	tab.tree.Cursor = devIdx
	tab, _ = sendKey(tab, "enter")

	if tab.tree.IsExpanded(devID) {
		t.Error("expected saved expansion untouched by Enter while filtered")
	}

	// Clearing the filter should reveal dev collapsed, not expanded.
	tab, _ = sendKey(tab, "esc")
	if tab.tree.IsExpanded(devID) {
		t.Error("expected dev collapsed after clearing filter")
	}
	if got := len(tab.tree.Rows); got != 3 {
		t.Errorf("expected 3 collapsed rows after clear, got %d", got)
	}
}

// TestSessionsTabMoveModeIgnoresFilter guards the buddy-flagged bug: a
// committed filter must be suspended while picking a move destination, so the
// user can move a session to ANY workspace, not just matching ones. The filter
// is restored (not cleared) when move mode exits.
func TestSessionsTabMoveModeIgnoresFilter(t *testing.T) {
	tab, _, _ := newTestSessionsTab(t)
	tab = simulateActivate(tab)

	// Commit a filter that hides api + empty.
	tab = typeSearch(tab, "dev")
	tab, _ = sendKey(tab, "enter")
	if findRowIndexByID(tab, outline.WorkspaceID("api")) >= 0 {
		t.Fatal("precondition: api should be filtered out before move")
	}

	// Cursor onto a dev session and enter move mode.
	sIdx := findRowIndexByID(tab, outline.SessionID("dev-2"))
	if sIdx < 0 {
		t.Fatal("dev-2 session row not found under filter")
	}
	tab.tree.Cursor = sIdx
	tab, _ = sendKey(tab, "m")
	if tab.mode != sessionsModeMove {
		t.Fatalf("expected move mode, got %d", tab.mode)
	}

	// All workspaces must be reachable as move targets.
	if findRowIndexByID(tab, outline.WorkspaceID("api")) < 0 {
		t.Error("expected api reachable as move target despite filter")
	}
	if findRowIndexByID(tab, outline.WorkspaceID("empty")) < 0 {
		t.Error("expected empty reachable as move target despite filter")
	}
	// Filter is suspended, not destroyed.
	if tab.searchQuery != "dev" {
		t.Errorf("expected filter suspended (retained) during move, got %q", tab.searchQuery)
	}

	// Cancelling move restores the filter.
	tab, _ = sendKey(tab, "esc")
	if tab.searchQuery != "dev" {
		t.Errorf("expected filter retained after move cancel, got %q", tab.searchQuery)
	}
	if findRowIndexByID(tab, outline.WorkspaceID("api")) >= 0 {
		t.Error("expected api filtered out again after move cancel")
	}
}

// navigateToWorkspaceInMoveMode walks the cursor up and down (in move mode)
// trying to land on the workspace row with the given ID. Returns false if
// no progress can be made.
func navigateToWorkspaceInMoveMode(tab *SessionsTab, targetID string) bool {
	for i := 0; i < 16; i++ {
		row := tab.tree.Current()
		if row != nil && row.ID == targetID {
			return true
		}
		prev := tab.tree.Cursor
		tab, _ = sendKey(tab, "down")
		if tab.tree.Cursor == prev {
			break
		}
	}
	for i := 0; i < 16; i++ {
		row := tab.tree.Current()
		if row != nil && row.ID == targetID {
			return true
		}
		prev := tab.tree.Cursor
		tab, _ = sendKey(tab, "up")
		if tab.tree.Cursor == prev {
			break
		}
	}
	row := tab.tree.Current()
	return row != nil && row.ID == targetID
}

// ── Helpers ──

func mockCalled(mock *tmux.MockRunner, method string) bool {
	for _, c := range mock.Calls {
		if c.Method == method {
			return true
		}
	}
	return false
}
