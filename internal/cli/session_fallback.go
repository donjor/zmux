package cli

import (
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

const maxOwnedAttachAttempts = 8

type (
	ownedAttachFunc         func(tmux.Runner, string) error
	sessionlessFallbackFunc func(*apppkg.App) error
)

func attachOwnedSession(app *apppkg.App, name string) error {
	return attachOwnedSessionWith(app, name, session.Attach)
}

func attachOwnedSessionWith(app *apppkg.App, name string, attach ownedAttachFunc) error {
	return attachOwnedSessionLoop(app, name, attach, runSessionlessDashboard)
}

func attachOwnedSessionLoop(app *apppkg.App, name string, attach ownedAttachFunc, fallback sessionlessFallbackFunc) error {
	if attach == nil {
		attach = session.Attach
	}
	if fallback == nil {
		fallback = runSessionlessDashboard
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback(app)
	}

	if app.Runner.IsInsideTmux() {
		return attach(app.Runner, name)
	}

	workspaceName, _ := attachFallbackWorkspace(app, name)
	target := name
	seen := make(map[string]bool, maxOwnedAttachAttempts)

	for attempt := 0; attempt < maxOwnedAttachAttempts; attempt++ {
		root := session.RootName(strings.TrimSpace(target))
		if root == "" || seen[root] {
			return fallback(app)
		}
		seen[root] = true

		if err := ensureDetachOnDestroyAllowsFallback(app.Runner, target); err != nil {
			return err
		}

		err := attach(app.Runner, target)
		if app.Runner.HasSession(root) {
			return err
		}

		next, ok := nextAttachFallbackTarget(app, workspaceName, seen)
		if !ok {
			return fallback(app)
		}
		target = next
		attach = session.Attach
	}

	return fallback(app)
}

func runSessionlessDashboard(app *apppkg.App) error {
	return runNewDashboard(app, string(dashboard.TabWorkspaces))
}

func hasLiveSessions(app *apppkg.App) bool {
	sessions, err := session.ListSessions(app.Runner)
	return err == nil && len(sessions) > 0
}

func attachFallbackWorkspace(app *apppkg.App, name string) (string, bool) {
	if app.WorkspaceStore == nil {
		return "", false
	}
	return app.WorkspaceStore.WorkspaceFor(session.RootName(name))
}

func nextAttachFallbackTarget(app *apppkg.App, workspaceName string, seen map[string]bool) (string, bool) {
	if workspaceName == "" || app.WorkspaceStore == nil {
		return "", false
	}
	ws, err := app.WorkspaceStore.GetWorkspace(workspaceName)
	if err != nil || ws == nil {
		return "", false
	}
	rec := resolveLastActive(app, ws)
	if rec.TmuxName == "" {
		return "", false
	}
	target := liveWorkspaceSessionTarget(app, workspaceName, rec)
	root := session.RootName(target)
	if root == "" || seen[root] || !app.Runner.HasSession(target) {
		return "", false
	}
	_ = app.WorkspaceStore.SetLastActive(workspaceName, rec.ID)
	return target, true
}

func ensureDetachOnDestroyAllowsFallback(runner tmux.Runner, target string) error {
	value, err := runner.DisplayMessage(target, "#{detach-on-destroy}")
	if err != nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "off", "0", "false":
		return fmt.Errorf("session fallback requires tmux detach-on-destroy=on; remove `set -g detach-on-destroy off` from tmux config")
	default:
		return nil
	}
}
