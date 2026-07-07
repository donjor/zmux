package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
)

func resolveSessionTarget(app *apppkg.App, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return currentSessionName(app)
	}

	if strings.Contains(input, "/") {
		ws, label, err := parseWorkspaceSessionTarget(input)
		if err != nil {
			return "", err
		}
		rec, ok := app.WorkspaceStore.SessionRecord(ws, label)
		if !ok {
			return "", fmt.Errorf("session %q is not in workspace %q", label, ws)
		}
		return liveWorkspaceSessionTarget(app, ws, rec), nil
	}

	if app.Runner.IsInsideTmux() {
		current, _ := app.Runner.DisplayMessage("", "#{session_name}")
		if wsName, ok := app.WorkspaceStore.WorkspaceFor(session.RootName(strings.TrimSpace(current))); ok {
			if rec, ok := app.WorkspaceStore.SessionRecord(wsName, input); ok {
				return liveWorkspaceSessionTarget(app, wsName, rec), nil
			}
		}
	}

	if wsName, ok := workspaceForCWD(app); ok {
		if rec, ok := app.WorkspaceStore.SessionRecord(wsName, input); ok {
			return liveWorkspaceSessionTarget(app, wsName, rec), nil
		}
	}

	matches := matchingSessionLabels(app, input)
	if len(matches) == 1 {
		return liveWorkspaceSessionTarget(app, matches[0].Workspace, matches[0].Record), nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("session label %q is ambiguous; use workspace/session", input)
	}

	// Raw tmux name escape hatch for debug/interop. Keep this after
	// workspace-local label resolution so raw unmanaged names cannot shadow
	// normal workspace/session identity.
	if app.Runner.HasSession(input) {
		return input, nil
	}
	return "", fmt.Errorf("session %q not found; use workspace/session for workspace-local labels", input)
}

// currentSessionName resolves the session the caller is inside, root-
// normalized: a grouped clone (dev-b) collapses to its root, matching the
// user-facing label model and the root-keyed workspace store. Window/pane
// operations are unaffected — clones share windows with their root.
func currentSessionName(app *apppkg.App) (string, error) {
	if !app.Runner.IsInsideTmux() {
		return "", fmt.Errorf("not inside tmux — use --session workspace/session to specify target")
	}
	name, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return "", fmt.Errorf("not inside a tmux session")
	}
	return session.RootName(strings.TrimSpace(name)), nil
}

func parseWorkspaceSessionTarget(target string) (string, string, error) {
	if strings.Count(target, "/") != 1 {
		return "", "", fmt.Errorf("target must be workspace/session")
	}
	parts := strings.SplitN(target, "/", 2)
	ws, label := parts[0], parts[1]
	if err := workspace.ValidateWorkspaceName(ws); err != nil {
		return "", "", err
	}
	if err := workspace.ValidateSessionLabel(label); err != nil {
		return "", "", err
	}
	return ws, label, nil
}

func workspaceForCWD(app *apppkg.App) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	cwd = filepath.Clean(cwd)
	workspaces, err := app.WorkspaceStore.ListWorkspaces()
	if err != nil {
		return "", false
	}
	var match string
	matchLen := -1
	for _, ws := range workspaces {
		if ws.RootDir == "" {
			continue
		}
		root := filepath.Clean(ws.RootDir)
		if cwd == root || strings.HasPrefix(cwd, root+string(os.PathSeparator)) {
			if len(root) > matchLen {
				match = ws.Name
				matchLen = len(root)
			}
		}
	}
	return match, match != ""
}

type sessionLabelMatch struct {
	Workspace string
	Record    workspace.WorkspaceSession
}

func matchingSessionLabels(app *apppkg.App, label string) []sessionLabelMatch {
	workspaces, err := app.WorkspaceStore.ListWorkspaces()
	if err != nil {
		return nil
	}
	var matches []sessionLabelMatch
	for _, ws := range workspaces {
		for _, rec := range ws.Sessions {
			if rec.Label == label {
				matches = append(matches, sessionLabelMatch{Workspace: ws.Name, Record: rec})
			}
		}
	}
	return matches
}
