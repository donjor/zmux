package workspace

import (
	"time"

	"github.com/donjor/zmux/internal/session"
)

// SetLastActive updates the last-active session for a workspace.
func (s *Store) SetLastActive(workspace, sessionName string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	ws, ok := st.Workspaces[workspace]
	if !ok {
		return nil // workspace not found, noop
	}
	root := session.RootName(sessionName)
	ws.LastActiveSession = root
	ws.UpdatedAt = time.Now()
	return s.Save(st)
}

// Reconcile synchronizes workspace state with live tmux sessions.
// - Removes dead session memberships (but keeps workspace objects).
// - Auto-heals live unmanaged root sessions into same-named workspaces.
// - Does nothing if liveRoots is empty (tmux failure protection).
func (s *Store) Reconcile(liveRoots map[string]bool) error {
	if len(liveRoots) == 0 {
		return nil // tmux failure protection
	}

	st, err := s.Load()
	if err != nil {
		return err
	}

	changed := false

	// Remove dead sessions from workspace membership.
	for _, ws := range st.Workspaces {
		var alive []string
		for _, sess := range ws.Sessions {
			if liveRoots[sess] {
				alive = append(alive, sess)
			} else {
				changed = true
			}
		}
		if len(alive) != len(ws.Sessions) {
			ws.Sessions = alive
			ws.UpdatedAt = time.Now()
			// Clear last_active if it was removed.
			if ws.LastActiveSession != "" && !liveRoots[ws.LastActiveSession] {
				if len(alive) > 0 {
					ws.LastActiveSession = alive[0]
				} else {
					ws.LastActiveSession = ""
				}
			}
		}
	}

	// Auto-heal: live root sessions not in any workspace get their own.
	for rootName := range liveRoots {
		if !st.HasSession(rootName) {
			now := time.Now()
			wsName := rootName
			ws, ok := st.Workspaces[wsName]
			if !ok {
				ws = &Workspace{
					Name:      wsName,
					Sessions:  []string{},
					CreatedAt: now,
					UpdatedAt: now,
				}
				st.Workspaces[wsName] = ws
			}
			ws.Sessions = append(ws.Sessions, rootName)
			if ws.LastActiveSession == "" {
				ws.LastActiveSession = rootName
			}
			ws.UpdatedAt = now
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return s.Save(st)
}
