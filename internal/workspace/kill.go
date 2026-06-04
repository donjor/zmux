package workspace

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
)

// MembershipRemover is the slice of the workspace store that KillSession
// needs. Keeping it as an interface lets callers like the picker pass in a
// narrow mutator without depending on the concrete Store.
type MembershipRemover interface {
	RemoveSession(rootSession string) error
}

// KillSession kills a tmux session and removes it from workspace membership
// once no live session shares its root. Grouped clones (dev-b, dev-c)
// keep the root in the workspace even if dev itself was the target — so
// killing the root while a clone is alive doesn't orphan that clone.
//
// All session-kill paths should funnel through here so the workspace store
// stays in sync with live tmux state. The status bar reads SessionsIn(ws),
// which is unfiltered; stale entries surface as ghost sibling pills.
func KillSession(runner tmux.Runner, store MembershipRemover, name string) error {
	if err := session.Kill(runner, name); err != nil {
		return err
	}
	if store == nil {
		return nil
	}
	root := session.RootName(name)
	if hasLiveSessionInGroup(runner, root) {
		return nil
	}
	if err := store.RemoveSession(root); err != nil {
		return fmt.Errorf("workspace cleanup: %w", err)
	}
	return nil
}

// hasLiveSessionInGroup returns true when any live tmux session shares the
// given root (i.e. dev OR dev-b OR dev-c…). Used to decide whether the root
// is still represented in tmux after a kill — if so, workspace membership
// must stay so the surviving clone keeps its workspace context.
func hasLiveSessionInGroup(runner tmux.Runner, root string) bool {
	sessions, err := runner.ListSessions()
	if err != nil {
		// Conservative: if we can't enumerate, fall back to a plain root
		// existence check so we don't accidentally orphan anything.
		return runner.HasSession(root)
	}
	for _, s := range sessions {
		if session.RootName(s.Name) == root {
			return true
		}
	}
	return false
}
