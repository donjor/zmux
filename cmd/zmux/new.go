package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var newTemplateFlag string

// Deprecated: kept for one release cycle.
var newWorkspaceFlag string

var newCmd = &cobra.Command{
	Use:     "new [workspace] [session]",
	Aliases: []string{"n"},
	Short:   "Create a new session in a workspace",
	Long: `Create a new tmux session and attach to it.

  zmux new <workspace>             Create workspace + session (session = workspace name)
  zmux new <workspace> <session>   Add session to existing workspace
  zmux new                         Create tmp-N session (no workspace)
  zmux new -t <template> <ws>      Create from template in workspace

If the workspace doesn't exist, it is created automatically.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			dir = os.Getenv("HOME")
		}

		// Deprecated -w flag: rewrite to new positional args.
		if newWorkspaceFlag != "" {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			fmt.Fprintf(os.Stderr, "Warning: -w flag is deprecated. Use: zmux new %s %s\n", newWorkspaceFlag, name)
			return runNewInWorkspace(newWorkspaceFlag, name, dir)
		}

		switch len(args) {
		case 0:
			// zmux new → tmp session (no workspace)
			if newTemplateFlag != "" {
				return fmt.Errorf("template requires a workspace: zmux new -t %s <workspace>", newTemplateFlag)
			}
			return runNewTmp(dir)
		case 1:
			// zmux new <workspace>
			wsName := args[0]
			if newTemplateFlag != "" {
				return runNewFromTemplate(wsName, wsName, dir)
			}
			return runNewInWorkspace(wsName, "", dir)
		case 2:
			// zmux new <workspace> <session>
			wsName := args[0]
			sessName := args[1]
			if newTemplateFlag != "" {
				return runNewFromTemplate(wsName, sessName, dir)
			}
			return runNewInWorkspace(wsName, sessName, dir)
		}
		return nil
	},
}

// runNewTmp creates a tmp-N session with no workspace (backward compat).
func runNewTmp(dir string) error {
	name := session.NextTmpName(app.Runner)
	if err := session.Create(app.Runner, name, dir); err != nil {
		return err
	}
	return session.Attach(app.Runner, name)
}

// runNewInWorkspace creates a session within a workspace.
func runNewInWorkspace(wsName, sessName, dir string) error {
	// Validate workspace name.
	if err := session.ValidateName(wsName); err != nil {
		return fmt.Errorf("invalid workspace name: %w", err)
	}

	// Check if workspace exists.
	ws, err := app.WorkspaceStore.GetWorkspace(wsName)
	if err != nil {
		return err
	}

	if sessName == "" {
		if ws != nil && len(ws.Sessions) > 0 {
			// Workspace exists with sessions — ambiguous without session name.
			return fmt.Errorf(
				"workspace %q already exists with %d session(s)\n"+
					"  Use: zmux open %s          (enter existing)\n"+
					"  Or:  zmux new %s <session>  (add a session)",
				wsName, len(ws.Sessions), wsName, wsName,
			)
		}
		// New workspace or empty workspace — session defaults to workspace name.
		sessName = wsName
	}

	if err := session.ValidateName(sessName); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	if app.Runner.HasSession(sessName) {
		return fmt.Errorf("session %q already exists", sessName)
	}

	// Ensure workspace exists.
	if _, err := app.WorkspaceStore.EnsureWorkspace(wsName, dir); err != nil {
		return err
	}

	// Create the tmux session.
	if err := session.Create(app.Runner, sessName, dir); err != nil {
		return err
	}

	// Register in workspace.
	if err := app.WorkspaceStore.AddSession(wsName, sessName); err != nil {
		return err
	}
	_ = app.WorkspaceStore.SetLastActive(wsName, sessName)

	return session.Attach(app.Runner, sessName)
}

func runNewFromTemplate(wsName, sessName, dir string) error {
	cfg := config.DefaultConfig()
	templates, _ := session.LoadTemplates(app.FS, cfg.Templates.Paths)

	var tmpl *session.Template
	for i := range templates {
		if templates[i].Name == newTemplateFlag {
			tmpl = &templates[i]
			break
		}
	}

	if tmpl == nil {
		available := make([]string, len(templates))
		for i, t := range templates {
			available[i] = t.Name
		}
		return fmt.Errorf("template %q not found (available: %s)", newTemplateFlag, joinOr(available))
	}

	if sessName == "" {
		sessName = wsName
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

	// Register in workspace.
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

func init() {
	newCmd.Flags().StringVarP(&newTemplateFlag, "template", "t", "", "create from template")
	newCmd.Flags().StringVarP(&newWorkspaceFlag, "workspace", "w", "", "tag session to workspace (deprecated: use positional args)")
	_ = newCmd.Flags().MarkHidden("workspace")
	rootCmd.AddCommand(newCmd)
}
