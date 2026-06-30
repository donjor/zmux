// Package tabs implements the individual dashboard tab models.
package tabs

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/overmind"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceoutline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

// sessionsMode tracks the current interaction mode.
type sessionsMode int

const (
	sessionsModeList                sessionsMode = iota // browsing the workspace tree
	sessionsModeRename                                  // inline rename input (ws or session)
	sessionsModeCreate                                  // inline create-workspace input
	sessionsModeConfirmKill                             // y/N confirm
	sessionsModeConfirmKillAttached                     // second-step confirm for attached ws
	sessionsModeMove                                    // inline move-session destination picker
	sessionsModeSearch                                  // inline `/` search input over the tree
)

// confirmState / renameState / moveState live in mode_state.go and are
// shared with the Session & Workspace tab.

// ── Messages ──

// sessionsDataMsg carries fetched workspace + external data.
type sessionsDataMsg struct {
	reqID      int64
	workspaces []workspaceview.WorkspaceViewModel
	current    string // current session name (for "current" marker)
	catalog    *source.Catalog
	err        error
}

func (m sessionsDataMsg) TargetTab() dashboard.TabID { return dashboard.TabWorkspaces }

// sessionsMutationDoneMsg signals that a mutation completed; the tab will refetch.
type sessionsMutationDoneMsg struct {
	reqID int64
}

func (m sessionsMutationDoneMsg) TargetTab() dashboard.TabID { return dashboard.TabWorkspaces }

// SessionsTab implements the Tab interface for global workspace management.
type SessionsTab struct {
	runner   tmux.Runner
	styles   styles.Styles
	wsLoader workspaceview.WorkspaceDataLoader
	wsStore  *workspace.Store
	overmind overmind.Client

	// Tree owns row data, cursor, and expansion state.
	tree *outline.Tree

	// Snapshot data.
	workspaces []workspaceview.WorkspaceViewModel
	current    string
	catalog    *source.Catalog

	// Viewport — handles scrolling automatically.
	vp     viewport.Model
	width  int
	height int

	// Interaction mode.
	mode sessionsMode

	// Inputs / overlay state.
	renameInput textinput.Model
	createInput textinput.Model
	searchInput textinput.Model
	rename      *renameState
	confirm     *confirmState
	moveSt      *moveState

	// createWsTarget is the workspace a pending create targets. Empty means
	// create-mode is creating a workspace (C); non-empty means creating a
	// session in that workspace (c).
	createWsTarget string

	// searchQuery is the active filter over the tree. It is independent of
	// sessionsModeSearch: editing the query lives in that mode, but a
	// committed query keeps filtering while the user browses in list mode.
	searchQuery string

	// Async-correctness primitives.
	reqID         int64
	pending       *sessionsDataMsg // staged refetch arrived during a modal
	pendingJumpTo string           // one-shot row ID to land on after next apply
}

// NewSessionsTab creates a new workspaces tab.
// wsLoader returns enriched workspace view models; wsStore is required for
// CRUD mutations from inside the dashboard; om drives overmind restart/stop.
func NewSessionsTab(runner tmux.Runner, styles styles.Styles, wsLoader workspaceview.WorkspaceDataLoader, wsStore *workspace.Store, om overmind.Client) *SessionsTab {
	ri := textinput.New()
	ri.Placeholder = "new name..."
	ri.CharLimit = 64

	ci := textinput.New()
	ci.Placeholder = "workspace name..."
	ci.CharLimit = 64

	si := textinput.New()
	si.Placeholder = "search workspaces & sessions..."
	si.CharLimit = 64

	return &SessionsTab{
		runner:      runner,
		styles:      styles,
		wsLoader:    wsLoader,
		wsStore:     wsStore,
		overmind:    om,
		tree:        outline.NewTree(),
		renameInput: ri,
		createInput: ci,
		searchInput: si,
	}
}

// ── Tab interface ──

func (t *SessionsTab) ID() dashboard.TabID { return dashboard.TabWorkspaces }
func (t *SessionsTab) Title() string       { return "Workspaces" }
func (t *SessionsTab) Init() tea.Cmd       { return nil }

// CapturesEscape reports that Esc should be handled by the tab rather than
// close the dashboard: while in a capturing mode (rename/create/move/confirm/
// search), or while a committed search filter is active in list mode (Esc
// clears it).
func (t *SessionsTab) CapturesEscape() bool {
	return t.mode != sessionsModeList || t.searchQuery != ""
}

func (t *SessionsTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.tree.ResetExpansion()
	t.reqID = dashboard.NextReqID()
	return t.fetchData(t.reqID)
}

