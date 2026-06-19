package session

import (
	"fmt"
	"time"

	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tmux"
)

// Create creates a new tmux session with the given name and working directory.
func Create(runner tmux.Runner, name, dir string) error {
	if runner.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}
	return runner.NewSession(name, dir)
}

// Attach connects to the named session. If already inside tmux, delegates to
// SwitchView so an in-tmux switch to an already-attached session gets its own
// independent viewport (grouped clone) instead of sharing the view. If outside
// tmux and the session is already attached elsewhere, creates a grouped session
// (shared windows, independent viewport) named <session>-b, -c, -d, etc.
// Cleaned up automatically on detach.
func Attach(runner tmux.Runner, name string) error {
	if runner.IsInsideTmux() {
		_, err := SwitchView(runner, name)
		return err
	}

	// Check if session is already attached.
	sessions, err := runner.ListSessions()
	if err == nil {
		for _, s := range sessions {
			if s.Name == name && s.Attached {
				groupName := nextGroupName(runner, name)
				if err := runner.NewGroupedSession(name, groupName); err != nil {
					// Fallback to regular attach (mirrored).
					return runner.AttachSession(name)
				}
				markClone(runner, groupName)
				err := runner.AttachSession(groupName)
				// Clean up grouped session after detach.
				if killErr := runner.KillSession(groupName); killErr != nil {
					debug.Log("cleanup grouped session %s: %v", groupName, killErr)
				}
				return err
			}
		}
	}

	return runner.AttachSession(name)
}

// AttachMirror attaches to a session with a literal shared view — both clients
// see the exact same terminal. Useful for agent/user shared terminals where
// both need to see output and can type.
func AttachMirror(runner tmux.Runner, name string) error {
	if runner.IsInsideTmux() {
		return runner.SwitchClient(name)
	}
	// Plain tmux attach (no -d, no grouped session) = mirror mode.
	return runner.AttachSession(name)
}

// AttachHijack forcefully attaches to a session, detaching any existing clients.
// Use when you want to steal a session (e.g., dead SSH connection left it attached).
func AttachHijack(runner tmux.Runner, name string) error {
	if runner.IsInsideTmux() {
		return runner.SwitchClient(name)
	}
	// tmux attach -d detaches other clients.
	return runner.AttachSessionDetach(name)
}

// nextGroupName finds the next available grouped session suffix: name-b, name-c, etc.
func nextGroupName(runner tmux.Runner, base string) string {
	sessions, err := runner.ListSessions()
	taken := map[string]bool{}
	if err == nil {
		for _, s := range sessions {
			taken[s.Name] = true
		}
	}
	cloneFormat := "%s-%c"
	if isManagedRawSession(base) {
		cloneFormat = "%s__clone_%c"
	}
	for c := 'b'; c <= 'z'; c++ {
		candidate := fmt.Sprintf(cloneFormat, base, c)
		if !taken[candidate] {
			return candidate
		}
	}
	// Fallback if somehow a-z are all taken.
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano()%10000)
}

func isManagedRawSession(name string) bool {
	return len(name) > 4 && name[:4] == "zws_"
}

// Switch switches the current tmux client to the named session. It is the plain
// switch-client wrapper; callers that want per-client view independence within a
// workspace should use SwitchView instead.
func Switch(runner tmux.Runner, name string) error {
	return runner.SwitchClient(name)
}

