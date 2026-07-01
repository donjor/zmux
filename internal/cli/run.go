package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/recipe"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/recipeup"
	"github.com/spf13/cobra"
)

// isRosterTabName reports whether name is a canonical roster tab — the shared
// roster the zmux skill teaches (dev/scratch/agent-shells, plus peer/worker
// patterns). Ad-hoc names outside it earn a one-line reuse nudge. Keep in sync
// with the roster in skills/zmux/SKILL.md.
func isRosterTabName(name string) bool {
	switch name {
	case "dev", "scratch", "claude", "codex":
		return true
	}
	return strings.HasSuffix(name, "-peer") || strings.HasPrefix(name, "worker")
}

// callerLifecycle reads the lifecycle origin/scope of the pane the command was
// invoked from ($TMUX_PANE), enabling origin inheritance: a `run` fired from
// inside an agent-shell tab is itself agent-originated. Returns empties when not
// in a pane or unstamped — ResolveOrigin then falls back to env/human.
func callerLifecycle(app *apppkg.App) (origin, scope string) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return "", ""
	}
	origin, _ = app.Runner.ShowPaneOption(pane, tabs.OptOrigin)
	scope, _ = app.Runner.ShowPaneOption(pane, tabs.OptScope)
	return origin, scope
}

func newRunCmd(app *apppkg.App) *cobra.Command {
	var runWindowName string
	var runSessionFlag string
	var runDetach bool
	var runFollow bool
	var runTimeout int
	var runFollowLines int
	var runYes bool
	var runDryRun bool
	var runForceCommand bool
	var runWorkspace string
	var runCWD string
	var runRecipeSession string
	var runTabMode string
	var runTTL time.Duration
	var runKeep bool
	var runScope string
	var runOrigin string

	cmd := &cobra.Command{
		Use:   "run <recipe|command>",
		Short: "Run a recipe or a command in a named tab",
		Long: `Run a recipe, or run a command in a named tab.

If the first argument names a recipe and no tab-target flags are present, zmux
opens the recipe runner. Use -y/--yes to accept recipe defaults.

Otherwise, zmux run keeps its command-in-tab behavior and waits for completion
by default. Use --command when a command name collides with a recipe name.

By default, zmux run:
  1. Creates/reuses a named tab
  2. Runs the command
  3. Waits for it to finish (prints output live)
  4. Returns the command's exit code

For long-running processes (servers), use --detach or --follow:
  --detach/-d  Fire and forget. Don't wait for completion.
  --follow/-f  Tail output live. Ctrl+C stops following (process keeps running).

Examples:
  zmux run dev                                # recipe form with defaults
  zmux run dev -y                             # recipe defaults, no form
  zmux run dev --dry-run                      # print recipe plan
  zmux run --command dev -n dev               # force command mode
  zmux run 'npm test' -n test                 # wait for completion (default)
  zmux run 'make build' -n build -T 60        # wait with 60s timeout
  zmux run 'npm run dev' -n server -d         # detach (server, don't wait)
  zmux run 'npm run dev' -n server -f         # follow output live
  zmux run 'npm test' -n tests -s myproject   # target specific session`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			MaybeReap(app, time.Now()) // GC stale tabs before spawning a new one
			if !runForceCommand && shouldDispatchRecipeRun(cmd, app, args, runWindowName, runSessionFlag, runFollow, runDetach, runYes, runDryRun) {
				return runRecipeFromRun(app, cmd, args[0], args[1:], recipe.PlanOptions{
					CWD:       runCWD,
					Workspace: runWorkspace,
					Session:   runRecipeSession,
					TabMode:   runTabMode,
					Detach:    runDetach,
				}, runYes, runDryRun)
			}
			if runYes || runDryRun || runWorkspace != "" || runCWD != "" || runRecipeSession != "" || runTabMode != "" {
				return fmt.Errorf("recipe flags require a recipe name; %q is not a recipe", args[0])
			}

			command := strings.Join(args, " ")
			if !tabs.ValidScope(runScope) {
				return fmt.Errorf("unknown lifecycle scope %q (want task|daemon|shell|peer|worker|agent-shell)", runScope)
			}

			// Determine target session.
			sessionName, err := resolveSessionTarget(app, runSessionFlag)
			if err != nil {
				return err
			}

			if !app.Runner.HasSession(sessionName) {
				return fmt.Errorf("session %q does not exist", sessionName)
			}

			// Determine tab name.
			name := runWindowName
			if name == "" {
				parts := strings.Fields(command)
				if len(parts) > 0 {
					name = parts[0]
				} else {
					name = "task"
				}
			}

			// Completion and glyph state are owned by shell lifecycle hooks in
			// the target pane. A blocking run stages a nonce in pane metadata;
			// shell-event start/end consumes it and writes a silent
			// @zmux_run_result. No stdout sentinel or tab-state epilogue is
			// appended in the normal path.
			shouldWait := !runDetach && !runFollow
			actualCommand := command
			nonce := ""
			if shouldWait {
				nonce = runNonce()
			}

			// Simple single-line commands are typed verbatim at the prompt so
			// they land in the tab's shell history — a human can Up-arrow to
			// re-run them (restart a dev server, re-run tests). Anything that
			// interactive bash or send-keys could reinterpret falls back to a
			// temp script, which sidesteps all quoting issues.
			sendCmd := actualCommand
			if !isSimpleCommand(actualCommand) {
				scriptPath, cleanup, err := writeCommandScript(actualCommand, runTimeout)
				if err != nil {
					return fmt.Errorf("write command script: %w", err)
				}
				if cleanup != nil {
					defer cleanup()
				}
				sendCmd = fmt.Sprintf("bash %s", scriptPath)
			}

			// Find an existing tab to reuse. The choke point matches a
			// logical tab (id → label, wherever its pane lives — full,
			// pane-of, docked) before the legacy window label/name pass, and
			// claims an explicitly-named unlabeled name-match so the first
			// restart after tmux auto-renames it still reuses. Names derived
			// from the command never claim — incidental names must not
			// become stable labels.
			rt, err := resolveTabTargetForMutation(app, sessionName, name, runWindowName)
			if err != nil {
				return err
			}
			if rt.found() {
				// Delivering input clears stale done|failed first (ratified
				// clear table: typing-by-proxy = user input) — same as send/type.
				rt.clearStale(app)
				waitPane := ""
				if shouldWait {
					waitPane = runWaitPaneID(app, rt)
					if waitPane == "" {
						return fmt.Errorf("cannot resolve pane for lifecycle wait on %s:%s", sessionName, name)
					}
					if err := stageRunID(app, waitPane, nonce); err != nil {
						return err
					}
				}
				if err := app.Runner.SendKeys(rt.Target, sendCmd, "Enter"); err != nil {
					return fmt.Errorf("send to %s: %w", rt.Target, err)
				}
				fmt.Fprintf(os.Stderr, "sent to %s:%s\n", sessionName, name)
				if shouldWait {
					return waitForRunResult(app, rt.Target, waitPane, nonce, runTimeout, runFollowLines)
				}
				if runFollow {
					return followOutput(app, rt.Target, runFollowLines)
				}
				return nil
			}

			// No existing tab — create one. Surface this when a name was
			// explicitly requested so the operator notices a fresh tab rather
			// than a silent duplicate.
			if runWindowName != "" {
				fmt.Fprintf(os.Stderr, "no tab %q in %s — creating\n", name, sessionName)
				// Roster nudge (plan 038): a fresh ad-hoc named tab is the
				// sprawl slip. Non-blocking, create-only; --keep / --scope daemon
				// and peer/worker names opt out (they're deliberate roster tabs).
				if !isRosterTabName(name) && !runKeep && runScope != tabs.ScopeDaemon && runScope != tabs.ScopePeer && runScope != tabs.ScopeWorker {
					fmt.Fprintf(os.Stderr, "tip: %q is an ad-hoc tab — reuse 'scratch' (one-offs) or 'dev' (runtime) to avoid sprawl (zmux skill: tab roster)\n", name)
				}
			}

			dir, _ := os.Getwd()
			var newOpts []tmux.WindowOpt
			if runDetach {
				// Don't yank the user's focus to a fire-and-forget tab.
				newOpts = append(newOpts, tmux.Detached())
			}
			paneID, err := app.Runner.NewWindow(sessionName, name, dir, newOpts...)
			if err != nil {
				return fmt.Errorf("create tab: %w", err)
			}

			// Target the new pane directly — the id stays valid across
			// auto-rename and placement moves mid-run. session:name is the
			// fallback for runners that don't report a pane id.
			target := fmt.Sprintf("%s:%s", sessionName, name)
			if paneID != "" {
				target = paneID
			}

			// Stamp identity at birth on all run-created panes so shell
			// lifecycle hooks have a managed pane to update. Only an explicit
			// -n becomes a stable pane label; command-derived names remain
			// incidental and can auto-rename without becoming an address pin.
			if paneID != "" {
				label, labelSource := "", ""
				if runWindowName != "" {
					label, labelSource = name, tablabel.SourcePane
				}
				if _, err := tabs.Stamp(app.Runner, paneID, paneID, label, labelSource); err != nil {
					return fmt.Errorf("stamp tab: %w", err)
				}
				// Lifecycle birth stamp (plan 038): records origin/scope/born so
				// the reaper can reason about this tab. Set-once on born, so a
				// reused tab keeps its original identity. Non-fatal — hygiene
				// metadata must never break the actual run.
				scope := runScope
				if scope == "" {
					scope = tabs.ScopeTask
				}
				callerOrigin, callerScope := callerLifecycle(app)
				origin := tabs.ResolveOrigin(runOrigin, callerOrigin, callerScope, os.Getenv("ZMUX_ACTOR"))
				_ = tabs.StampBirth(app.Runner, paneID, origin, scope, time.Now())
				if runTTL > 0 {
					_ = tabs.SetTTL(app.Runner, paneID, runTTL)
				}
				if runKeep {
					_ = tabs.SetKeep(app.Runner, paneID, true)
				}
				_ = tabs.TouchInput(app.Runner, paneID, time.Now())
			}

			waitPane := ""
			if shouldWait {
				waitPane = target
				if paneID != "" {
					waitPane = paneID
				}
				if err := stageRunID(app, waitPane, nonce); err != nil {
					return err
				}
			}
			if err := app.Runner.SendKeys(target, sendCmd, "Enter"); err != nil {
				return fmt.Errorf("send to %s: %w", target, err)
			}

			fmt.Fprintf(os.Stderr, "running in %s:%s\n", sessionName, name)

			if shouldWait {
				return waitForRunResult(app, target, waitPane, nonce, runTimeout, runFollowLines)
			}
			if runFollow {
				return followOutput(app, target, runFollowLines)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&runWindowName, "name", "n", "", "tab name (default: derived from command)")
	cmd.Flags().StringVarP(&runSessionFlag, "session", "s", "", "target session (default: current session)")
	cmd.Flags().BoolVarP(&runDetach, "detach", "d", false, "fire and forget (don't wait for completion)")
	cmd.Flags().BoolVarP(&runFollow, "follow", "f", false, "tail output live (Ctrl+C to stop)")
	cmd.Flags().IntVarP(&runTimeout, "timeout", "T", 120, "timeout in seconds (default 120)")
	cmd.Flags().IntVar(&runFollowLines, "lines", 50, "lines to capture when tailing")
	cmd.Flags().BoolVarP(&runYes, "yes", "y", false, "run recipe defaults without the form")
	cmd.Flags().BoolVar(&runDryRun, "dry-run", false, "print a recipe plan and exit")
	cmd.Flags().BoolVar(&runForceCommand, "command", false, "force command-in-tab mode even if the name matches a recipe")
	cmd.Flags().StringVar(&runWorkspace, "workspace", "", "recipe workspace override")
	cmd.Flags().StringVar(&runCWD, "cwd", "", "recipe working directory override")
	cmd.Flags().StringVar(&runRecipeSession, "recipe-session", "", "recipe session override")
	cmd.Flags().StringVar(&runTabMode, "tab-mode", "", "recipe tab mode: run, ready, or empty")
	cmd.Flags().DurationVar(&runTTL, "ttl", 0, "auto-reap this tab after it's idle this long (e.g. 30m, 2h)")
	cmd.Flags().BoolVar(&runKeep, "keep", false, "never auto-reap this tab")
	cmd.Flags().StringVar(&runScope, "scope", "", "lifecycle scope: task (default), daemon, shell, peer, worker, agent-shell")
	cmd.Flags().StringVar(&runOrigin, "origin", "", "lifecycle origin override: agent|human (default inferred)")
	_ = cmd.Flags().MarkHidden("origin")
	return cmd
}

func shouldDispatchRecipeRun(cmd *cobra.Command, app *apppkg.App, args []string, tabName string, sessionFlag string, follow bool, detach bool, yes bool, dryRun bool) bool {
	if len(args) == 0 || tabName != "" || sessionFlag != "" || follow {
		return false
	}
	if detach && !yes && !dryRun {
		return false
	}
	if cmd.Flags().Changed("timeout") || cmd.Flags().Changed("lines") {
		return false
	}
	defs, err := loadRecipeDefinitions(app)
	if err != nil {
		return false
	}
	_, ok := recipe.Find(defs, args[0])
	return ok
}

func runRecipeFromRun(app *apppkg.App, cmd *cobra.Command, name string, items []string, opts recipe.PlanOptions, yes bool, dryRun bool) error {
	opts.Items = append([]string(nil), items...)
	defs, err := loadRecipeDefinitions(app)
	if err != nil {
		return err
	}
	def, ok := recipe.Find(defs, name)
	if !ok {
		return fmt.Errorf("recipe %q not found (available: %s)", name, recipe.JoinNames(defs))
	}
	if yes || dryRun {
		plan, err := planRecipe(app, def.Recipe, opts)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), recipe.RenderPlan(plan))
		if dryRun {
			return nil
		}
		return recipe.Execute(app.Runner, app.WorkspaceStore, plan)
	}
	return recipeup.RunRecipe(app, name, items, opts)
}

