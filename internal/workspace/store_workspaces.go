package workspace

import (
	"fmt"
	"sort"
	"time"
)

// GetWorkspace returns the workspace by name, or nil if not found.
func (s *Store) GetWorkspace(name string) (*Workspace, error) {
	st, err := s.Load()
	if err != nil {
		return nil, err
	}
	ws := st.Workspaces[name]
	return ws, nil
}

// CreateWorkspace creates a new workspace with the given metadata.
// Returns error if it already exists or if the name is invalid.
func (s *Store) CreateWorkspace(name, rootDir string) error {
	if err := ValidateWorkspaceName(name); err != nil {
		return err
	}
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := st.Workspaces[name]; ok {
		return fmt.Errorf("workspace %q already exists", name)
	}
	now := time.Now()
	st.Workspaces[name] = &Workspace{
		Name:      name,
		RootDir:   rootDir,
		Sessions:  []WorkspaceSession{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.Save(st)
}

// EnsureWorkspace creates a workspace if it doesn't exist, or returns the existing one.
func (s *Store) EnsureWorkspace(name, rootDir string) (*Workspace, error) {
	if err := ValidateWorkspaceName(name); err != nil {
		return nil, err
	}
	st, err := s.Load()
	if err != nil {
		return nil, err
	}
	ws, ok := st.Workspaces[name]
	if ok {
		return ws, nil
	}
	now := time.Now()
	ws = &Workspace{
		Name:      name,
		RootDir:   rootDir,
		Sessions:  []WorkspaceSession{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	st.Workspaces[name] = ws
	if err := s.Save(st); err != nil {
		return nil, err
	}
	return ws, nil
}

// DeleteWorkspace removes a workspace and all its session memberships.
func (s *Store) DeleteWorkspace(name string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := st.Workspaces[name]; !ok {
		return fmt.Errorf("workspace %q not found", name)
	}
	delete(st.Workspaces, name)
	return s.Save(st)
}

// RenameWorkspace renames a workspace.
func (s *Store) RenameWorkspace(old, new string) error {
	if err := ValidateWorkspaceName(new); err != nil {
		return err
	}
	st, err := s.Load()
	if err != nil {
		return err
	}
	ws, ok := st.Workspaces[old]
	if !ok {
		return fmt.Errorf("workspace %q not found", old)
	}
	if _, exists := st.Workspaces[new]; exists {
		return fmt.Errorf("workspace %q already exists", new)
	}
	ws.Name = new
	ws.UpdatedAt = time.Now()
	st.Workspaces[new] = ws
	delete(st.Workspaces, old)
	return s.Save(st)
}

// ListWorkspaces returns all workspaces sorted by name.
func (s *Store) ListWorkspaces() ([]Workspace, error) {
	st, err := s.Load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(st.Workspaces))
	for name := range st.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Workspace, 0, len(names))
	for _, name := range names {
		ws := st.Workspaces[name]
		ws.Name = name
		out = append(out, *ws)
	}
	return out, nil
}

// Workspaces returns a sorted list of workspace names.
// Convenience wrapper for callers that only need names.
func (s *Store) Workspaces() []string {
	st, err := s.Load()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(st.Workspaces))
	for name := range st.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
