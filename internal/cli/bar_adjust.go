package cli

import (
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

// newBarAdjustCmd reconciles per-session status-line options to the configured
// layout. Called from tmux hooks (session-created, session-closed,
// client-session-changed) and after bar.Apply(). For two-line/split it pins a
// stable two rows on every session — no longer toggled by session count, so the
// bar never reflows when a workspace's session count crosses 1 (plan 024).
//
// The non-two-line guard below is defensive only: config normalizes the
// removed "single" layout to two-line (plan 024), so in practice the hook
// always reconciles to two rows.
func newBarAdjustCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "bar-adjust",
		Short:  "Reconcile status line count to the configured layout",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(app.FS)
			if cfg.Bar.Layout != "two-line" && cfg.Bar.Layout != "split" {
				return nil
			}

			zmuxBin := config.SelfBin(app.Profile)

			reconcileBarStatusLines(app.Runner, cfg.Bar.Layout, cfg.Bar.TopBar, zmuxBin)
			return nil
		},
	}
}

// reconcileBarStatusLines sets every tmux session's per-session status-line
// options to match the configured layout. For two-line/split every session
// gets a stable 2 rows (top bar + normal bar) regardless of session count —
// this is the always-2-line contract, and overwriting per-session clears any
// stale single-line override left by the old count-based collapse. The else
// branch (one line) is a defensive fallback for a non-two-line layout, which
// config normalization makes unreachable in practice (plan 024). The global
// options set by bar.Apply drive new sessions; this reconciles existing ones.
func reconcileBarStatusLines(runner tmux.Runner, layout, topBar, zmuxBin string) {
	allSessions, err := runner.ListSessions()
	if err != nil {
		return
	}

	twoLine := layout == "two-line" || layout == "split"
	topBarCmd := bar.TopBarFormatCmd(zmuxBin, topBar)

	for _, sess := range allSessions {
		if twoLine {
			_ = runner.SetSessionOption(sess.Name, "status", "2")
			_ = runner.SetSessionOption(sess.Name, "status-format[0]", topBarCmd)
			_ = runner.SetSessionOption(sess.Name, "status-format[1]",
				bar.TmuxDefaultStatusFormat)
		} else {
			_ = runner.SetSessionOption(sess.Name, "status", "on")
			_ = runner.SetSessionOption(sess.Name, "status-format[0]",
				bar.TmuxDefaultStatusFormat)
		}
	}
}