// runNonce returns a random hex token scoping a command result to one
// invocation. crypto/rand so concurrent runs can never mint the same nonce (a
// clock-based nonce theoretically could). Clock fallback only if the system RNG
// is unreadable, which Go treats as effectively impossible.
func runNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// isSimpleCommand reports whether a command can be typed verbatim at an
// interactive shell prompt without changing meaning, so it lands in shell
// history and a human can Up-arrow to re-run it. Anything an interactive
// shell or tmux send-keys could reinterpret falls back to the temp-script
// path:
//   - newlines / CR (each line would submit separately)
//   - tabs (the pty delivers a literal Tab — triggers shell completion)
//   - `!` (bash history expansion fires even inside double quotes)
//   - backslashes (escape handling differs between prompt and script)
//   - a leading dash (send-keys would parse it as a flag)
//   - a single token starting uppercase (could collide with a tmux key
//     name like Enter or Up; real commands conventionally start lowercase)
func isSimpleCommand(cmd string) bool {
	if cmd == "" || strings.ContainsAny(cmd, "\n\r\t!\\") {
		return false
	}
	if strings.HasPrefix(cmd, "-") {
		return false
	}
	if !strings.ContainsRune(cmd, ' ') && cmd[0] >= 'A' && cmd[0] <= 'Z' {
		return false
	}
	return true
}

