// Package workspace manages workspaces — first-class project containers
// that group tmux sessions. State is persisted in ~/.zmux/workspaces.toml.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	toml "github.com/pelletier/go-toml/v2"
)

// State is the legacy v1 on-disk format (flat session->workspace map).
// Kept for migration only.
type State struct {
	Sessions map[string]string `toml:"sessions"`
}

// Store reads and writes workspace state backed by a TOML file.
type Store struct {
	fs   config.FS
	file string // resolved path, cached after first call
}

// NewStore creates a Store that persists to ~/.zmux/workspaces.toml.
func NewStore(fs config.FS) *Store {
	return &Store{fs: fs}
}

// path returns the resolved file path, caching after the first call.
func (s *Store) path() (string, error) {
	if s.file != "" {
		return s.file, nil
	}
	home, err := s.fs.UserHomeDir()
	if err != nil {
		return "", err
	}
	s.file = filepath.Join(home, ".zmux", "workspaces.toml")
	return s.file, nil
}

// ── Load / Save ──

// Load reads StateV2 from disk. Auto-migrates from v1 transparently.
// Returns an empty StateV2 (not error) if the file does not exist.
func (s *Store) Load() (StateV2, error) {
	p, err := s.path()
	if err != nil {
		return emptyStateV2(), err
	}

	data, err := s.fs.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyStateV2(), nil
		}
		return emptyStateV2(), err
	}

	// Try v2 first.
	var v2 StateV2
	if err := toml.Unmarshal(data, &v2); err == nil && v2.Version >= 2 {
		if v2.Workspaces == nil {
			v2.Workspaces = make(map[string]*Workspace)
		}
		v2.populateNames()
		return v2, nil
	}

	// Fall back to v1.
	var v1 State
	if err := toml.Unmarshal(data, &v1); err != nil {
		return emptyStateV2(), err
	}
	if v1.Sessions == nil || len(v1.Sessions) == 0 {
		return emptyStateV2(), nil
	}

	// Migrate and save immediately.
	migrated := migrateV1toV2(v1)
	_ = s.Save(migrated) // best-effort save
	return migrated, nil
}

// Save writes StateV2 to disk, creating parent directories as needed.
func (s *Store) Save(st StateV2) error {
	p, err := s.path()
	if err != nil {
		return err
	}

	if err := s.fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	st.Version = 2
	data, err := toml.Marshal(st)
	if err != nil {
		return err
	}

	return s.fs.WriteFile(p, data, 0o644)
}

// ── Workspace CRUD ──

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
// Returns error if it already exists.
func (s *Store) CreateWorkspace(name, rootDir string) error {
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
		Sessions:  []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.Save(st)
}

// EnsureWorkspace creates a workspace if it doesn't exist, or returns the existing one.
func (s *Store) EnsureWorkspace(name, rootDir string) (*Workspace, error) {
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
		Sessions:  []string{},
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

// ── Session membership ──

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

// ── Lifecycle ──

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

// Cleanup is a backward-compatible alias for Reconcile.
// Deprecated: use Reconcile instead.
func (s *Store) Cleanup(liveRoots map[string]bool) error {
	return s.Reconcile(liveRoots)
}

// ── Backward compatibility (v1 API shims) ──
// These methods exist so callers that haven't been updated yet continue to compile.
// They operate on v2 state internally.

// Set tags a root session to a workspace (v1 compat shim).
func (s *Store) Set(rootSession, workspace string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}

	// Remove from any existing workspace.
	if ws, found := st.WorkspaceForSession(rootSession); found {
		ws.Sessions = removeString(ws.Sessions, rootSession)
		ws.UpdatedAt = time.Now()
	}

	// Add to target workspace (create if needed).
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

// Delete removes a session from workspace tracking (v1 compat shim).
func (s *Store) Delete(rootSession string) error {
	return s.RemoveSession(rootSession)
}

// Rename moves a workspace entry from one root session name to another (v1 compat shim).
func (s *Store) Rename(oldRoot, newRoot string) error {
	return s.RenameSession(oldRoot, newRoot)
}

// ── Helpers ──

func removeString(slice []string, val string) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			out = append(out, s)
		}
	}
	return out
}

func sortStrings(s []string) {
	sort.Strings(s)
}
