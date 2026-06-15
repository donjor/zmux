package workspace

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
)

// CreateManagedSession creates a tmux session with canonical zmux identity
// (zws_<workspace>__<label>), stamps its identity metadata, and records it in
// the store. It is the single create path shared by the CLI (`zmux new`) and the
// dashboard so both produce identical, addressable sessions instead of the
// dashboard's old raw-label sessions that the picker and bar could not resolve.
//
// The caller ensures the workspace already exists; this adds a session to it. On
// any failure after the tmux session is created, the session is killed so no
// orphan is left behind.
func CreateManagedSession(runner tmux.Runner, store *Store, wsName, label, dir string) (WorkspaceSession, error) {
	if store == nil {
		return WorkspaceSession{}, fmt.Errorf("workspace store unavailable")
	}
	if err := ValidateSessionLabel(label); err != nil {
		return WorkspaceSession{}, fmt.Errorf("invalid session label %q: %w", label, err)
	}
	rec, err := NewSessionRecord(wsName, label)
	if err != nil {
		return WorkspaceSession{}, err
	}
	if runner.HasSession(rec.TmuxName) {
		return WorkspaceSession{}, fmt.Errorf("session %q already exists in workspace %q", label, wsName)
	}
	if err := session.Create(runner, rec.TmuxName, dir); err != nil {
		return WorkspaceSession{}, err
	}
	if err := StampSessionMetadata(runner, wsName, rec); err != nil {
		_ = runner.KillSession(rec.TmuxName)
		return WorkspaceSession{}, err
	}
	if err := store.AddSessionRecord(wsName, rec); err != nil {
		_ = runner.KillSession(rec.TmuxName)
		return WorkspaceSession{}, err
	}
	return rec, nil
}
