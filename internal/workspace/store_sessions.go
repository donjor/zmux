package workspace

import (
	"fmt"
	"time"

	"github.com/donjor/zmux/internal/session"
)

// AddSession adds a workspace-local session label to a workspace's session list.
// Creates the workspace if it doesn't exist.
func (s *Store) AddSession(workspace, label string) error {
	rec, err := NewSessionRecord(workspace, label)
	if err != nil {
		return err
	}
	return s.AddSessionRecord(workspace, rec)
}

// AddSessionRecord adds a fully resolved session identity record to a workspace.
func (s *Store) AddSessionRecord(workspace string, rec WorkspaceSession) error {
	st, err := s.Load()
	if err != nil {
		return err
	}

	if rec.ID == "" || rec.Label == "" || rec.TmuxName == "" {
		return fmt.Errorf("invalid session record for workspace %q", workspace)
	}

	// Check if session is already in another workspace by durable identity or raw tmux name.
	if ws, _, found := st.SessionFor(rec.ID); found {
		if ws.Name == workspace {
			return nil
		}
		return fmt.Errorf("session %q already in workspace %q — use MoveSession to reassign", rec.Label, ws.Name)
	}
	if ws, existing, found := st.SessionFor(rec.TmuxName); found {
		if ws.Name == workspace {
			if existing.Label == rec.Label {
				return nil
			}
			return fmt.Errorf("tmux session %q already tracked in workspace %q as label %q", rec.TmuxName, workspace, existing.Label)
		}
		return fmt.Errorf("session %q already in workspace %q — use MoveSession to reassign", rec.TmuxName, ws.Name)
	}
	if ws, existing, found := st.SessionFor(rec.Label); found {
		if ws.Name == workspace {
			return nil // already in this workspace
		}
		// Labels are local to a workspace, so this is only an error if the raw
		// name or ID matched. A matching label elsewhere is fine.
		_ = existing
	}

	ws, ok := st.Workspaces[workspace]
	if !ok {
		now := time.Now()
		ws = &Workspace{
			Name:      workspace,
			Sessions:  []WorkspaceSession{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		st.Workspaces[workspace] = ws
	}

	if _, _, found := findSessionRecord(ws.Sessions, rec.Label); found {
		return nil
	}
	ws.Sessions = append(ws.Sessions, rec)
	ws.UpdatedAt = time.Now()
	ws.populateDerived()
	return s.Save(st)
}

// RemoveSession removes a session from its workspace by raw tmux name, label, or ID.
func (s *Store) RemoveSession(key string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	key = session.RootName(key)
	ws, rec, found := st.SessionFor(key)
	if !found {
		return nil // not tracked
	}
	ws.Sessions = removeSessionRecord(ws.Sessions, rec.ID)
	if ws.LastActiveSessionID == rec.ID && len(ws.Sessions) > 0 {
		ws.LastActiveSessionID = ws.Sessions[0].ID
	} else if len(ws.Sessions) == 0 {
		ws.LastActiveSessionID = ""
	}
	ws.UpdatedAt = time.Now()
	ws.populateDerived()
	return s.Save(st)
}

// MoveSession moves a session from its current workspace to a new one.
func (s *Store) MoveSession(key, destWorkspace string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if err := ValidateWorkspaceName(destWorkspace); err != nil {
		return err
	}

	key = session.RootName(key)
	var rec WorkspaceSession
	if ws, foundRec, found := st.SessionFor(key); found {
		rec = foundRec
		ws.Sessions = removeSessionRecord(ws.Sessions, rec.ID)
		if ws.LastActiveSessionID == rec.ID {
			if len(ws.Sessions) > 0 {
				ws.LastActiveSessionID = ws.Sessions[0].ID
			} else {
				ws.LastActiveSessionID = ""
			}
		}
		ws.UpdatedAt = time.Now()
		ws.populateDerived()
	} else {
		rec, err = NewSessionRecord(destWorkspace, key)
		if err != nil {
			return err
		}
	}

	// Add to destination.
	destWS, ok := st.Workspaces[destWorkspace]
	if !ok {
		now := time.Now()
		destWS = &Workspace{
			Name:      destWorkspace,
			Sessions:  []WorkspaceSession{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		st.Workspaces[destWorkspace] = destWS
	}
	if rec.TmuxName == "" || rec.Label == "" || rec.ID == "" {
		rec, err = NewSessionRecord(destWorkspace, key)
		if err != nil {
			return err
		}
	}
	destWS.Sessions = append(destWS.Sessions, rec)
	destWS.UpdatedAt = time.Now()
	destWS.populateDerived()

	return s.Save(st)
}

// RenameSession updates a workspace-local session label and generated tmux name.
func (s *Store) RenameSession(oldKey, newLabel string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if err := ValidateSessionLabel(newLabel); err != nil {
		return err
	}
	oldKey = session.RootName(oldKey)
	ws, rec, found := st.SessionFor(oldKey)
	if !found {
		return nil // not tracked
	}
	newRec, err := NewSessionRecord(ws.Name, newLabel)
	if err != nil {
		return err
	}
	newRec.CreatedAt = rec.CreatedAt
	for i, sess := range ws.Sessions {
		if sess.ID == rec.ID {
			ws.Sessions[i] = newRec
			break
		}
	}
	if ws.LastActiveSessionID == rec.ID {
		ws.LastActiveSessionID = newRec.ID
	}
	ws.UpdatedAt = time.Now()
	ws.populateDerived()
	return s.Save(st)
}

// LegacySessionRecordFor resolves a live legacy raw tmux name to the v3 record
// that should replace it. It returns false on ambiguous fallback matches.
func (s *Store) LegacySessionRecordFor(raw string) (string, WorkspaceSession, bool) {
	st, err := s.Load()
	if err != nil {
		return "", WorkspaceSession{}, false
	}
	var (
		foundWS  string
		foundRec WorkspaceSession
		matches  int
	)
	for wsName, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, rec := range ws.Sessions {
			if rec.LegacyTmuxName == raw || (rec.LegacyTmuxName == "" && legacyRawCandidate(wsName, rec, raw)) {
				foundWS = wsName
				foundRec = rec
				matches++
			}
		}
	}
	if matches == 1 {
		return foundWS, foundRec, true
	}
	return "", WorkspaceSession{}, false
}

// SessionRecordForTmuxName resolves a record by generated raw tmux name only.
func (s *Store) SessionRecordForTmuxName(raw string) (string, WorkspaceSession, bool) {
	st, err := s.Load()
	if err != nil {
		return "", WorkspaceSession{}, false
	}
	for wsName, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, rec := range ws.Sessions {
			if rec.TmuxName == raw {
				return wsName, rec, true
			}
		}
	}
	return "", WorkspaceSession{}, false
}

// ClearLegacySessionName removes the one-shot legacy rename hint after the live
// tmux session has been renamed and stamped with managed metadata.
func (s *Store) ClearLegacySessionName(workspaceName, sessionID string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	ws, ok := st.Workspaces[workspaceName]
	if !ok {
		return nil
	}
	for i, rec := range ws.Sessions {
		if rec.ID == sessionID {
			if rec.LegacyTmuxName == "" {
				return nil
			}
			ws.Sessions[i].LegacyTmuxName = ""
			ws.Sessions[i].UpdatedAt = time.Now()
			ws.UpdatedAt = time.Now()
			ws.populateDerived()
			return s.Save(st)
		}
	}
	return nil
}

func legacyRawCandidate(wsName string, rec WorkspaceSession, raw string) bool {
	if rec.Label == raw {
		return true
	}
	return rec.Label == "main" && raw == "main-"+wsName
}

// WorkspaceFor returns the workspace name for a session.
// Grouped sessions (e.g. dev-b) are resolved to their root before lookup.
func (s *Store) WorkspaceFor(name string) (string, bool) {
	st, err := s.Load()
	if err != nil {
		return "", false
	}
	root := session.RootName(name)
	ws, _, found := st.SessionFor(root)
	if !found {
		return "", false
	}
	return ws.Name, true
}

// SessionLabelsIn returns ordered workspace-local session labels in a workspace.
func (s *Store) SessionLabelsIn(workspace string) []string {
	st, err := s.Load()
	if err != nil {
		return nil
	}
	ws, ok := st.Workspaces[workspace]
	if !ok {
		return nil
	}
	return sessionLabels(ws.Sessions)
}

// SessionTargetsIn returns ordered raw tmux session names in a workspace.
func (s *Store) SessionTargetsIn(workspace string) []string {
	st, err := s.Load()
	if err != nil {
		return nil
	}
	ws, ok := st.Workspaces[workspace]
	if !ok {
		return nil
	}
	return sessionTargets(ws.Sessions)
}

// SessionRecord resolves a session by workspace-local label, raw tmux name, or ID.
func (s *Store) SessionRecord(workspace, key string) (WorkspaceSession, bool) {
	st, err := s.Load()
	if err != nil {
		return WorkspaceSession{}, false
	}
	ws, ok := st.Workspaces[workspace]
	if !ok {
		return WorkspaceSession{}, false
	}
	rec, _, found := findSessionRecord(ws.Sessions, session.RootName(key))
	return rec, found
}

// SessionRecordFor resolves a session by raw tmux name, label, or ID globally.
func (s *Store) SessionRecordFor(key string) (string, WorkspaceSession, bool) {
	st, err := s.Load()
	if err != nil {
		return "", WorkspaceSession{}, false
	}
	ws, rec, found := st.SessionFor(session.RootName(key))
	if !found {
		return "", WorkspaceSession{}, false
	}
	return ws.Name, rec, true
}

// SessionPosition returns (1-based position, total count) for a session
// within its workspace. Returns ok=false if session is not tracked.
func (s *Store) SessionPosition(sessionName string) (pos, count int, ok bool) {
	st, err := s.Load()
	if err != nil {
		return 0, 0, false
	}
	root := session.RootName(sessionName)
	ws, rec, found := st.SessionFor(root)
	if !found {
		return 0, 0, false
	}
	for i, sess := range ws.Sessions {
		if sess.ID == rec.ID {
			return i + 1, len(ws.Sessions), true
		}
	}
	return 0, 0, false
}
