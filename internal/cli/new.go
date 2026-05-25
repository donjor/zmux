package cli

import (
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newNewCmd(app *apppkg.App) *cobra.Command {
	var newTemplateFlag string
	// Deprecated: kept for one release cycle.
	var newWorkspaceFlag string

	cmd := &cobra.Command{
		Use:     "new <workspace> [session...]",
		Aliases: []string{"n"},
		Short:   "Create a workspace and sessions",
		Long: `Create a new workspace with sessions and attach.

  zmux new myapp                   Create workspace 'myapp' + session 'main', attach
  zmux new myapp auth              Create workspace (if needed) + session 'auth', attach
  zmux new myapp auth server dev   Create workspace + multiple sessions, attach first
  zmux new myapp -t dev-setup      Create workspace + template-defined sessions

If the workspace already exists:
  zmux new myapp         → error (use zmux myapp to attach)
  zmux new myapp <sess>  → adds session to existing workspace`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				dir = os.Getenv("HOME")
			}

			// Deprecated -w flag.
			if newWorkspaceFlag != "" {
				name := ""
				if len(args) > 0 {
					name = args[0]
				}
				fmt.Fprintf(os.Stderr, "Warning: -w flag is deprecated. Use: zmux new %s %s\n", newWorkspaceFlag, name)
				return runNewInWorkspace(app, newWorkspaceFlag, []string{name}, dir)
			}

			if len(args) == 0 {
				// Backward compat: zmux new → tmp-N session (no workspace).
				if newTemplateFlag != "" {
					return fmt.Errorf("template requires a workspace: zmux new -t %s <workspace>", newTemplateFlag)
				}
				return runNewTmp(app, dir)
			}

			wsName := args[0]
			sessionNames := args[1:]

			// Validate workspace name.
			if err := workspace.ValidateWorkspaceName(wsName); err != nil {
				return err
			}

			// Template mode.
			if newTemplateFlag != "" {
				return runNewFromTemplate(app, wsName, sessionNames, dir, newTemplateFlag)
			}

			return runNewInWorkspace(app, wsName, sessionNames, dir)
		},
	}

	cmd.Flags().StringVarP(&newTemplateFlag, "template", "t", "", "create from template")
	cmd.Flags().StringVarP(&newWorkspaceFlag, "workspace", "w", "", "tag session to workspace (deprecated: use positional args)")
	_ = cmd.Flags().MarkHidden("workspace")
	return cmd
}

// runNewTmp creates a tmp-N session with no workspace (backward compat).
func runNewTmp(app *apppkg.App, dir string) error {
	name := session.NextTmpName(app.Runner)
	if err := session.Create(app.Runner, name, dir); err != nil {
		return err
	}
	return session.Attach(app.Runner, name)
}

// runNewInWorkspace creates a workspace (if needed) and sessions within it.
func runNewInWorkspace(app *apppkg.App, wsName string, sessionNames []string, dir string) error {
	// Check if workspace exists.
	ws, err := app.WorkspaceStore.GetWorkspace(wsName)
	if err != nil {
		return err
	}

	if len(sessionNames) == 0 || (len(sessionNames) == 1 && sessionNames[0] == "") {
		// zmux new <workspace> — no session names.
		if ws != nil && len(ws.Sessions) > 0 {
			return fmt.Errorf(
				"workspace %q already exists — use zmux %s to attach or zmux new %s <session> to add a session",
				wsName, wsName, wsName,
			)
		}
		// New workspace — default session name matches workspace name
		// to avoid global tmux collisions (every workspace can have
		// its own primary session without naming conflicts).
		sessionNames = []string{wsName}
	}

	// Ensure workspace exists.
	if _, err := app.WorkspaceStore.EnsureWorkspace(wsName, dir); err != nil {
		return err
	}

	// Create each session.
	var firstSession string
	for _, sessName := range sessionNames {
		if sessName == "" {
			continue
		}
		if err := session.ValidateName(sessName); err != nil {
			return fmt.Errorf("invalid session name %q: %w", sessName, err)
		}
		if app.Runner.HasSession(sessName) {
			return fmt.Errorf("session %q already exists", sessName)
		}

		if err := session.Create(app.Runner, sessName, dir); err != nil {
			return err
		}
		if err := app.WorkspaceStore.AddSession(wsName, sessName); err != nil {
			return err
		}
		if firstSession == "" {
			firstSession = sessName
		}
	}

	if firstSession == "" {
		return fmt.Errorf("no sessions created")
	}

	_ = app.WorkspaceStore.SetLastActive(wsName, firstSession)
	return session.Attach(app.Runner, firstSession)
}

func runNewFromTemplate(app *apppkg.App, wsName string, sessionNames []string, dir string, templateFlag string) error {
	cfg := config.DefaultConfig()
	templates, _ := session.LoadTemplates(app.FS, cfg.Templates.Paths)

	var tmpl *session.Template
	for i := range templates {
		if templates[i].Name == templateFlag {
			tmpl = &templates[i]
			break
		}
	}

	if tmpl == nil {
		available := make([]string, len(templates))
		for i, t := range templates {
			available[i] = t.Name
		}
		return fmt.Errorf("template %q not found (available: %s)", templateFlag, joinOr(available))
	}

	// Determine session name.
	sessName := wsName
	if len(sessionNames) > 0 && sessionNames[0] != "" {
		sessName = sessionNames[0]
	}

	if app.Runner.HasSession(sessName) {
		return fmt.Errorf("session %q already exists", sessName)
	}

	// Ensure workspace.
	if _, err := app.WorkspaceStore.EnsureWorkspace(wsName, dir); err != nil {
		return err
	}

	if err := session.CreateFromTemplate(app.Runner, *tmpl, sessName, dir); err != nil {
		return err
	}

	if err := app.WorkspaceStore.AddSession(wsName, sessName); err != nil {
		return err
	}
	_ = app.WorkspaceStore.SetLastActive(wsName, sessName)

	return session.Attach(app.Runner, sessName)
}

func joinOr(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}
