package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/spf13/cobra"
)

// reapThrottle is the minimum gap between lazy sweeps. Lazy triggers (command
// hooks, tmux hooks) fire far more often than the reaper needs to run; the gap
// dedupes them. The flag→kill timing invariant needs sweeps spread across hours,
// so a one-minute floor changes nothing there.
const reapThrottle = time.Minute

// optLastReap is the global throttle stamp (last lazy sweep, unix seconds).
const optLastReap = "@zmux_last_reap"

// newReapCmd classifies every tab by lifecycle policy and, by default, applies
// the verdict: adopt unborn tabs, flag stale ones, kill those a prior sweep
// already flagged. --dry-run reports without changing anything. --lazy routes
// through the throttle (for command/tmux hooks); --quiet suppresses output.
func newReapCmd(app *apppkg.App) *cobra.Command {
	var dryRun, lazy, quiet bool
	var nowOverride int64
	cmd := &cobra.Command{
		Use:   "reap",
		Short: "Adopt, flag, or kill stale tabs by lifecycle policy",
		Long: "Classifies every tab by lifecycle policy (origin/scope/age/idle/live) and\n" +
			"applies the verdict: adopt unborn tabs, flag stale ones, kill those an\n" +
			"earlier sweep already flagged. Kills are re-validated pane-exactly and never\n" +
			"drop a session's last window. --dry-run reports without changing anything.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now()
			if nowOverride > 0 {
				now = time.Unix(nowOverride, 0) // --now: deterministic clock for grounding
			}
			ctx := tabs.ReapContext{Now: now, CallerPaneID: os.Getenv("TMUX_PANE")}

			if dryRun {
				rows, err := app.Runner.ListLogicalPaneRows()
				if err != nil {
					return fmt.Errorf("scan tabs: %w", err)
				}
				printReapPlan(tabs.PlanReap(rows, ctx))
				return nil
			}

			if lazy {
				MaybeReap(app, ctx.Now)
				return nil
			}

			stats, err := tabs.ApplyReap(app.Runner, ctx)
			markReaped(app, ctx.Now)
			if !quiet {
				fmt.Fprintf(os.Stderr, "reaped: %d killed · %d flagged · %d adopted\n",
					stats.Killed, stats.Flagged, stats.Adopted)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list verdicts only; change nothing")
	cmd.Flags().BoolVar(&lazy, "lazy", false, "respect the sweep throttle (for command/tmux hooks)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress the summary line")
	// --now overrides the policy clock (unix seconds) so the full age/idle/flag
	// timeline is drivable deterministically through the binary — the same Now
	// injection the library tests use, exposed for live grounding on zzmux. Dev/
	// test only; hidden from normal help.
	cmd.Flags().Int64Var(&nowOverride, "now", 0, "override the reaper clock (unix seconds) — dev/test grounding")
	_ = cmd.Flags().MarkHidden("now")
	return cmd
}

// MaybeReap runs a throttled, best-effort apply pass. It is the lazy entry point
// wired into high-traffic commands (ls/tabs/run) and tmux hooks; errors are
// swallowed so housekeeping never breaks the foreground command. The guard hook
// must never call this — reaping is a mutation.
//
// exemptPane, when non-empty, is a pane the caller is about to reuse (e.g.
// run's resolved scratch lane) — the sweep must not kill it out from under the
// in-flight command (GC-before-resolve race).
func MaybeReap(app *apppkg.App, now time.Time, exemptPane ...string) {
	if !app.Runner.ServerRunning() {
		return
	}
	if !shouldReap(app, now) {
		return
	}
	markReaped(app, now)
	exempt := ""
	if len(exemptPane) > 0 {
		exempt = exemptPane[0]
	}
	_, _ = tabs.ApplyReap(app.Runner, tabs.ReapContext{Now: now, CallerPaneID: os.Getenv("TMUX_PANE"), ExemptPaneID: exempt})
}

// shouldReap reports whether the throttle window has elapsed since the last
// sweep. The stamp is read with show-options -gqv (client-independent, unlike
// display-message format expansion which fails open in a no-client hook
// context); an unset/unparseable stamp means "never swept" → run.
func shouldReap(app *apppkg.App, now time.Time) bool {
	raw, err := app.Runner.ShowGlobalOption(optLastReap)
	if err != nil {
		return true
	}
	last, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return true
	}
	return now.Sub(time.Unix(last, 0)) >= reapThrottle
}

func markReaped(app *apppkg.App, now time.Time) {
	_ = app.Runner.SetOption("-g", optLastReap, strconv.FormatInt(now.Unix(), 10))
}

func printReapPlan(ds []tabs.ReapDecision) {
	var kill, flag, adopt, keep int
	for _, d := range ds {
		switch d.Action {
		case tabs.ReapKill:
			kill++
			fmt.Printf("KILL   %-18s %s\n", reapLabel(d), d.Reason)
		case tabs.ReapFlag:
			flag++
			fmt.Printf("FLAG   %-18s %s\n", reapLabel(d), d.Reason)
		case tabs.ReapAdopt:
			adopt++
			fmt.Printf("ADOPT  %-18s %s\n", reapLabel(d), d.Reason)
		default:
			keep++
			if d.Scope == tabs.ScopePeer || d.Scope == tabs.ScopeTask {
				fmt.Printf("KEEP   %-18s %s\n", reapLabel(d), d.Reason)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "\n%d kill · %d flag · %d adopt · %d keep (dry-run: nothing changed)\n",
		kill, flag, adopt, keep)
}

func reapLabel(d tabs.ReapDecision) string {
	if d.Label != "" {
		return d.Label
	}
	return d.PaneID
}
