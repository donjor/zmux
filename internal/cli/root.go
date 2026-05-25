// Package cli is the zmux command tree. It is an importable, externally
// testable package; the cmd/zmux binary is a thin launcher that calls Run.
//
// Layout:
//
//   - root.go            — Cobra root command + Run entry point, top-level
//     dispatch, shorthand resolution, tmux version check, flag wiring.
//   - popup_modes.go     — Popup-mode entry points (--dashboard, --palette,
//     --tab-picker) and their result handlers.
//   - session_picker.go  — Outside-tmux workspace+session picker flow.
//   - errors.go          — Error formatting + process exit-code mapping.
//   - Other files        — Individual cobra subcommands (init, apply, new,
//     tab, workspace, theme, bar, terminal, ...).
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
)

// Run builds the root command, executes it, and returns a process exit code.
// It is the single entry point used by package main; os.Exit stays in main so
// this stays testable.
func Run(a *apppkg.App, version string) int {
	if err := NewRootCmd(a, version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		return exitCodeForError(err)
	}
	return 0
}

// NewRootCmd builds the complete cobra command tree wired to the given App.
// No package-level state is used; every command captures app via closure.
func NewRootCmd(a *apppkg.App, version string) *cobra.Command {
	var dashboardFlag bool
	var dashboardTabFlag string
	var paletteFlag bool
	var tabPickerFlag bool
	var pickerFlag bool

	rootCmd := &cobra.Command{
		Use:           "zmux",
		Short:         "An opinionated, all-in-one tmux management wrapper",
		Long:          "zmux replaces tmux's sharp edges with a beautiful, interactive experience.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if tabPickerFlag {
				return runTabPicker(a)
			}

			if paletteFlag {
				return runPalette(a)
			}

			if dashboardFlag {
				return runDashboard(a, dashboardTabFlag)
			}

			if pickerFlag {
				return runSessionPicker(a)
			}

			if err := checkTmuxVersion(a); err != nil {
				return err
			}

			if a.Runner.ServerRunning() {
				_, _ = session.CleanupTmp(a.Runner)
			}

			if len(args) > 0 {
				return resolveShorthand(a, args)
			}

			if a.Runner.IsInsideTmux() {
				return launchDashboardPopup(a)
			}

			return runSessionPicker(a)
		},
	}

	// Root persistent flags.
	rootCmd.PersistentFlags().BoolVar(&dashboardFlag, "dashboard", false, "render dashboard TUI directly (used by popup)")
	rootCmd.PersistentFlags().StringVar(&dashboardTabFlag, "dashboard-tab", "", "initial tab for dashboard (current, sessions, settings, help)")
	rootCmd.PersistentFlags().BoolVar(&paletteFlag, "palette", false, "render command palette directly (used by popup)")
	rootCmd.PersistentFlags().BoolVar(&tabPickerFlag, "tab-picker", false, "render tab picker directly (used by Alt+`)")
	rootCmd.PersistentFlags().BoolVar(&pickerFlag, "picker", false, "render the workspace+session picker directly (used by prefix+w/s popup)")

	// Override cobra's default -h/--help to show our styled help.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printStyledHelp(a)
	})

	// Build the command tree.
	barCmd := newBarCmd(a)
	themeCmd, themePullCmd := newThemeCmd(a)

	// Bar render: needs a reference to barCmd for the debug subcommand.
	barRenderCmd := newBarRenderCmd(a, barCmd)

	paneCmd := newPaneCmd(a)
	completionCmd := newCompletionCmd(rootCmd)

	rootCmd.AddCommand(
		newApplyCmd(a),
		newBarAdjustCmd(a),
		barCmd,
		barRenderCmd,
		completionCmd,
		newHelpCmd(a),
		newInitCmd(a, version),
		newKeysCmd(),
		newKillCmd(a),
		newLsCmd(a),
		newNewCmd(a),
		newOpenCmd(a),
		paneCmd,
		newTopLevelPaneListCmd(a, "panes"),
		newTopLevelPaneListCmd(a, "list-panes"),
		newRefreshCmd(a),
		newRunCmd(a),
		newSendCmd(a),
		newSessionCmd(a),
		newSetupCmd(a),
		newSnapshotCmd(a),
		newStatusCmd(a),
		newTabCmd(a),
		newTabsCmd(a),
		newTerminalCmd(a),
		themeCmd,
		newTypeCmd(a),
		newVersionCmd(version),
		newWatchCmd(a),
		newWorkspaceCmd(a),
	)

	// Register dynamic completions.
	for _, sub := range themeCmd.Commands() {
		if sub.Use == "set <name>" {
			registerThemeCompletions(sub)
			break
		}
	}
	registerBarCompletions(barCmd)
	registerSyncCompletions(themePullCmd)

	return rootCmd
}

