package tabs

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/workspace"
)

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
	if err := store.CreateWorkspace("dev", ""); err != nil {
		t.Fatalf("create dev: %v", err)
	}
	if err := store.CreateWorkspace("api", ""); err != nil {
		t.Fatalf("create api: %v", err)
	}
	if err := store.CreateWorkspace("empty", ""); err != nil {
		t.Fatalf("create empty: %v", err)
	}
	if err := store.AddSession("dev", "dev"); err != nil {
		t.Fatalf("add dev session: %v", err)
	}
	if err := store.AddSession("dev", "dev-2"); err != nil {
		t.Fatalf("add dev-2 session: %v", err)
	}
	if err := store.AddSession("api", "api"); err != nil {
		t.Fatalf("add api session: %v", err)
	}

	loader := func() []tui.WorkspaceViewModel {
		ws, _ := store.ListWorkspaces()
		live, _ := session.ListSessions(mock)
		return tui.BuildWorkspaceViewModels(ws, live)
	}

	tab := NewSessionsTab(mock, tui.DefaultStyles(), loader, store)
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
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
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

	tab, _ = sendKey(tab, "n")
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
	prepareMockForRename(mock, "dev", "dev-renamed")

	tab, cmd := sendKey(tab, "enter")
	tab = runMutationCmd(tab, cmd)

	row := tab.tree.Current()
	if row == nil || row.ID != outline.SessionID("dev-renamed") {
		t.Errorf("expected cursor on dev-renamed, got %+v", row)
	}
}

func prepareMockForRename(mock *tmux.MockRunner, oldName, newName string) {
	for i := range mock.Sessions {
		if mock.Sessions[i].Name == oldName {
			mock.Sessions[i].Name = newName
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
		if s == "dev-2" {
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
		workspaces: []tui.WorkspaceViewModel{{Workspace: workspace.Workspace{Name: "fresh"}}},
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
		workspaces: []tui.WorkspaceViewModel{
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
	if !strings.Contains(help, "n:new") {
		t.Errorf("expected 'n:new' in help, got %q", help)
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
