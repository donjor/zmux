package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/workspace"
)

func addLegacyWorkspaceSession(t *testing.T, store *workspace.Store, wsName, label string) {
	t.Helper()
	if err := store.AddSessionRecord(wsName, workspace.WorkspaceSession{
		ID:       "test-" + wsName + "-" + label,
		Label:    label,
		TmuxName: label,
	}); err != nil {
		t.Fatalf("add %s/%s session: %v", wsName, label, err)
	}
}