// writeCommandScript writes a command to a temp script file for safe execution
// via send-keys. Returns the script path and a cleanup function.
// This avoids shell quoting issues with newlines, quotes, and special characters.
func writeCommandScript(command string, timeoutSec int) (string, func(), error) {
	f, err := os.CreateTemp("", "zmux-cmd-*.sh")
	if err != nil {
		return "", nil, err
	}

	// Echo the command so the terminal shows what's being run, then execute it,
	// preserving the command's exit status even when the script self-deletes.
	// Use printf to avoid echo interpreting escape sequences.
	escaped := strings.ReplaceAll(command, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '\\033[2m$ %%s\\033[0m\\n' %q\n__zmux_cmd_cleanup() { __zmux_status=$?; rm -f %q; exit $__zmux_status; }\ntrap __zmux_cmd_cleanup EXIT\n%s\n", escaped, f.Name(), command)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()

	// The script self-deletes (rm -f at the end), so cleanup is a no-op.
	// But provide a fallback cleanup in case it never runs.
	cleanupDelay := time.Duration(timeoutSec+10) * time.Second
	cleanup := func() {
		go func(delay time.Duration) {
			time.Sleep(delay)
			os.Remove(f.Name())
		}(cleanupDelay)
	}

	return f.Name(), cleanup, nil
}

func runWaitPaneID(app *apppkg.App, rt resolvedTab) string {
	if rt.Tab != nil {
		return rt.Tab.PaneID
	}
	if rt.stateOK {
		return rt.state.PaneID
	}
	svc := tabstate.New(app.Runner, os.Getenv)
	t, err := rt.stateTarget(svc)
	if err != nil {
		return ""
	}
	return t.PaneID
}

func stageRunID(app *apppkg.App, paneID, nonce string) error {
	if paneID == "" || nonce == "" {
		return fmt.Errorf("cannot stage empty run id")
	}
	if err := app.Runner.SetPaneOption(paneID, tabs.OptNextRunID, nonce); err != nil {
		return fmt.Errorf("stage run result channel: %w", err)
	}
	_ = app.Runner.UnsetPaneOption(paneID, tabs.OptRunResult)
	return nil
}

// waitForRunResult watches pane output while waiting for the shell lifecycle
// hook to publish @zmux_run_result=<nonce>:<exit>. Completion is metadata, not
// terminal text, so stdout stays free of AGENT_DONE-style sentinels.
var runLifecycleStartGrace = 5 * time.Second

func waitForRunResult(app *apppkg.App, target, paneID, nonce string, timeoutSec, followLines int) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	timeout := time.Duration(timeoutSec) * time.Second
	deadline := time.Now().Add(timeout)
	startDeadline := time.Now().Add(runLifecycleStartDeadline(timeout))
	started := false
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	seen := map[string]bool{}
	printNew := func(output string) {
		for _, line := range strings.Split(output, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !seen[trimmed] {
				seen[trimmed] = true
				fmt.Println(line)
			}
		}
	}

	for {
		select {
		case <-sig:
			return fmt.Errorf("interrupted")

		case <-ticker.C:
			output, err := app.Runner.CapturePane(target, followLines)
			if err == nil {
				printNew(output)
			}
			if exitCode, ok := runResultExit(app, paneID, nonce); ok {
				_ = app.Runner.UnsetPaneOption(paneID, tabs.OptRunResult)
				if exitCode != 0 {
					return fmt.Errorf("command exited with code %d", exitCode)
				}
				return nil
			}
			if !started && runLifecycleStarted(app, paneID, nonce) {
				started = true
			}
			if !started && time.Now().After(startDeadline) {
				_ = app.Runner.UnsetPaneOption(paneID, tabs.OptNextRunID)
				return fmt.Errorf("timeout after %ds waiting for shell lifecycle to start in the target pane (run `zmux setup shell` and open a fresh tab, or use --detach/--follow for REPLs/TUIs/non-interactive shells)", int(runLifecycleStartDeadline(timeout).Seconds()))
			}
			if time.Now().After(deadline) {
				if err != nil {
					if output, _ := app.Runner.CapturePane(target, followLines); output != "" {
						fmt.Print(output)
					}
				}
				return fmt.Errorf("timeout after %ds waiting for shell lifecycle result (run `zmux setup shell` for the target shell, or inspect output with `zmux watch`)", timeoutSec)
			}
		}
	}
}