// resolveShorthand handles `zmux <name>` and `zmux <ws> <session>` dispatch.
//
// Two-arg form: the workspace must exist. If the session exists, attach.
// If not, create the session in the workspace and attach — equivalent to
// `zmux new <ws> <session>`.
//
// Single-arg form is workspace-first: checks if <name> is a workspace and
// attaches to its last-active session. Falls back to session attach/create
// if no matching workspace.
func resolveShorthand(app *apppkg.App, args []string) error {
	if len(args) == 2 {
		wsName := args[0]
		sessName := args[1]
		ws, _ := app.WorkspaceStore.GetWorkspace(wsName)
		if ws == nil {
			return fmt.Errorf("workspace %q not found — use zmux new %s %s to create", wsName, wsName, sessName)
		}
		if app.Runner.HasSession(sessName) {
			_ = app.WorkspaceStore.SetLastActive(wsName, sessName)
			return session.Attach(app.Runner, sessName)
		}
		// Session doesn't exist → create it in the workspace and attach.
		dir, _ := os.Getwd()
		if dir == "" {
			dir = os.Getenv("HOME")
		}
		if err := session.Create(app.Runner, sessName, dir); err != nil {
			return err
		}
		_ = app.WorkspaceStore.AddSession(wsName, sessName)
		_ = app.WorkspaceStore.SetLastActive(wsName, sessName)
		return session.Attach(app.Runner, sessName)
	}

	// Single arg: workspace-first, then session fallback.
	name := args[0]
	if ws, _ := app.WorkspaceStore.GetWorkspace(name); ws != nil {
		if target := resolveLastActive(app, ws); target != "" {
			_ = app.WorkspaceStore.SetLastActive(name, target)
			return session.Attach(app.Runner, target)
		}
		// Workspace exists but no live sessions — fall through to create.
	}

	if app.Runner.HasSession(name) {
		return session.Attach(app.Runner, name)
	}

	dir, _ := os.Getwd()
	if dir == "" {
		dir = os.Getenv("HOME")
	}
	if err := session.Create(app.Runner, name, dir); err != nil {
		return err
	}
	return session.Attach(app.Runner, name)
}

func checkTmuxVersion(app *apppkg.App) error {
	ver, err := app.Runner.Version()
	if err != nil {
		return fmt.Errorf("tmux not found — install tmux >= 3.2 to use zmux")
	}

	// Parse major.minor from version string like "3.4" or "3.2a".
	parts := strings.SplitN(ver, ".", 2)
	if len(parts) < 2 {
		return nil // can't parse, let it through
	}
	major := 0
	minor := 0
	_, _ = fmt.Sscanf(parts[0], "%d", &major)
	// Minor may have trailing letters like "2a".
	_, _ = fmt.Sscanf(parts[1], "%d", &minor)

	if major < 3 || (major == 3 && minor < 2) {
		return fmt.Errorf("tmux %s found, but zmux requires >= 3.2 (for popup support)", ver)
	}
	return nil
}
