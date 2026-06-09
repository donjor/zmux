package tabs

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	tabspkg "github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// fetchData gathers the current session, its windows, sibling sessions
// within the same workspace, and the workspace view model. All work is
// done inside the returned command so it runs off the UI thread.
func (t *CurrentTab) fetchData(reqID int64) tea.Cmd {
	runner := t.runner
	wsLoader := t.wsLoader
	wsStore := t.wsStore
	return func() tea.Msg {
		sessionName, err := runner.DisplayMessage("", "#{session_name}")
		if err != nil {
			return currentDataMsg{reqID: reqID, err: err}
		}
		sessionName = session.RootName(strings.TrimSpace(sessionName))

		if sessionName == "" {
			return currentDataMsg{reqID: reqID, err: fmt.Errorf("no active session")}
		}

		sessionDir, _ := runner.DisplayMessage("", "#{session_path}")
		sessionDir = strings.TrimSpace(sessionDir)

		attachedStr, _ := runner.DisplayMessage("", "#{session_attached}")
		attached := 0
		_, _ = fmt.Sscanf(strings.TrimSpace(attachedStr), "%d", &attached)

		// Workspace name for the current session.
		wsName := ""
		if wsStore != nil {
			if name, ok := wsStore.WorkspaceFor(sessionName); ok {
				wsName = name
			}
		}
		if wsName == "" {
			// Fall back to the session name itself so we always show a header.
			wsName = sessionName
		}

		windows := fetchWindowDetails(runner, sessionName)

		var siblings []session.SessionInfo
		var wsModel *workspaceview.WorkspaceViewModel
		if wsLoader != nil {
			all := wsLoader()
			for i := range all {
				if all[i].Name == wsName {
					m := all[i]
					wsModel = &m
					for _, s := range all[i].LiveSessions {
						if s.Name != sessionName {
							siblings = append(siblings, s)
						}
					}
					break
				}
			}
		}

		// Fetch basic windows for each sibling (no pane/process detail —
		// keep it cheap; current session gets the full treatment).
		siblingWindows := make(map[string][]tmux.Window, len(siblings))
		for _, s := range siblings {
			if ws, err := runner.ListWindows(s.Name); err == nil {
				siblingWindows[s.Name] = ws
			}
		}

		return currentDataMsg{
			reqID:          reqID,
			wsName:         wsName,
			sessionName:    sessionName,
			sessionDir:     sessionDir,
			attached:       attached,
			windows:        windows,
			siblings:       siblings,
			siblingWindows: siblingWindows,
			wsModel:        wsModel,
		}
	}
}

// fetchWindowDetails enriches tmux windows with pane + process stats.
func fetchWindowDetails(runner tmux.Runner, sessionName string) []windowDetail {
	rawWindows, err := runner.ListWindows(sessionName)
	if err != nil {
		return nil
	}

	rawPanes, _ := runner.ListPanes(sessionName)

	panesByWindow := make(map[int][]tmux.Pane)
	for _, p := range rawPanes {
		panesByWindow[p.WindowIndex] = append(panesByWindow[p.WindowIndex], p)
	}

	var pids []int
	for _, w := range rawWindows {
		for _, p := range panesByWindow[w.Index] {
			if p.Active && p.PID > 0 {
				pids = append(pids, p.PID)
			}
		}
	}

	allStats := tmux.GetBatchProcessStats(pids)

	details := make([]windowDetail, 0, len(rawWindows))
	for _, w := range rawWindows {
		wd := windowDetail{
			Window: w,
			Panes:  panesByWindow[w.Index],
		}
		for _, p := range wd.Panes {
			if p.Active && p.PID > 0 {
				if stats, ok := allStats[p.PID]; ok {
					wd.Stats = stats
					wd.Uptime = stats.Uptime
				}
				break
			}
		}
		details = append(details, wd)
	}
	return details
}

// fetchMoveDestinations returns the list of sessions a window can be
// moved into (everything but the current session).
func (t *CurrentTab) fetchMoveDestinations() tea.Cmd {
	runner := t.runner
	current := t.sessionName
	reqID := t.reqID
	return func() tea.Msg {
		sessions, err := runner.ListSessions()
		if err != nil {
			return currentMoveDestMsg{reqID: reqID}
		}
		var targets []currentMoveTarget
		for _, s := range sessions {
			if tabspkg.IsReservedSession(s.Name) {
				continue // never offer the dock as a move destination
			}
			if s.Name != current {
				targets = append(targets, currentMoveTarget{
					Name:    s.Name,
					Windows: s.Windows,
				})
			}
		}
		return currentMoveDestMsg{reqID: reqID, sessions: targets}
	}
}