func runLifecycleStartDeadline(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > runLifecycleStartGrace {
		return runLifecycleStartGrace
	}
	return timeout
}

func runLifecycleStarted(app *apppkg.App, paneID, nonce string) bool {
	next, err := app.Runner.ShowPaneOption(paneID, tabs.OptNextRunID)
	if err != nil {
		return false
	}
	return strings.TrimSpace(next) != nonce
}

func runResultExit(app *apppkg.App, paneID, nonce string) (int, bool) {
	value, err := app.Runner.ShowPaneOption(paneID, tabs.OptRunResult)
	if err != nil || value == "" {
		return 0, false
	}
	prefix := nonce + ":"
	if !strings.HasPrefix(strings.TrimSpace(value), prefix) {
		return 0, false
	}
	exitCode, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(value, prefix)))
	if err != nil {
		return 0, false
	}
	return exitCode, true
}

// followOutput tails the output of a tmux pane, printing new content as it appears.
func followOutput(app *apppkg.App, target string, followLines int) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	lastLen := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "--- following %s (Ctrl+C to stop) ---\n", target)

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "\n--- stopped following ---")
			return nil
		case <-ticker.C:
			output, err := app.Runner.CapturePane(target, followLines)
			if err != nil {
				continue
			}

			lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
			if len(lines) > lastLen {
				for _, line := range lines[lastLen:] {
					fmt.Println(line)
				}
				lastLen = len(lines)
			} else if len(lines) < lastLen {
				lastLen = len(lines)
			}
		}
	}
}
