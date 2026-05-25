package cli

import (
	"os"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

// newBarAdjustCmd dynamically toggles tmux status line count for two-line
// layouts. Called from tmux hooks (session-created, session-closed,
// client-session-changed) and after bar.Apply(). Sets per-session
// options so each terminal shows the right number of status lines
// independent of other sessions.
func newBarAdjustCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "bar-adjust",
		Short:  "Adjust status line count based on workspace sessions",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(app.FS)
			if cfg.Bar.Layout != "two-line" && cfg.Bar.Layout != "split" {
				return nil
			}

			zmuxBin, err := os.Executable()
			if err != nil {
				return nil
			}

			adjustBarStatusLines(app.Runner, app.WorkspaceStore, cfg.Bar.TopBar, zmuxBin)
			return nil
		},
	}
}

// adjustBarStatusLines iterates all tmux sessions and sets per-session
// status line count. Workspaces with 2+ sessions get 2 lines (top bar +
// normal bar); others get 1 line.
func adjustBarStatusLines(runner tmux.Runner, ws *workspace.Store, topBar, zmuxBin string) {
	allSessions, err := runner.ListSessions()
	if err != nil {
		return
	}

	topBarCmd := bar.TopBarFormatCmd(zmuxBin, topBar)

	for _, sess := range allSessions {
		wsName, _ := ws.WorkspaceFor(sess.Name)
		var count int
		if wsName != "" {
			count = len(ws.SessionsIn(wsName))
		}

		if count > 1 {
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
