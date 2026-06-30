package workspaceview

import (
	"testing"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
)

func TestBuildWorkspaceViewModelsClaimsPinnedCloneByRoot(t *testing.T) {
	rec, err := workspace.NewSessionRecord("dev", "main")
	if err != nil {
		t.Fatal(err)
	}
	models := BuildWorkspaceViewModels(
		[]workspace.Workspace{{Name: "dev", Sessions: []workspace.WorkspaceSession{rec}}},
		[]session.SessionInfo{
			{Name: rec.TmuxName, Label: rec.Label, SessionID: rec.ID},
			{Name: rec.TmuxName + "__clone_b", PinnedView: true, ViewRoot: rec.TmuxName},
		},
	)

	if len(models) != 1 {
		t.Fatalf("models = %d; want 1", len(models))
	}
	if got := len(models[0].LiveSessions); got != 2 {
		t.Fatalf("live sessions = %d; want root plus pinned view", got)
	}
	pinned := models[0].LiveSessions[1]
	if pinned.Workspace != "dev" || pinned.Label != "main" || pinned.SessionID != rec.ID {
		t.Fatalf("pinned clone did not inherit workspace metadata: %#v", pinned)
	}
}
