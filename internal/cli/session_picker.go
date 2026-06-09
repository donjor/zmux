package cli

// runSessionPicker — the workspace+session picker launched when zmux runs
// outside a tmux client with no positional args. Owns the post-pick
// dispatch (attach / new / workspace-create / external-attach / workspace-focus)
// so the picker model itself stays UI-only.

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/picker"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

func runSessionPicker(app *apppkg.App) error {
	// Resolve working directory.
	dir, err := os.Getwd()
	if err != nil {
		dir = os.Getenv("HOME")
		if dir == "" {
			dir = "/"
		}
	}

	styles, _, _ := loadActiveStyles(app)
	model := picker.NewPickerModel(app.Runner, styles)
	model.SetWorkspaceStore(app.WorkspaceStore)
	model.SetWorkspaceDataLoader(func() []workspaceview.WorkspaceViewModel {
		return loadWorkspaceView(app, workspaceViewOptions{Reconcile: true})
	})

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("workspace picker: %w", err)
	}

	pk, ok := result.(picker.PickerModel)
	if !ok {
		return nil
	}

	res := pk.Result

	switch res.Action {
	case "attach":
		return session.Attach(app.Runner, res.Session)

	case "hijack":
		return session.AttachHijack(app.Runner, res.Session)

	case "new":
		name := res.Name
		if name == "" {
			name = session.NextTmpName(app.Runner)
		}
		// If the session already exists (e.g. "main" across workspaces),
		// pick the next available name based on the workspace.
		if app.Runner.HasSession(name) {
			if res.Workspace != "" {
				name = nextSessionName(app, name, res.Workspace)
			} else {
				name = session.NextTmpName(app.Runner)
			}
		}
		if err := session.Create(app.Runner, name, dir); err != nil {
			return err
		}
		// Auto-assign to workspace if specified.
		if res.Workspace != "" {
			_ = app.WorkspaceStore.AddSession(res.Workspace, name)
			_ = app.WorkspaceStore.SetLastActive(res.Workspace, name)
		}
		return session.Attach(app.Runner, name)

	case "workspace-create":
		wsName := res.Workspace
		if err := app.WorkspaceStore.CreateWorkspace(wsName, dir); err != nil {
			return err
		}
		// Use the session name from the picker result if provided
		// (e.g. "myapp dev" → Name="dev"), otherwise default to "main".
		// For the default "main", use the workspace name as the tmux
		// session name so multiple workspaces can each have a "main"
		// without global tmux collisions.
		sessName := res.Name
		if sessName == "" {
			sessName = wsName
		} else if app.Runner.HasSession(sessName) {
			sessName = nextSessionName(app, sessName, wsName)
		}
		if err := session.Create(app.Runner, sessName, dir); err != nil {
			return err
		}
		_ = app.WorkspaceStore.AddSession(wsName, sessName)
		_ = app.WorkspaceStore.SetLastActive(wsName, sessName)
		return session.Attach(app.Runner, sessName)

	case "overmind-connect":
		src := res.ExternalSource
		if src != nil && src.Overmind != nil {
			return app.Overmind.Connect(src.Overmind.ControlSocket, res.Session)
		}
		if src != nil {
			return source.ConnectFallback(src.Endpoint, res.Session, "")
		}
		return fmt.Errorf("no source for overmind-connect")

	case "external-attach":
		src := res.ExternalSource
		if src != nil {
			return source.ConnectFallback(src.Endpoint, res.Session, "")
		}
		return fmt.Errorf("no source for external-attach")

	case "workspace-focus":
		ws, err := app.WorkspaceStore.GetWorkspace(res.Workspace)
		if err != nil || ws == nil {
			return fmt.Errorf("workspace %q not found", res.Workspace)
		}
		target := resolveLastActive(app, ws)
		if target == "" {
			return fmt.Errorf("workspace %q has no live sessions", res.Workspace)
		}
		_ = app.WorkspaceStore.SetLastActive(res.Workspace, target)
		return session.Attach(app.Runner, target)
	}

	return nil
}
