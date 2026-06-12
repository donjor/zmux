// Package workspace manages workspaces — first-class project containers
// that group tmux sessions. State is persisted in ~/.zmux/workspaces.toml.
//
// The Store API is split across files by concern:
//
//   - store.go          — Store, NewStore, path resolution, Load/Save
//   - store_workspaces.go — Workspace CRUD (Get/Create/Ensure/Delete/Rename/List)
//   - store_sessions.go   — Session membership (Add/Remove/Move/Rename/Lookup)
//   - store_lifecycle.go  — SetLastActive, Reconcile against live tmux
//   - store_helpers.go    — small slice helpers shared across the package
package workspace

import (
	"os"
	"path/filepath"

	"github.com/donjor/zmux/internal/config"
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

// NewStore creates a Store that persists to ~/.zmux/workspaces.toml (resolved
// lazily via path()). Retained for tests and default-profile back-compat.
func NewStore(fs config.FS) *Store {
	return &Store{fs: fs}
}

// NewStoreAt creates a Store persisting to an explicit file path. The composition
// root uses this to point the store at the active profile's state dir
// (e.g. ~/.zzmux/workspaces.toml) so the edge binary doesn't share workspace state.
func NewStoreAt(fs config.FS, file string) *Store {
	return &Store{fs: fs, file: file}
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

// Load reads StateV3 from disk. Auto-migrates from v1/v2 transparently.
// Returns an empty StateV3 (not error) if the file does not exist.
func (s *Store) Load() (StateV3, error) {
	p, err := s.path()
	if err != nil {
		return emptyStateV3(), err
	}

	data, err := s.fs.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyStateV3(), nil
		}
		return emptyStateV3(), err
	}

	// Try v3 first.
	var v3 StateV3
	if err := toml.Unmarshal(data, &v3); err == nil && v3.Version >= 3 {
		if v3.Workspaces == nil {
			v3.Workspaces = make(map[string]*Workspace)
		}
		v3.populateNames()
		return v3, nil
	}

	// Fall back to v2.
	var v2 stateV2Disk
	if err := toml.Unmarshal(data, &v2); err == nil && v2.Version >= 2 {
		migrated := migrateV2toV3(v2)
		_ = s.Save(migrated) // best-effort save
		return migrated, nil
	}

	// Fall back to v1.
	var v1 State
	if err := toml.Unmarshal(data, &v1); err != nil {
		return emptyStateV3(), err
	}
	if len(v1.Sessions) == 0 {
		return emptyStateV3(), nil
	}

	// Migrate and save immediately.
	migrated := migrateV1toV3(v1)
	_ = s.Save(migrated) // best-effort save
	return migrated, nil
}

// Save writes StateV3 to disk, creating parent directories as needed.
func (s *Store) Save(st StateV3) error {
	p, err := s.path()
	if err != nil {
		return err
	}

	if err := s.fs.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	st.Version = 3
	data, err := toml.Marshal(st)
	if err != nil {
		return err
	}

	return s.fs.WriteFile(p, data, 0o644)
}