func (t *SessionsTab) Deactivate() {
	// Bump reqID so any in-flight responses become stale.
	t.reqID = dashboard.NextReqID()
	t.exitMode()
	t.pending = nil
	t.pendingJumpTo = ""
	t.renameInput.Blur()
	t.createInput.Blur()
	// Drop any active search filter so the tab is fresh on re-entry.
	t.searchQuery = ""
	t.searchInput.SetValue("")
	t.searchInput.Blur()
}

func (t *SessionsTab) Resize(width, height int) {
	t.width = width
	t.height = height
	t.vp.SetWidth(width)
	t.vp.SetHeight(height)
}

// Update processes messages for the workspaces tab.
func (t *SessionsTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil

	case sessionsDataMsg:
		t.onDataMsg(msg)
		return t, nil

	case sessionsMutationDoneMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.reqID = dashboard.NextReqID()
		return t, t.fetchData(t.reqID)

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward stray messages to the active text input.
	switch t.mode {
	case sessionsModeRename:
		var cmd tea.Cmd
		t.renameInput, cmd = t.renameInput.Update(msg)
		return t, cmd
	case sessionsModeCreate:
		var cmd tea.Cmd
		t.createInput, cmd = t.createInput.Update(msg)
		return t, cmd
	case sessionsModeSearch:
		var cmd tea.Cmd
		t.searchInput, cmd = t.searchInput.Update(msg)
		return t, cmd
	}
	return t, nil
}

// onDataMsg handles an incoming data message: dropped if stale, staged if a
// modal is open, or applied immediately otherwise.
func (t *SessionsTab) onDataMsg(msg sessionsDataMsg) {
	if msg.reqID != t.reqID {
		return
	}
	if msg.err != nil {
		// Surface the error via status flash, but don't crash the tab.
		return
	}
	if t.mode != sessionsModeList {
		t.pending = &msg
		return
	}
	t.applyData(msg)
}

// applyData replaces the snapshot data and rebuilds the tree.
func (t *SessionsTab) applyData(msg sessionsDataMsg) {
	t.workspaces = msg.workspaces
	t.current = msg.current
	t.catalog = msg.catalog
	rows := t.buildRows()
	if t.pendingJumpTo != "" {
		// If a committed search filter would hide the row we just acted on
		// (rename/create/move target), drop the filter and rebuild — otherwise
		// SetRowsAndJumpTo's silent fallback chain lands the cursor on an
		// unrelated row. Kills set no pendingJumpTo, so they keep the filter.
		if t.searchQuery != "" && !rowsContain(rows, t.pendingJumpTo) {
			t.searchQuery = ""
			t.searchInput.SetValue("")
			rows = t.buildRows()
		}
		t.tree.SetRowsAndJumpTo(rows, t.pendingJumpTo)
		t.pendingJumpTo = ""
	} else {
		t.tree.SetRows(rows)
	}
}

// exitMode resets to list mode and applies any staged data refetch.
func (t *SessionsTab) exitMode() {
	t.mode = sessionsModeList
	t.confirm = nil
	t.rename = nil
	t.moveSt = nil
	t.createWsTarget = ""
	t.renameInput.Blur()
	t.createInput.Blur()
	t.searchInput.Blur()
	if t.pending != nil {
		t.applyData(*t.pending)
		t.pending = nil
	}
}

// ── Data fetching ──

func (t *SessionsTab) fetchData(reqID int64) tea.Cmd {
	runner := t.runner
	wsLoader := t.wsLoader
	return func() tea.Msg {
		var workspaces []workspaceview.WorkspaceViewModel
		if wsLoader != nil {
			workspaces = wsLoader()
		}

		current, _ := runner.DisplayMessage("", "#{session_name}")
		current = session.RootName(strings.TrimSpace(current))

		// External sources are best-effort. Failures fall back to empty.
		cat, _ := source.Discover(runner.Endpoint())

		return sessionsDataMsg{
			reqID:      reqID,
			workspaces: workspaces,
			current:    current,
			catalog:    cat,
		}
	}
}

// ── Mutation helpers ──
//
// The actual mutation logic lives in shared_mutations.go (shared with the
// Session & Workspace tab). The wrappers here exist to (a) snapshot tab
// state into closure args, (b) set pendingJumpTo for the next refetch, and
// (c) wrap the result in the tab-specific done-message type.

