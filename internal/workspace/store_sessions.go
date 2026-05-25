package workspace

import (
	"fmt"
	"time"

	"github.com/donjor/zmux/internal/session"
)

// AddSession adds a root session to a workspace's session list.
// Creates the workspace if it doesn't exist.
func (s *Store) AddSession(workspace, rootSession string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}

	// Check if session is already in another workspace.
	if ws, found := st.WorkspaceForSession(rootSession); found {
		if ws.Name == workspace {
			return nil // already in this workspace
		}
		return fmt.Errorf("session %q already in workspace %q — use MoveSession to reassign", rootSession, ws.Name)
	}

	ws, ok := st.Workspaces[workspace]
	if !ok {
		now := time.Now()
		ws = &Workspace{
			Name:      workspace,
			Sessions:  []string{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		st.Workspaces[workspace] = ws
	}

	ws.Sessions = append(ws.Sessions, rootSession)
	ws.UpdatedAt = time.Now()
	return s.Save(st)
}

// RemoveSession removes a root session from its workspace.
func (s *Store) RemoveSession(rootSession string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	ws, found := st.WorkspaceForSession(rootSession)
	if !found {
		return nil // not tracked
	}
	ws.Sessions = removeString(ws.Sessions, rootSession)
	if ws.LastActiveSession == rootSession && len(ws.Sessions) > 0 {
		ws.LastActiveSession = ws.Sessions[0]
	} else if len(ws.Sessions) == 0 {
		ws.LastActiveSession = ""
	}
	ws.UpdatedAt = time.Now()
	return s.Save(st)
}

// MoveSession moves a root session from its current workspace to a new one.
func (s *Store) MoveSession(rootSession, destWorkspace string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}

	// Remove from current workspace.
	if ws, found := st.WorkspaceForSession(rootSession); found {
		ws.Sessions = removeString(ws.Sessions, rootSession)
		if ws.LastActiveSession == rootSession {
			if len(ws.Sessions) > 0 {
				ws.LastActiveSession = ws.Sessions[0]
			} else {
				ws.LastActiveSession = ""
			}
		}
		ws.UpdatedAt = time.Now()
	}

	// Add to destination.
	destWS, ok := st.Workspaces[destWorkspace]
	if !ok {
		now := time.Now()
		destWS = &Workspace{
			Name:      destWorkspace,
			Sessions:  []string{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		st.Workspaces[destWorkspace] = destWS
	}
	destWS.Sessions = append(destWS.Sessions, rootSession)
	destWS.UpdatedAt = time.Now()

	return s.Save(st)
}

// RenameSession updates a session name across workspace membership and last_active.
func (s *Store) RenameSession(oldRoot, newRoot string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	ws, found := st.WorkspaceForSession(oldRoot)
	if !found {
		return nil // not tracked
	}
	for i, sess := range ws.Sessions {
		if sess == oldRoot {
			ws.Sessions[i] = newRoot
			break
		}
	}
	if ws.LastActiveSession == oldRoot {
		ws.LastActiveSession = newRoot
	}
	ws.UpdatedAt = time.Now()
	return s.Save(st)
}

// WorkspaceFor returns the workspace name for a session.
// Grouped sessions (e.g. dev-b) are resolved to their root before lookup.
func (s *Store) WorkspaceFor(name string) (string, bool) {
	st, err := s.Load()
	if err != nil {
		return "", false
	}
	root := session.RootName(name)
	ws, found := st.WorkspaceForSession(root)
	if !found {
		return "", false
	}
	return ws.Name, true
}

// SessionsIn returns ordered session names in a workspace.
func (s *Store) SessionsIn(workspace string) []string {
	st, err := s.Load()
	if err != nil {
		return nil
	}
	ws, ok := st.Workspaces[workspace]
	if !ok {
		return nil
	}
	out := make([]string, len(ws.Sessions))
	copy(out, ws.Sessions)
	return out
}

// SessionPosition returns (1-based position, total count) for a session
// within its workspace. Returns ok=false if session is not tracked.
func (s *Store) SessionPosition(sessionName string) (pos, count int, ok bool) {
	st, err := s.Load()
	if err != nil {
		return 0, 0, false
	}
	root := session.RootName(sessionName)
	ws, found := st.WorkspaceForSession(root)
	if !found {
		return 0, 0, false
	}
	for i, sess := range ws.Sessions {
		if sess == root {
			return i + 1, len(ws.Sessions), true
		}
	}
	return 0, 0, false
}
