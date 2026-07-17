package cli

import (
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

// newScratchCmd builds the `zmux scratch` command. The bare positional form
// `zmux scratch '<cmd>'` runs a bounded command in the single shared `scratch`
// lane (the same default an unnamed `zmux run` now targets), reusing the tab on
// every rerun instead of minting one per command. The `extract` subcommand is
// the popup helper that promotes the scratch shell's cwd into a real tab.
//
// Cobra dispatch: a first arg that names a known subcommand (`extract`) routes
// there; anything else falls through to this RunE as the command to run — so
// the blessed bare verb and the legacy subcommand coexist.
func newScratchCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var timeout int
	var follow bool

	cmd := &cobra.Command{
		Use:   "scratch [command]",
		Short: "Run a bounded command in the shared scratch tab (or: scratch extract)",
		Long: `Run a bounded command in the single reusable ` + "`scratch`" + ` tab — the default
lane for one-off commands (typecheck, test, lint, build). Every rerun reuses the
same tab instead of minting a fresh one per command, so ephemeral runs never
sprawl the roster. This is the explicit form of what an unnamed ` + "`zmux run`" + `
now does by default.

  zmux scratch 'bun run lint'        # run in the reused scratch tab
  zmux scratch 'go test ./...'       # rerun reuses the same tab

Durable/no-exit runtimes (dev servers, watchers) do NOT belong here — give them
their own kept tab with ` + "`zmux run '<cmd>' -n dev -d`" + `.

Run ` + "`zmux scratch extract`" + ` from inside the scratch shell popup to promote its
cwd into a new tab in the parent session.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			command := strings.TrimSpace(strings.Join(args, " "))
			if command == "" {
				return fmt.Errorf("usage: zmux scratch '<command>' (or: zmux scratch extract)")
			}
			sessionName, err := resolveSessionTarget(app, sessionFlag)
			if err != nil {
				return err
			}
			if !app.Runner.HasSession(sessionName) {
				return fmt.Errorf("session %q does not exist", sessionName)
			}
			// Always the shared scratch lane: claim it as a stable label so it
			// dedups/reuses, bounded lifecycle (no detach/keep/until, task scope).
			return runCommandInTab(app, sessionName, scratchLane, command, runTabOpts{
				claimLabel:  scratchLane,
				follow:      follow,
				timeout:     timeout,
				followLines: 50,
			})
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session (default: current session)")
	cmd.Flags().IntVarP(&timeout, "timeout", "T", 120, "timeout in seconds (default 120)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "tail output live (Ctrl+C to stop)")
	cmd.AddCommand(newScratchExtractCmd(app))
	return cmd
}

func newScratchExtractCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "extract",
		Short: "Promote scratch shell cwd to a new tab in the parent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Runner.IsInsideTmux() {
				return fmt.Errorf("zmux scratch extract must run inside the scratch popup")
			}

			session, err := app.Runner.DisplayMessage("", "#S")
			if err != nil || session == "" {
				return fmt.Errorf("could not resolve parent session: %w", err)
			}

			dir, err := os.Getwd()
			if err != nil || dir == "" {
				dir = os.Getenv("HOME")
			}

			if _, err := app.Runner.NewWindow(session, "", dir); err != nil {
				return fmt.Errorf("create tab in %s: %w", session, err)
			}

			// Close the popup we are currently inside. -C is best-effort —
			// older tmux returns an error but the new tab is already there,
			// so swallow it and let the user close the popup manually.
			_ = app.Runner.DisplayPopup("-C")
			return nil
		},
	}
}
