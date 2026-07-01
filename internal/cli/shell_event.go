package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newShellEventCmd(app *apppkg.App) *cobra.Command {
	var paneFlag string
	var commandFlag string
	var exitFlag int

	cmd := &cobra.Command{
		Use:    "shell-event <start|end>",
		Short:  "Record shell command lifecycle events (internal)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "start":
				return runShellEventStart(app, paneFlag, commandFlag)
			case "end":
				return runShellEventEnd(app, paneFlag, exitFlag)
			default:
				return fmt.Errorf("unknown shell event %q (want start|end)", args[0])
			}
		},
	}
	cmd.Flags().StringVar(&paneFlag, "pane", "", "target pane (default: $TMUX_PANE)")
	cmd.Flags().StringVar(&commandFlag, "command", "", "sanitized command text for diagnostics")
	cmd.Flags().IntVar(&exitFlag, "exit", 0, "exit code for end events")
	return cmd
}

func runShellEventStart(app *apppkg.App, pane, command string) error {
	target, ok := shellEventTarget(app, pane)
	if !ok {
		return nil
	}
	if err := ensureManagedShellPane(app, target.PaneID); err != nil {
		return nil // shell hooks fail open: unmanaged/dead panes simply don't publish state
	}
	now := time.Now()
	seq := nextCommandSeq(app.Runner, target.PaneID)
	runID, _ := app.Runner.ShowPaneOption(target.PaneID, tabs.OptNextRunID)
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdSeq, Value: strconv.Itoa(seq)},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdState, Value: tabs.CmdRunning},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdStartedAt, Value: strconv.FormatInt(now.Unix(), 10)},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdFinishedAt, Unset: true},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdLastExit, Unset: true},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptRunResult, Unset: true},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptNextRunID, Unset: true},
	}
	if runID != "" {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdRunID, Value: strings.TrimSpace(runID)})
	} else {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdRunID, Unset: true})
	}
	if text := sanitizeCommandText(command); text != "" {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdText, Value: text})
	} else {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdText, Unset: true})
	}
	if err := app.Runner.ApplyOptions(writes); err != nil {
		return nil
	}
	_ = tabstate.New(app.Runner, os.Getenv).Set(target, tabstate.StateRunning, "shell", "")
	return nil
}

func runShellEventEnd(app *apppkg.App, pane string, exitCode int) error {
	target, ok := shellEventTarget(app, pane)
	if !ok {
		return nil
	}
	if err := ensureManagedShellPane(app, target.PaneID); err != nil {
		return nil
	}
	now := time.Now()
	state := tabs.CmdDone
	glyph := tabstate.StateDone
	msg := ""
	if exitCode != 0 {
		state = tabs.CmdFailed
		glyph = tabstate.StateFailed
		msg = fmt.Sprintf("exit %d", exitCode)
	}
	runID, _ := app.Runner.ShowPaneOption(target.PaneID, tabs.OptCmdRunID)
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdState, Value: state},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdFinishedAt, Value: strconv.FormatInt(now.Unix(), 10)},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdLastExit, Value: strconv.Itoa(exitCode)},
		{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptCmdRunID, Unset: true},
	}
	if runID != "" {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: target.PaneID, Key: tabs.OptRunResult, Value: fmt.Sprintf("%s:%d", strings.TrimSpace(runID), exitCode)})
	}
	if err := app.Runner.ApplyOptions(writes); err != nil {
		return nil
	}
	_ = tabstate.New(app.Runner, os.Getenv).Set(target, glyph, "shell", msg)
	return nil
}

func shellEventTarget(app *apppkg.App, pane string) (tabstate.Target, bool) {
	if pane == "" {
		pane = os.Getenv("TMUX_PANE")
	}
	if pane == "" {
		return tabstate.Target{}, false
	}
	target, err := tabstate.ResolveTarget(app.Runner, pane, os.Getenv)
	if err != nil {
		return tabstate.Target{}, false
	}
	return target, true
}

func ensureManagedShellPane(app *apppkg.App, paneID string) error {
	id, err := app.Runner.ShowPaneOption(paneID, tabs.OptTabID)
	if err != nil {
		return err
	}
	if id == "" {
		windowID, derr := app.Runner.DisplayMessage(paneID, "#{window_id}")
		if derr == nil {
			_ = adoptWindow(app, strings.TrimSpace(windowID))
		}
		id, err = app.Runner.ShowPaneOption(paneID, tabs.OptTabID)
		if err != nil {
			return err
		}
	}
	if id == "" {
		return fmt.Errorf("pane %s is not a zmux-managed tab", paneID)
	}
	origin, _ := app.Runner.ShowPaneOption(paneID, tabs.OptOrigin)
	if origin == "" {
		_ = tabs.StampBirth(app.Runner, paneID, tabs.OriginHuman, tabs.ScopeShell, time.Now())
	}
	return nil
}

func nextCommandSeq(r tmux.Runner, paneID string) int {
	cur, err := r.ShowPaneOption(paneID, tabs.OptCmdSeq)
	if err != nil {
		return 1
	}
	n, err := strconv.Atoi(strings.TrimSpace(cur))
	if err != nil || n < 0 {
		return 1
	}
	return n + 1
}

func sanitizeCommandText(command string) string {
	command = strings.Join(strings.Fields(command), " ")
	if len(command) > 160 {
		command = strings.TrimSpace(command[:160])
	}
	return command
}