// ── Mutation helpers ──
//
// Workspace / session rename + kill helpers wrap shared functions in
// shared_mutations.go (which the Workspaces tab also uses). The wrappers
// here exist to (a) snapshot tab state into closure args, (b) set
// pendingJumpTo for the next refetch, and (c) wrap the result in the
// tab-specific done-message type.

// renameWorkspace renames a workspace and queues a jump-to on the new ID.
// Errors (validation, name conflict) surface as a status flash so the user
// gets feedback instead of a silent no-op.
func (t *CurrentTab) renameWorkspace(oldName, newName string) tea.Cmd {
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
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// renameSession renames a tmux session and queues a jump-to on the new ID.
// Errors surface as a status flash; silent failure here is what the user
// hit when reporting the rename flow as "fragile".
func (t *CurrentTab) renameSession(oldName, newName string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID
	t.pendingJumpTo = outline.SessionID(newName)
	return func() tea.Msg {
		if err := renameSessionMutation(runner, wsStore, oldName, newName); err != nil {
			return dashboard.SetStatusIntent{
				Text:    fmt.Sprintf("rename session failed: %v", err),
				IsError: true,
			}
		}
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// renameWindow renames a window in the given session. The session arg
// allows renaming sibling-session windows, not just the current one.
func (t *CurrentTab) renameWindow(sessionName, oldName, newName string, idx int) tea.Cmd {
	runner := t.runner
	reqID := t.reqID
	t.pendingJumpTo = outline.WindowID(sessionName, idx)
	return func() tea.Msg {
		_ = runner.RenameWindow(sessionName, oldName, newName)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// killWorkspace kills all live sessions in the workspace and deletes it.
func (t *CurrentTab) killWorkspace(name string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	reqID := t.reqID

	// Snapshot live session names — current session + its siblings.
	sessNames := make([]string, 0, 1+len(t.siblings))
	sessNames = append(sessNames, t.sessionName)
	for _, s := range t.siblings {
		sessNames = append(sessNames, s.Name)
	}

	return func() tea.Msg {
		_ = killWorkspaceMutation(runner, wsStore, name, sessNames)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// killSession kills a session. When the target is the currently-attached
// session, we switch the client to a sibling first so killing the underlying
// tmux session doesn't drop the client (and with it the dashboard popup).
//
// A standalone session with no siblings cannot be killed from the in-session
// dashboard — there's nowhere to switch to, and killing it would terminate
// the client mid-action. The user has to fall back to `zmux kill <ws>` or
// killing the whole workspace from outside.
func (t *CurrentTab) killSession(name string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	wsName := t.wsName
	reqID := t.reqID
	isCurrent := name == t.sessionName
	fallback := ""
	if isCurrent && len(t.siblings) > 0 {
		fallback = t.siblings[0].Name
	}

	if isCurrent && fallback == "" {
		return func() tea.Msg {
			return dashboard.SetStatusIntent{
				Text:    "Cannot kill the only session — would drop your client. Run this from outside tmux instead.",
				IsError: true,
			}
		}
	}

	return func() tea.Msg {
		if fallback != "" {
			_ = runner.SwitchClient(fallback)
			if wsStore != nil {
				_ = wsStore.SetLastActive(wsName, fallback)
			}
		}
		_ = killSessionMutation(runner, wsStore, name)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// killWindow kills a window by index in the given session. The session
// arg allows killing sibling-session windows.
func (t *CurrentTab) killWindow(sessionName string, idx int) tea.Cmd {
	runner := t.runner
	reqID := t.reqID
	return func() tea.Msg {
		_ = runner.KillWindow(sessionName, idx)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// killPane kills a pane by id.
func (t *CurrentTab) killPane(paneID string) tea.Cmd {
	runner := t.runner
	reqID := t.reqID
	return func() tea.Msg {
		_ = runner.KillPane(paneID)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// newWindow creates a new window in the named session.
func (t *CurrentTab) newWindow(sessionName, dir string) tea.Cmd {
	runner := t.runner
	reqID := t.reqID
	return func() tea.Msg {
		_, _ = runner.NewWindow(sessionName, "", dir)
		return currentMutationDoneMsg{reqID: reqID}
	}
}

// createSessionInWorkspace creates a tmux session and attaches it to a workspace.
func (t *CurrentTab) createSessionInWorkspace(wsName, sessionName string) tea.Cmd {
	runner := t.runner
	wsStore := t.wsStore
	dir := t.sessionDir
	reqID := t.reqID
	t.pendingJumpTo = outline.SessionID(sessionName)
	return func() tea.Msg {
		_ = session.Create(runner, sessionName, dir)
		if wsStore != nil {
			_ = wsStore.AddSession(wsName, sessionName)
		}
		return currentMutationDoneMsg{reqID: reqID}
	}
}
