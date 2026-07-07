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
	if isIgnoredLifecycleCommand(command) {
		return nil
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
	svc := tabstate.New(app.Runner, os.Getenv)
	interactive := isDaemonVenuePane(app, target.PaneID) || isInteractiveVenueCommand(command)
	sig := tabs.DisplaySignals{CommandState: tabs.CmdRunning, CommandSource: "shell", CommandInteractive: interactive}
	sig.ManualState, sig.ManualSource = currentAttentionSignal(app, target.PaneID)
	res := tabs.ResolveDisplayState(sig)
	if !res.Set {
		clearShellOwnedDisplayState(app, svc, target)
		return nil
	}
	_ = svc.Set(target, res.State, res.Source, res.Message)
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
	if exitCode != 0 {
		state = tabs.CmdFailed
	}
	command, _ := app.Runner.ShowPaneOption(target.PaneID, tabs.OptCmdText)
	interactive := isDaemonVenuePane(app, target.PaneID) || isInteractiveVenueCommand(command)
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
	svc := tabstate.New(app.Runner, os.Getenv)
	sig := tabs.DisplaySignals{CommandState: state, CommandSource: "shell", CommandInteractive: interactive, CommandExit: strconv.Itoa(exitCode)}
	sig.ManualState, sig.ManualSource = currentAttentionSignal(app, target.PaneID)
	res := tabs.ResolveDisplayState(sig)
	if !res.Set {
		clearShellOwnedDisplayState(app, svc, target)
		return nil
	}
	_ = svc.Set(target, res.State, res.Source, res.Message)
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
		// A pane can carry a scope without birth identity (daemon tabs get
		// scoped at creation); backfilling birth must not clobber it.
		scope, _ := app.Runner.ShowPaneOption(paneID, tabs.OptScope)
		scope = strings.TrimSpace(scope)
		if scope == "" {
			scope = tabs.ScopeShell
		}
		_ = tabs.StampBirth(app.Runner, paneID, tabs.OriginHuman, scope, time.Now())
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

func clearShellOwnedDisplayState(app *apppkg.App, svc *tabstate.Service, target tabstate.Target) {
	source, _ := app.Runner.ShowPaneOption(target.PaneID, tabstate.OptSource)
	switch strings.TrimSpace(source) {
	case "", "shell", "run", "state-exit", "claude-stop", "codex-stop", "pi-agent", "turn":
		_ = svc.Clear(target)
	}
}

func currentAttentionSignal(app *apppkg.App, paneID string) (string, string) {
	state, _ := app.Runner.ShowPaneOption(paneID, tabstate.OptState)
	if strings.TrimSpace(state) != string(tabstate.StateAttention) {
		return "", ""
	}
	source, _ := app.Runner.ShowPaneOption(paneID, tabstate.OptSource)
	return strings.TrimSpace(state), strings.TrimSpace(source)
}

func isDaemonVenuePane(app *apppkg.App, paneID string) bool {
	scope, _ := app.Runner.ShowPaneOption(paneID, tabs.OptScope)
	return strings.TrimSpace(scope) == tabs.ScopeDaemon
}

func isIgnoredLifecycleCommand(command string) bool {
	cmd := sanitizeCommandText(command)
	return cmd == "" || cmd == "builtin exit \"$@\"" || cmd == "unset i" || cmd == "unset i | unset i"
}

func isInteractiveVenueCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	for len(fields) > 0 {
		word := trimShellWord(fields[0])
		switch {
		case strings.Contains(word, "=") && !strings.HasPrefix(word, "="):
			fields = fields[1:]
			continue
		case word == "command" || word == "exec" || word == "env" || word == "noglob":
			fields = fields[1:]
			continue
		case word == "sudo" && len(fields) > 1:
			fields = fields[1:]
			continue
		}
		break
	}
	if len(fields) == 0 {
		return false
	}
	cmd := trimShellWord(fields[0])
	base := cmd
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "pi", "claude", "codex", "agy", "ssh", "nvim", "vim", "less", "man", "top", "htop", "btop", "tmux":
		return true
	case "bash", "zsh", "fish", "sh", "nu":
		return len(fields) == 1 || fields[1] == "-i" || fields[1] == "--login" || fields[1] == "-l"
	case "python", "python3", "node", "deno", "bun", "irb", "ruby", "php", "psql", "sqlite3", "mysql", "redis-cli":
		return len(fields) == 1 || fields[1] == "-i" || fields[1] == "--interactive"
	}
	return false
}

func trimShellWord(s string) string {
	return strings.Trim(s, "'\"")
}
