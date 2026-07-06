package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

// whereContext is the resolved "where am I" answer: the current pane's place in
// the workspace → session → tab hierarchy. Session is the workspace-local label;
// SessionTmux is the raw zws_… name that other verbs accept as -s.
type whereContext struct {
	Workspace   string `json:"workspace"`
	Session     string `json:"session"`
	SessionTmux string `json:"session_tmux"`
	Tab         string `json:"tab"`
	PaneID      string `json:"pane"`
	WindowIndex int    `json:"window_index"`
	Dir         string `json:"dir"`
}

// newWhereCmd is `zmux where` (alias `whoami`): a first-class report of the
// current context — workspace, session, tab, pane, cwd — as one answer. Unlike
// `pane current` (the pane-scoped primitive) it also resolves the workspace and
// logical tab, so "where am I" is a single command rather than parsing the
// zws_<ws>__<label> session name by hand.
func newWhereCmd(app *apppkg.App) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "where",
		Aliases: []string{"whoami"},
		Short:   "Report the current context: workspace, session, tab, pane, cwd",
		Long: `Report the current context as one answer: workspace, session (local
label + raw tmux name), tab, pane id, and cwd.

Composes the same identity other verbs resolve — use it to know what to pass as
-s, or just to orient. For pane-only facts use ` + "`zmux pane current`" + `;
for config (theme/bar/prefix) use ` + "`zmux status`" + `.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paneID := os.Getenv("TMUX_PANE")
			if paneID == "" || !app.Runner.IsInsideTmux() {
				return fmt.Errorf("zmux where requires tmux in the current profile (no current pane)")
			}
			pane := currentPaneByID(app, paneID)
			wsName, rec, found := app.WorkspaceStore.SessionRecordFor(pane.Session)
			tabName := logicalTabName(app, paneID)
			ctx := buildWhereContext(pane, wsName, rec, found, tabName)

			if asJSON {
				enc, err := json.MarshalIndent(ctx, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(enc))
				return nil
			}
			renderWhere(cmd.OutOrStdout(), ctx)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the context as JSON")
	return cmd
}

// buildWhereContext composes the answer from already-resolved inputs. Pure (no
// I/O) so the fallback rules — unmanaged session, clone collapse, missing label
// or tab — are unit-tested directly.
//
// SessionTmux is the live session name (grouped dev-b clones collapsed to their
// root via RootName), never the stored record name: the name tmux reports for
// the current pane is always a valid -s handle, whereas a record's TmuxName can
// lag a session-rename migration. "What -s reaches me" must reflect live state.
func buildWhereContext(pane tmux.Pane, wsName string, rec workspace.WorkspaceSession, wsFound bool, tabName string) whereContext {
	root := session.RootName(pane.Session)
	ctx := whereContext{
		Tab:         tabName,
		PaneID:      pane.ID,
		WindowIndex: pane.WindowIndex,
		Dir:         pane.Dir,
		SessionTmux: root,
	}
	if wsFound {
		ctx.Workspace = wsName
		ctx.Session = rec.Label
	}
	if ctx.Session == "" {
		ctx.Session = root // unmanaged / raw session: the root name is the label
	}
	if ctx.Tab == "" {
		// Unclaimed pane (no logical tab): the tmux window name is the closest
		// tab-like identity. pane_title is a last resort — apps rewrite it via
		// OSC escapes, so it can read as an editor/terminal title, not a tab.
		ctx.Tab = pane.WindowName
		if ctx.Tab == "" {
			ctx.Tab = pane.Title
		}
	}
	return ctx
}

// currentPaneByID returns the current window's pane matching id, or a bare
// pane carrying just the id when the scan misses (e.g. detached).
func currentPaneByID(app *apppkg.App, paneID string) tmux.Pane {
	if panes, err := app.Runner.ListWindowPanes(""); err == nil {
		for _, p := range panes {
			if p.ID == paneID {
				return p
			}
		}
	}
	return tmux.Pane{ID: paneID}
}

// logicalTabName returns the logical tab's display name for a pane, or "" when
// the pane is not (yet) a zmux-managed tab.
func logicalTabName(app *apppkg.App, paneID string) string {
	all, err := tabs.ListLogicalTabs(app.Runner)
	if err != nil {
		return ""
	}
	for i := range all {
		if all[i].PaneID == paneID {
			return tabs.DisplayName(&all[i])
		}
	}
	return ""
}

func renderWhere(w io.Writer, ctx whereContext) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "workspace\t%s\n", dashIfEmpty(ctx.Workspace))
	if ctx.SessionTmux != "" && ctx.SessionTmux != ctx.Session {
		fmt.Fprintf(tw, "session\t%s\t(%s)\n", dashIfEmpty(ctx.Session), ctx.SessionTmux)
	} else {
		fmt.Fprintf(tw, "session\t%s\n", dashIfEmpty(ctx.Session))
	}
	fmt.Fprintf(tw, "tab\t%s\t%s\n", dashIfEmpty(ctx.Tab), ctx.PaneID)
	fmt.Fprintf(tw, "cwd\t%s\n", dashIfEmpty(ctx.Dir))
	_ = tw.Flush()
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
