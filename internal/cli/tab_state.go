package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/spf13/cobra"
)

// newTabStateCmd is the `zmux tab state` verb (plan 026 P1): write/clear tab
// lifecycle states on pane options (canonical) + window mirror, so the bar
// can signal attention/failed/running/ready/done from any tab.
func newTabStateCmd(app *apppkg.App) *cobra.Command {
	var (
		targetFlag   string
		sessionFlag  string
		sourceFlag   string
		msgFlag      string
		ifFlag       string
		quietFlag    bool
		byVisibility bool
	)

	cmd := &cobra.Command{
		Use:   "state <attention|failed|running|ready|done|clear> [target]",
		Short: "Set or clear a tab's lifecycle state (bar glyph)",
		Long: `Set or clear a tab lifecycle state. State is stored on the pane
(canonical — survives future placement moves) and mirrored to the window for
the status bar. No target means the calling pane ($TMUX_PANE).

Targets: a tab name in the current session (label-aware), a pane id (%N), or
a raw tmux target (session:window).

Examples:
  zmux tab state running buddy --source run
  zmux tab state ready --source claude-stop --quiet
  zmux tab state done --source run --quiet --by-visibility
  zmux tab state clear --target '%12' --if attention --source focus --quiet
  zmux tab state failed test --msg 'exit 2'`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runTabState(app, cmd, tabStateArgs{
				action:       args[0],
				positional:   argOrEmpty(args, 1),
				target:       targetFlag,
				session:      sessionFlag,
				source:       sourceFlag,
				msg:          msgFlag,
				ifState:      ifFlag,
				byVisibility: byVisibility,
			})
			if quietFlag {
				// Hook mode: hooks outside tmux, dead panes, detached servers —
				// none of it may surface as noise. Validation errors also stay
				// silent in the terminal, but land in the ZMUX_DEBUG log so a
				// broken hook command is diagnosable without dropping --quiet.
				if err != nil {
					debug.Log("tab state --quiet swallowed error", "err", err)
				}
				return nil
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&targetFlag, "target", "t", "", "target pane/window/tab (overrides positional)")
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&sourceFlag, "source", "", "state source label (run, focus, claude-stop, ...)")
	cmd.Flags().StringVar(&msgFlag, "msg", "", "optional message (display-only)")
	cmd.Flags().StringVar(&ifFlag, "if", "", "clear only when current state matches (clear action only)")
	cmd.Flags().BoolVar(&quietFlag, "quiet", false, "hook mode: never fail, never print")
	cmd.Flags().BoolVar(&byVisibility, "by-visibility", false, "done action only: store attention instead when the pane is not visible")
	return cmd
}

type tabStateArgs struct {
	action       string
	positional   string
	target       string
	session      string
	source       string
	msg          string
	ifState      string
	byVisibility bool
}

func runTabState(app *apppkg.App, cmd *cobra.Command, a tabStateArgs) error {
	svc := tabstate.New(app.Runner, os.Getenv)

	tgt, err := resolveTabStateTarget(app, svc, a)
	if err != nil {
		return err
	}

	if a.action == "clear" {
		if a.byVisibility {
			return fmt.Errorf("--by-visibility only applies to the done action")
		}
		if a.ifState != "" {
			want, err := tabstate.Parse(a.ifState)
			if err != nil {
				return err
			}
			cleared, err := svc.ClearIf(tgt, want)
			if err != nil {
				return err
			}
			if cleared {
				fmt.Fprintf(cmd.OutOrStdout(), "tab state cleared (%s): %s\n", want, tgt.Window)
			}
			return nil
		}
		if err := svc.Clear(tgt); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "tab state cleared: %s\n", tgt.Window)
		return nil
	}

	if a.ifState != "" {
		return fmt.Errorf("--if only applies to the clear action")
	}
	st, err := tabstate.Parse(a.action)
	if err != nil {
		return err
	}
	if a.byVisibility {
		if st != tabstate.StateDone {
			return fmt.Errorf("--by-visibility only applies to the done action")
		}
		written, err := svc.SetDoneByVisibility(tgt, a.source, a.msg)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "tab state: %s → %s\n", written, tgt.Window)
		return nil
	}
	if err := svc.Set(tgt, st, a.source, a.msg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tab state: %s → %s\n", st, tgt.Window)
	return nil
}

// resolveTabStateTarget turns flag/positional input into a resolved state
// target. Bare tab names (no % prefix, no colon) resolve through the logical
// choke point in the current/--session session — the same path send/type use
// — so `tab state running buddy` works after tmux auto-renames the window
// and follows the tab across placement moves. Pane/window specs and the
// empty spec go straight to the service's ladder ($TMUX_PANE → client pane).
func resolveTabStateTarget(app *apppkg.App, svc *tabstate.Service, a tabStateArgs) (tabstate.Target, error) {
	spec := a.target
	if spec == "" {
		spec = a.positional
	}
	if spec == "" || strings.HasPrefix(spec, "%") || strings.Contains(spec, ":") {
		return svc.Resolve(spec)
	}

	sessionName := a.session
	if sessionName == "" {
		if !app.Runner.IsInsideTmux() {
			return tabstate.Target{}, errors.New("tab-name target outside tmux — use --session")
		}
		name, err := app.Runner.DisplayMessage("", "#{session_name}")
		if err != nil {
			return tabstate.Target{}, fmt.Errorf("current session: %w", err)
		}
		sessionName = strings.TrimSpace(name)
	}
	rt, err := resolveTabTargetForMutation(app, sessionName, spec, spec)
	if err != nil {
		return tabstate.Target{}, err
	}
	if !rt.found() {
		return tabstate.Target{}, fmt.Errorf("no tab %q in session %q", spec, sessionName)
	}
	return rt.stateTarget(svc)
}

func argOrEmpty(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// newTabStateExitCmd maps an exit code to a state write: 0 → done, anything
// else → failed with "exit N". Hidden compatibility shim for older hooks or
// external scripts; normal command lifecycle is now owned by shell-event hooks.
// Always silent: it executes at the user's prompt and must never noise it.
func newTabStateExitCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "state-exit <code>",
		Short:  "Write done/failed from an exit code (compat shim)",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := strconv.Atoi(args[0])
			if err != nil {
				return nil // garbage in, no glyph out — stay silent
			}
			if code == 0 {
				markTabState(app, "", tabstate.StateDone, "run", "")
			} else {
				markTabState(app, "", tabstate.StateFailed, "run", fmt.Sprintf("exit %d", code))
			}
			return nil
		},
	}
}

// markTabState best-effort sets a lifecycle state on a tmux target spec.
// Lifecycle hook writes must never fail the command that triggered them — a
// dead pane or detached server just skips the glyph. Resolves the spec at write
// time, so a pane that moved placement mid-run still mirrors to the right window.
func markTabState(app *apppkg.App, target string, st tabstate.State, source, msg string) {
	svc := tabstate.New(app.Runner, os.Getenv)
	t, err := svc.Resolve(target)
	if err != nil {
		return
	}
	_ = svc.Set(t, st, source, msg)
}
