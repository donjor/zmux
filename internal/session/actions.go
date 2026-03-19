package session

import (
	"fmt"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// Create creates a new tmux session with the given name and working directory.
func Create(runner tmux.Runner, name, dir string) error {
	if runner.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}
	return runner.NewSession(name, dir)
}

// CreateFromTemplate creates a session using a Template definition.
// It creates the session with the first window, then adds remaining windows
// and sends any configured commands.
//
// Note: tmux base-index is 1 (set by zmux conf), so the first window is index 1.
func CreateFromTemplate(runner tmux.Runner, tmpl Template, name, dir string) error {
	if runner.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}

	if len(tmpl.Windows) == 0 {
		return runner.NewSession(name, dir)
	}

	// Create session — tmux creates it with one default window at base-index (1).
	if err := runner.NewSession(name, dir); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Rename the first window by targeting index 1 (base-index).
	if err := runner.RenameWindow(name, "1", tmpl.Windows[0].Name); err != nil {
		return fmt.Errorf("rename first window: %w", err)
	}

	// Send command to first window if specified.
	if tmpl.Windows[0].Command != "" {
		target := fmt.Sprintf("%s:%s", name, tmpl.Windows[0].Name)
		if err := runner.SendKeys(target, tmpl.Windows[0].Command, "Enter"); err != nil {
			return fmt.Errorf("send keys to %s: %w", target, err)
		}
	}

	// Create additional windows (they get indices 2, 3, ...).
	for _, w := range tmpl.Windows[1:] {
		if err := runner.NewWindow(name, w.Name, dir); err != nil {
			return fmt.Errorf("create window %q: %w", w.Name, err)
		}
		if w.Command != "" {
			target := fmt.Sprintf("%s:%s", name, w.Name)
			if err := runner.SendKeys(target, w.Command, "Enter"); err != nil {
				return fmt.Errorf("send keys to %s: %w", target, err)
			}
		}
	}

	// Select the focus window by name.
	// Window index = base-index (1) + slice position.
	if tmpl.Options.Focus != "" {
		for i, w := range tmpl.Windows {
			if w.Name == tmpl.Options.Focus {
				if err := runner.SelectWindow(name, i+1); err != nil {
					return fmt.Errorf("select focus window %q: %w", w.Name, err)
				}
				break
			}
		}
	}

	return nil
}

// Attach connects to the named session. If already inside tmux, uses
// SwitchClient. If outside tmux and the session is already attached
// elsewhere, creates a grouped session (shared windows, independent viewport)
// named <session>-b, -c, -d, etc. Cleaned up automatically on detach.
func Attach(runner tmux.Runner, name string) error {
	if runner.IsInsideTmux() {
		return runner.SwitchClient(name)
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
				err := runner.AttachSession(groupName)
				// Clean up grouped session after detach.
				_ = runner.KillSession(groupName)
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
	for c := 'b'; c <= 'z'; c++ {
		candidate := fmt.Sprintf("%s-%c", base, c)
		if !taken[candidate] {
			return candidate
		}
	}
	// Fallback if somehow a-z are all taken.
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano()%10000)
}

// Switch switches the current tmux client to the named session.
func Switch(runner tmux.Runner, name string) error {
	return runner.SwitchClient(name)
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
