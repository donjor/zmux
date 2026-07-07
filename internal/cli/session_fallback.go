package cli

import (
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

		forceDetachOnDestroyForFallback(app.Runner, target)

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
	return runDashboard(app, string(dashboard.TabWorkspaces))
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

// forceDetachOnDestroyForFallback makes the client detach (returning control to
// zmux) when its session is destroyed, so the owned-attach loop can pick the next
// target or the dashboard. Set per-session, not -g, so a user's global
// `detach-on-destroy off` keeps its meaning everywhere except zmux-owned attaches.
func forceDetachOnDestroyForFallback(runner tmux.Runner, target string) {
	_ = runner.SetSessionOption(target, "detach-on-destroy", "on")
}
