package workspace

import "time"

// Workspace is a first-class project container.
type Workspace struct {
	Name                string             `toml:"-"` // populated from map key
	RootDir             string             `toml:"root_dir,omitempty"`
	LastActiveSessionID string             `toml:"last_active_session_id,omitempty"`
	Sessions            []WorkspaceSession `toml:"sessions"`
	CreatedAt           time.Time          `toml:"created_at,omitempty"`
	UpdatedAt           time.Time          `toml:"updated_at,omitempty"`

	// LastActiveSession is derived for legacy callers/tests that still inspect
	// the loaded struct directly. It is not persisted; store helpers use IDs.
	LastActiveSession string `toml:"-"`
}

// WorkspaceSession is the durable identity record for one workspace-local
// session label. TmuxName is the generated raw tmux session name.
type WorkspaceSession struct {
	ID             string    `toml:"id"`
	Label          string    `toml:"label"`
	TmuxName       string    `toml:"tmux_name"`
	LegacyTmuxName string    `toml:"legacy_tmux_name,omitempty"`
	CreatedAt      time.Time `toml:"created_at,omitempty"`
	UpdatedAt      time.Time `toml:"updated_at,omitempty"`
}

// StateV3 is the on-disk format of workspaces.toml v3.
type StateV3 struct {
	Version    int                   `toml:"version"`
	Workspaces map[string]*Workspace `toml:"workspaces"`
}

// emptyStateV3 returns a fresh empty v3 state.
func emptyStateV3() StateV3 {
	return StateV3{
		Version:    3,
		Workspaces: make(map[string]*Workspace),
	}
}

// populateNames sets Workspace.Name from map keys and fills derived fields.
func (st *StateV3) populateNames() {
	for name, ws := range st.Workspaces {
		if ws != nil {
			ws.Name = name
			ws.populateDerived()
		}
	}
}

func (ws *Workspace) populateDerived() {
	ws.LastActiveSession = ""
	if ws.LastActiveSessionID == "" {
		return
	}
	for _, sess := range ws.Sessions {
		if sess.ID == ws.LastActiveSessionID {
			ws.LastActiveSession = sess.Label
			return
		}
	}
}

// HasSession returns true if a root session exists in any workspace.
func (st *StateV3) HasSession(rootSession string) bool {
	for _, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, s := range ws.Sessions {
			if s.TmuxName == rootSession || s.Label == rootSession || s.ID == rootSession || s.LegacyTmuxName == rootSession {
				return true
			}
		}
	}
	return false
}

// WorkspaceForSession returns the workspace that contains a root session.
func (st *StateV3) WorkspaceForSession(rootSession string) (*Workspace, bool) {
	for _, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, s := range ws.Sessions {
			if s.TmuxName == rootSession || s.Label == rootSession || s.ID == rootSession || s.LegacyTmuxName == rootSession {
				return ws, true
			}
		}
	}
	return nil, false
}

// SessionFor resolves a session record by raw tmux name, local label, or ID.
func (st *StateV3) SessionFor(name string) (*Workspace, WorkspaceSession, bool) {
	for _, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, sess := range ws.Sessions {
			if sess.TmuxName == name || sess.Label == name || sess.ID == name || sess.LegacyTmuxName == name {
				return ws, sess, true
			}
		}
	}
	return nil, WorkspaceSession{}, false
}
