package picker

import "github.com/donjor/zmux/internal/workspace"

func testWorkspaceSessions(labels ...string) []workspace.WorkspaceSession {
	out := make([]workspace.WorkspaceSession, 0, len(labels))
	for _, label := range labels {
		out = append(out, workspace.WorkspaceSession{
			ID:       "test-" + label,
			Label:    label,
			TmuxName: label,
		})
	}
	return out
}
