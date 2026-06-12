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
		if !app.Runner.IsInsideTmux() {
			return "", fmt.Errorf("not inside tmux — use --session workspace/session to specify target")
		}
		name, err := app.Runner.DisplayMessage("", "#{session_name}")
		if err != nil {
			return "", fmt.Errorf("not inside a tmux session")
		}
		return strings.TrimSpace(name), nil
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
		return rec.TmuxName, nil
	}

	if app.Runner.IsInsideTmux() {
		current, _ := app.Runner.DisplayMessage("", "#{session_name}")
		if wsName, ok := app.WorkspaceStore.WorkspaceFor(session.RootName(strings.TrimSpace(current))); ok {
			if rec, ok := app.WorkspaceStore.SessionRecord(wsName, input); ok {
				return rec.TmuxName, nil
			}
		}
	}

	if wsName, ok := workspaceForCWD(app); ok {
		if rec, ok := app.WorkspaceStore.SessionRecord(wsName, input); ok {
			return rec.TmuxName, nil
		}
	}

	matches := matchingSessionLabels(app, input)
	if len(matches) == 1 {
		return matches[0].TmuxName, nil
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

func matchingSessionLabels(app *apppkg.App, label string) []workspace.WorkspaceSession {
	workspaces, err := app.WorkspaceStore.ListWorkspaces()
	if err != nil {
		return nil
	}
	var matches []workspace.WorkspaceSession
	for _, ws := range workspaces {
		for _, rec := range ws.Sessions {
			if rec.Label == label {
				matches = append(matches, rec)
			}
		}
	}
	return matches
}
