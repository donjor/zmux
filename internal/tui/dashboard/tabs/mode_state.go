package tabs

// Shared modal-state structs used by both the Session & Workspace
// (CurrentTab) and Workspaces (SessionsTab) tabs. Before unification each
// tab carried its own near-identical confirmState / renameState / moveState
// triple — the only meaningful difference was that the current tab needed
// to address tmux windows in addition to workspaces and sessions, which we
// model with an optional windowIndex field that's left at its zero value
// for non-window targets.

// confirmState describes a pending y/N kill confirmation.
//
//   - kind is "workspace", "session", "window", or "pane".
//   - attached is only meaningful when kind == "workspace": it triggers
//     the second-step "this will detach you" confirmation.
//   - sessionName / windowIndex are only meaningful when kind == "window"
//     and identify the tmux session that owns the window (may differ
//     from the active session when targeting a sibling session's tab).
//   - paneID is only meaningful when kind == "pane".
type confirmState struct {
	kind        string
	name        string
	attached    bool
	sessionName string
	windowIndex int
	paneID      string
}

// renameState records what we're renaming.
//
//   - kind is "workspace", "session", or "window".
//   - sessionName / windowIndex are only meaningful when kind == "window"
//     and identify the owning tmux session.
type renameState struct {
	kind        string
	oldName     string
	sessionName string
	windowIndex int
}

// moveState describes an in-flight session-or-window move.
//
// The Workspaces tab uses sessionName + originWorkspace to move a session
// between workspaces. The Session & Workspace tab uses sessionName +
// windowIndex to move a window between sessions; in that case
// originWorkspace stays empty. Either way, the struct represents one
// "thing being moved".
type moveState struct {
	sessionName     string // root name of the session being moved (or owning the window)
	originWorkspace string // workspace the session lives in (sessions tab only)
	windowIndex     int    // window index being moved (current tab only)
}
