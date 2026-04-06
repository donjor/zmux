package workspace

import "time"

// Workspace is a first-class project container.
type Workspace struct {
	Name              string    `toml:"-"`                              // populated from map key
	RootDir           string    `toml:"root_dir,omitempty"`
	LastActiveSession string    `toml:"last_active_session,omitempty"`
	Sessions          []string  `toml:"sessions"`
	CreatedAt         time.Time `toml:"created_at,omitempty"`
	UpdatedAt         time.Time `toml:"updated_at,omitempty"`
}

// StateV2 is the on-disk format of workspaces.toml v2.
type StateV2 struct {
	Version    int                   `toml:"version"`
	Workspaces map[string]*Workspace `toml:"workspaces"`
}

// emptyStateV2 returns a fresh empty v2 state.
func emptyStateV2() StateV2 {
	return StateV2{
		Version:    2,
		Workspaces: make(map[string]*Workspace),
	}
}

// populateNames sets Workspace.Name from map keys.
func (st *StateV2) populateNames() {
	for name, ws := range st.Workspaces {
		if ws != nil {
			ws.Name = name
		}
	}
}

// HasSession returns true if a root session exists in any workspace.
func (st *StateV2) HasSession(rootSession string) bool {
	for _, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, s := range ws.Sessions {
			if s == rootSession {
				return true
			}
		}
	}
	return false
}

// WorkspaceForSession returns the workspace that contains a root session.
func (st *StateV2) WorkspaceForSession(rootSession string) (*Workspace, bool) {
	for _, ws := range st.Workspaces {
		if ws == nil {
			continue
		}
		for _, s := range ws.Sessions {
			if s == rootSession {
				return ws, true
			}
		}
	}
	return nil, false
}
