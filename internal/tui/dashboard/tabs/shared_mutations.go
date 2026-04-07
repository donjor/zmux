package tabs

import (
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// Shared mutation helpers used by both the Session & Workspace tab and the
// Workspaces tab. They take explicit dependencies (runner, wsStore) so they
// have no implicit ties to either tab's struct, and they return raw errors
// so each caller can produce its own done-message type for the bubbletea
// scheduler.
//
// The "snapshot loop" for killing all sessions in a workspace differs
// between the two tabs (one walks the live snapshot, the other concatenates
// the current session and its siblings) so killWorkspaceCmd accepts the
// pre-computed list of session names from the caller — the helper itself
// doesn't try to figure out what's in the workspace.

// renameWorkspaceMutation persists a workspace rename via the workspace
// store. A nil store is a no-op so tests that don't wire one in still work.
func renameWorkspaceMutation(wsStore *workspace.Store, oldName, newName string) error {
	if wsStore == nil {
		return nil
	}
	return wsStore.RenameWorkspace(oldName, newName)
}

// renameSessionMutation renames a tmux session and updates the workspace
// store mapping (best-effort: store errors are intentionally swallowed to
// match prior behaviour, since the tmux rename has already happened).
func renameSessionMutation(runner tmux.Runner, wsStore *workspace.Store, oldName, newName string) error {
	if err := session.Rename(runner, oldName, newName); err != nil {
		return err
	}
	if wsStore != nil {
		_ = wsStore.RenameSession(session.RootName(oldName), session.RootName(newName))
	}
	return nil
}

// killWorkspaceMutation kills the given session names and then deletes the
// workspace from the store. The caller is responsible for snapshotting the
// list of session names to kill — the helper deliberately doesn't reach
// back into either tab's view-model to compute it.
func killWorkspaceMutation(runner tmux.Runner, wsStore *workspace.Store, name string, sessNames []string) error {
	for _, n := range sessNames {
		_ = session.Kill(runner, n)
	}
	if wsStore != nil {
		_ = wsStore.DeleteWorkspace(name)
	}
	return nil
}

// killSessionMutation kills a single tmux session.
func killSessionMutation(runner tmux.Runner, name string) error {
	return session.Kill(runner, name)
}
