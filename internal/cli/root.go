// Package cli is the zmux command tree. It is an importable, externally
// testable package; the cmd/zmux binary is a thin launcher that calls Run.
//
// Layout:
//
//   - root.go            — Cobra root command + Run entry point, top-level
//     dispatch, removed shorthand guard, tmux version check, flag wiring.
//   - popup_modes.go     — Popup-mode entry points (--dashboard, --palette,
//     --tab-picker) and their result handlers.
//   - session_picker.go  — Outside-tmux workspace+session picker flow.
//   - errors.go          — Error formatting + process exit-code mapping.
//   - Other files        — Individual cobra subcommands (init, apply, new,
//     tab, workspace, theme, bar, terminal, ...).
package cli

import (
	"errors"
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
		if errors.Is(err, errInvalidCommand) {
			fmt.Fprintln(os.Stderr, errInvalidCommand)
			printStyledHelp(a)
			return exitCodeForError(err)
		}
		if m := formatError(err); m != "" {
			fmt.Fprintln(os.Stderr, m)
		}
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
	var workspacePickerFlag bool
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

			if workspacePickerFlag {
				return runWorkspacePicker(a)
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
				return errInvalidCommand
			}

			switch defaultRootEntry(a) {
			case entryDashboardPopup:
				return launchDashboardPopup(a)
			default:
				return runSessionPicker(a)
			}
		},
	}

	// Root persistent flags.
	rootCmd.PersistentFlags().BoolVar(&dashboardFlag, "dashboard", false, "render dashboard TUI directly (used by popup)")
	rootCmd.PersistentFlags().StringVar(&dashboardTabFlag, "dashboard-tab", "", "initial tab for dashboard (current, sessions, settings, help)")
	rootCmd.PersistentFlags().BoolVar(&paletteFlag, "palette", false, "render command palette directly (used by popup)")
	rootCmd.PersistentFlags().BoolVar(&tabPickerFlag, "tab-picker", false, "render tab picker directly (used by Alt+`)")
	rootCmd.PersistentFlags().BoolVar(&workspacePickerFlag, "workspace-picker", false, "render workspace switcher directly (used by Alt+w)")
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
		newBarSpinnerCmd(a),
		completionCmd,
		newGuardCmd(a),
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
		newRecipeCmd(a),
		newRunCmd(a),
		newScratchCmd(a),
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
		newUpCmd(a),
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

// rootEntryKind is the surface a bare `zmux` (no args, no popup flags) lands on.
type rootEntryKind int

const (
	entryPicker rootEntryKind = iota
	entryDashboardPopup
)

// defaultRootEntry decides the surface for a bare `zmux` invocation. Inside tmux
// → the dashboard popup; outside tmux → the workspace+session picker, regardless
// of live-session count. The picker owns the cold-start/empty state (create a
// workspace or tmp session and attach); the sessionless dashboard is reserved
// for the attach-fallback path in session_fallback.go (close-last-session /
// vanished target), never the explicit invocation. Pure decision over the runner
// so routing is unit-testable without driving Bubble Tea.
func defaultRootEntry(a *apppkg.App) rootEntryKind {
	if a.Runner.IsInsideTmux() {
		return entryDashboardPopup
	}
	return entryPicker
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