// SwitchView switches the current client to a view of target while preserving
// per-client view independence. If target's root session is already attached by
// another client, this client is switched to a fresh session-group clone — an
// independent viewport (own current window / active pane) over the shared window
// set — instead of collapsing onto the shared view. Any clone the client leaves
// behind is garbage-collected once it has no remaining clients.
//
// It returns the raw tmux session the client ended up on: the root target for a
// plain switch, or the generated clone name when a clone was created. Callers
// that perform a follow-up window/pane selection MUST target the returned
// session, not the logical target — selecting on the root would mutate the
// shared view and undo the independence.
func SwitchView(runner tmux.Runner, target string) (string, error) {
	// Current raw session, best-effort. On read failure we skip the no-op guard
	// and the leftover-clone GC, but never fail the switch over it.
	prev, _ := runner.DisplayMessage("", "#{session_name}")

	sessions, listErr := runner.ListSessions()

	// Resolve the logical root. Only strip a clone suffix when target is itself a
	// zmux clone — a standalone session merely named like a clone (e.g. "foo-b")
	// must be switched to literally, not redirected to "foo".
	root := target
	if isZmuxClone(sessions, target) {
		root = RootName(target)
	}

	// Already viewing this root: the exact root session, or a zmux-created clone
	// of it. A session merely *named* like a clone (a standalone "foo-b") is NOT
	// a view of foo, so it must fall through to a real switch — hence the
	// provenance check, not just a name match.
	if prev == root || (prev != "" && RootName(prev) == root && isZmuxClone(sessions, prev)) {
		return prev, nil
	}

	// Clone only when the root is already attached by some other client.
	dest := root
	if listErr == nil && sessionAttached(sessions, root) {
		clone := nextGroupName(runner, root)
		if gerr := runner.NewGroupedSession(root, clone); gerr == nil {
			markClone(runner, clone)
			dest = clone
		}
		// On clone-create failure, fall back to a plain switch to the root.
	}

	if serr := runner.SwitchClient(dest); serr != nil {
		if dest != root {
			// The clone was created detached and never became a live view.
			if kerr := runner.KillSession(dest); kerr != nil {
				debug.Log("switchview: kill orphaned clone %s: %v", dest, kerr)
			}
		}
		return "", serr
	}

	// Secondary teardown: a freshly created clone should evaporate when its last
	// client leaves, covering exits the GC below cannot see (terminal/client
	// close, raw-tmux switch). Must be armed AFTER the switch — arming while the
	// clone is still detached would destroy it immediately.
	if dest != root {
		if oerr := runner.SetSessionOption(dest, "destroy-unattached", "on"); oerr != nil {
			debug.Log("switchview: arm destroy-unattached on %s: %v", dest, oerr)
		}
	}

	// Primary teardown: GC the clone we just left, if any, once it is clientless.
	// This is what keeps clones from accumulating as the client moves between
	// sessions in a workspace.
	gcLeftClone(runner, prev)

	return dest, nil
}

// optionClone is the per-session marker zmux stamps on the ephemeral group
// clones it creates for independent viewports. It is the provenance signal that
// makes a clone safe to garbage-collect: tmux's own session_group is also set
// on sessions a user grouped by hand (tmux new-session -t foo -s foo-b), so the
// group alone is not proof zmux owns the session.
const optionClone = "@zmux_clone"

// markClone stamps the zmux-clone provenance marker on a freshly created clone.
func markClone(runner tmux.Runner, name string) {
	if err := runner.SetSessionOption(name, optionClone, "1"); err != nil {
		debug.Log("switchview: mark clone %s: %v", name, err)
	}
}

// sessionAttached reports whether the named session is currently attached.
func sessionAttached(sessions []tmux.Session, name string) bool {
	for _, s := range sessions {
		if s.Name == name {
			return s.Attached
		}
	}
	return false
}

// isZmuxClone reports whether the named session is a zmux-created group clone.
func isZmuxClone(sessions []tmux.Session, name string) bool {
	for _, s := range sessions {
		if s.Name == name {
			return s.Clone
		}
	}
	return false
}

// gcLeftClone kills prev when it is a zmux-created clone that no longer has any
// attached client. The @zmux_clone marker is the provenance guard: a session a
// user grouped by hand (or merely named like a clone) is never killed.
func gcLeftClone(runner tmux.Runner, prev string) {
	if prev == "" || RootName(prev) == prev {
		return
	}
	sessions, err := runner.ListSessions()
	if err != nil {
		return
	}
	for _, s := range sessions {
		if s.Name != prev {
			continue
		}
		if !s.Clone || s.Attached {
			return
		}
		if kerr := runner.KillSession(prev); kerr != nil {
			debug.Log("switchview: gc left clone %s: %v", prev, kerr)
		}
		return
	}
}

// Kill terminates the named session.
func Kill(runner tmux.Runner, name string) error {
	return runner.KillSession(name)
}

// Rename renames a session from old to new.
func Rename(runner tmux.Runner, old, newName string) error {
	return runner.RenameSession(old, newName)
}

// MoveWindow moves the current window from srcSession to dstSession.
func MoveWindow(runner tmux.Runner, srcSession, dstSession string) error {
	return runner.MoveWindow(srcSession, dstSession)
}

// CleanupTmp kills all unattached tmp-N sessions and returns the names
// of sessions that were killed.
func CleanupTmp(runner tmux.Runner) ([]string, error) {
	sessions, err := runner.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var killed []string
	for _, s := range sessions {
		if IsTemp(s.Name) && !s.Attached {
			if err := runner.KillSession(s.Name); err != nil {
				return killed, fmt.Errorf("kill session %q: %w", s.Name, err)
			}
			killed = append(killed, s.Name)
		}
	}

	return killed, nil
}