// killWorkspace kills all live sessions in the workspace and deletes it.
func (t *SessionsTab) killWorkspace(name string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID

	// Snapshot live session names so the closure doesn't depend on tab state.
	var sessNames []string
	for i := range t.workspaces {
		if t.workspaces[i].Name == name {
			for _, s := range t.workspaces[i].LiveSessions {
				sessNames = append(sessNames, s.Name)
			}
			break
		}
	}

	return func() tea.Msg {
		if err := killWorkspaceMutation(runner, wsStore, name, sessNames); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("kill workspace failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// killSession kills a single session.
func (t *SessionsTab) killSession(name string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID
	return func() tea.Msg {
		if err := killSessionMutation(runner, wsStore, name); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("kill session failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// renameWorkspace renames a workspace and queues a jump-to on the new ID.
// Errors surface as a status flash; silent failure is what the user hit
// when reporting the rename flow as "fragile".
func (t *SessionsTab) renameWorkspace(oldName, newName string) tea.Cmd {
	wsStore := t.wsStore
	reqID := t.reqID
	t.pendingJumpTo = outline.WorkspaceID(newName)
	return func() tea.Msg {
		if err := renameWorkspaceMutation(wsStore, oldName, newName); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("rename workspace failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// renameSession renames a tmux session and queues a jump-to on the new ID.
func (t *SessionsTab) renameSession(oldName, newName string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID
	t.pendingJumpTo = outline.SessionID(renamedSessionTarget(wsStore, oldName, newName))
	return func() tea.Msg {
		if err := renameSessionMutation(runner, wsStore, oldName, newName); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("rename session failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// createWorkspace creates a workspace and queues a jump-to on the new row.
func (t *SessionsTab) createWorkspace(name string) tea.Cmd {
	wsStore := t.wsStore
	reqID := t.reqID
	t.pendingJumpTo = outline.WorkspaceID(name)
	return func() tea.Msg {
		if wsStore == nil {
			return dashboard.SetStatusIntent{
				Text:    "create workspace failed: workspace store unavailable",
				IsError: true,
			}
		}
		if err := wsStore.CreateWorkspace(name, ""); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("create workspace failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// createSessionInWorkspace creates a canonically-named managed session in the
// target workspace (shared workspace.CreateManagedSession path, same as the CLI
// and the Current tab) and force-expands that workspace so the new session row
// is visible after the refetch. The caller queues the jump-to on the new row.
func (t *SessionsTab) createSessionInWorkspace(wsName, label string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID
	dir := createSessionDir(wsStore, wsName, "")
	t.tree.SetExpanded(outline.WorkspaceID(wsName), true)
	return func() tea.Msg {
		if _, err := createSessionMutation(runner, wsStore, wsName, label, dir); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("create session failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// moveSessionTo commits an inline move and queues a jump-to on the moved row.
func (t *SessionsTab) moveSessionTo(sessionName, destWorkspace string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID
	t.pendingJumpTo = outline.SessionID(sessionName)
	return func() tea.Msg {
		if pinned, err := isPinnedViewSession(runner, sessionName); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("move session failed: %v", err),
				IsError: true,
			}
		} else if pinned {
			return dashboard.SetStatusIntent{
				Text:    "move session failed: pinned views cannot be moved; move the root session",
				IsError: true,
			}
		}
		if wsStore == nil {
			return dashboard.SetStatusIntent{
				Text:    "move session failed: workspace store unavailable",
				IsError: true,
			}
		}
		if err := wsStore.MoveSession(session.RootName(sessionName), destWorkspace); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("move session failed: %v", err),
				IsError: true,
			}
		}
		return sessionsMutationDoneMsg{reqID: reqID}
	}
}

// ── ShortHelp ──

func (t *SessionsTab) ShortHelp() string {
	switch t.mode {
	case sessionsModeRename:
		return "enter:confirm  esc:cancel"
	case sessionsModeCreate:
		return "enter:create  esc:cancel"
	case sessionsModeConfirmKill, sessionsModeConfirmKillAttached:
		return "y:confirm  any:cancel"
	case sessionsModeMove:
		return "↑↓:workspace  enter:move  esc:cancel"
	case sessionsModeSearch:
		return "type:filter  ↑↓:move  enter:apply  esc:cancel"
	}

	// List mode: the trailing hint advertises search, plus esc:clear when a
	// filter is active.
	tail := "  /:search"
	if t.searchQuery != "" {
		tail += "  esc:clear"
	}

	row := t.tree.Current()
	if row == nil {
		return "C:workspace" + tail
	}

	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return strings.Join([]string{"enter:expand", "c:session", "C:workspace", "r:rename", "x:kill"}, "  ") + tail
	case outline.RowSession:
		return strings.Join([]string{"enter:switch", "c:session", "C:workspace", "r:rename", "x:kill", "m:move"}, "  ") + tail
	case outline.RowExternalGroup:
		return "enter:toggle  C:workspace" + tail
	case outline.RowExternalEntry:
		if g, ok := workspaceoutline.ExternalGroupForRow(t.catalog, row); ok && g != nil && g.Source.Kind == source.SourceOvermind {
			return "enter:connect  r:restart  x:stop  C:workspace" + tail
		}
		return "enter:attach  C:workspace" + tail
	}
	return "C:workspace" + tail
}
