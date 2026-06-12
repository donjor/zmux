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
	rec, _, found := findSessionRecord(ws.Sessions, root)
	if !found {
		return nil
	}
	ws.LastActiveSessionID = rec.ID
	ws.UpdatedAt = time.Now()
	ws.populateDerived()
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
		var alive []WorkspaceSession
		for _, sess := range ws.Sessions {
			if liveRoots[sess.TmuxName] || (sess.LegacyTmuxName != "" && liveRoots[sess.LegacyTmuxName]) {
				alive = append(alive, sess)
			} else {
				changed = true
			}
		}
		if len(alive) != len(ws.Sessions) {
			ws.Sessions = alive
			ws.UpdatedAt = time.Now()
			// Clear last_active if it was removed.
			if ws.LastActiveSessionID != "" {
				_, _, found := findSessionRecord(alive, ws.LastActiveSessionID)
				if found {
					ws.populateDerived()
					continue
				}
				if len(alive) > 0 {
					ws.LastActiveSessionID = alive[0].ID
				} else {
					ws.LastActiveSessionID = ""
				}
				ws.populateDerived()
			}
		}
	}

	// Auto-heal: live root sessions not in any workspace get their own.
	for rootName := range liveRoots {
		if st.HasSession(rootName) {
			continue
		}

		now := time.Now()
		wsName := rootName
		label := rootName
		rec := WorkspaceSession{
			ID:        StableSessionID(wsName, label),
			Label:     label,
			TmuxName:  rootName,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if parsedWS, parsedLabel, ok := ParseRawSessionName(rootName); ok {
			wsName = parsedWS
			label = parsedLabel
			parsedRec, err := NewSessionRecord(wsName, label)
			if err == nil {
				rec = parsedRec
			}
		}

		ws, ok := st.Workspaces[wsName]
		if !ok {
			ws = &Workspace{
				Name:      wsName,
				Sessions:  []WorkspaceSession{},
				CreatedAt: now,
				UpdatedAt: now,
			}
			st.Workspaces[wsName] = ws
		}
		ws.Sessions = append(ws.Sessions, rec)
		if ws.LastActiveSessionID == "" {
			ws.LastActiveSessionID = rec.ID
		}
		ws.UpdatedAt = now
		ws.populateDerived()
		changed = true
	}

	if !changed {
		return nil
	}
	return s.Save(st)
}
