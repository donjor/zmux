package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/recipe"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/recipeup"
	"github.com/spf13/cobra"
)

const zmuxSentinelPrefix = ":::AGENT_DONE"

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

			// Unless detached or following, wrap command with sentinel for
			// completion detection. The nonce scopes the sentinel to this
			// invocation — a reused tab can still show a previous run's
			// sentinel on screen, which must never satisfy this wait.
			shouldWait := !runDetach && !runFollow
			var actualCommand, nonce string
			if shouldWait {
				nonce = runNonce()
				actualCommand = fmt.Sprintf("%s; echo \"%s:%s:$?:::\"", command, zmuxSentinelPrefix, nonce)
			} else {
				// Detached/followed runs: nobody waits on a sentinel, but the
				// running glyph must still stop when the command does. An exit
				// epilogue writes done/failed from inside the pane (state-exit
				// maps $? → state). SelfBin keeps the write on this profile's
				// socket — a bare `zmux` in the pane's PATH would hit the live
				// binary from a zzmux pane.
				actualCommand = fmt.Sprintf("%s; %s tab state-exit $?", command, config.SelfBin(app.Profile))
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
				if err := app.Runner.SendKeys(rt.Target, sendCmd, "Enter"); err != nil {
					return fmt.Errorf("send to %s: %w", rt.Target, err)
				}
				// run-start sets running (ratified clear table) — targets the
				// reused tab's pane, never the caller's $TMUX_PANE.
				rt.markState(app, tabstate.StateRunning, "run", "")
				fmt.Fprintf(os.Stderr, "sent to %s:%s\n", sessionName, name)
				if shouldWait {
					return waitForSentinel(app, rt.Target, nonce, runTimeout, runFollowLines)
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

			// Stamp identity at birth on explicitly-named tabs: pane-scoped
			// id + pane-canonical label (window mirror rides along), so a
			// later `run -n <name>` finds the logical tab wherever it lives.
			// Set before the command runs, while the window name still matches.
			if runWindowName != "" && paneID != "" {
				if _, err := tabs.Stamp(app.Runner, paneID, paneID, name, tablabel.SourcePane); err != nil {
					return fmt.Errorf("stamp tab: %w", err)
				}
			}

			if err := app.Runner.SendKeys(target, sendCmd, "Enter"); err != nil {
				return fmt.Errorf("send to %s: %w", target, err)
			}
			markTabState(app, target, tabstate.StateRunning, "run", "")

			fmt.Fprintf(os.Stderr, "running in %s:%s\n", sessionName, name)

			if shouldWait {
				return waitForSentinel(app, target, nonce, runTimeout, runFollowLines)
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

// runNonce returns a random hex token scoping a sentinel to one invocation —
// crypto/rand so concurrent runs can never mint the same nonce (a clock-based
// nonce theoretically could). Clock fallback only if the system RNG is
// unreadable, which Go treats as effectively impossible.
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

	// Echo the command so the terminal shows what's being run,
	// then execute it, then self-delete the script.
	// Use printf to avoid echo interpreting escape sequences.
	escaped := strings.ReplaceAll(command, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '\\033[2m$ %%s\\033[0m\\n' %q\n%s\nrm -f %q\n", escaped, command, f.Name())
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

// waitForSentinel watches a tab for this run's AGENT_DONE sentinel and returns
// the exit code. Matching is scoped by the per-invocation nonce — a reused tab
// can still show a previous run's sentinel (or one recalled from shell
// history), which must never satisfy this wait.
func waitForSentinel(app *apppkg.App, target, nonce string, timeoutSec, followLines int) error {
	sentinelRe := regexp.MustCompile(zmuxSentinelPrefix + ":" + regexp.QuoteMeta(nonce) + `:(\d+):::`)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	seen := map[string]bool{}

	for {
		select {
		case <-sig:
			return fmt.Errorf("interrupted")

		case <-ticker.C:
			if time.Now().After(deadline) {
				output, _ := app.Runner.CapturePane(target, followLines)
				if output != "" {
					fmt.Print(output)
				}
				return fmt.Errorf("timeout after %ds", timeoutSec)
			}

			output, err := app.Runner.CapturePane(target, followLines)
			if err != nil {
				continue
			}

			lines := strings.Split(output, "\n")

			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if !seen[trimmed] {
					seen[trimmed] = true

					// Check for sentinel — don't print it. Sentinel exit is the
					// only place that may write done/failed: a timeout means
					// the result is unknown, so the tab stays `running` rather
					// than fabricating completion.
					if m := sentinelRe.FindStringSubmatch(trimmed); m != nil {
						exitCode, _ := strconv.Atoi(m[1])
						if exitCode != 0 {
							markTabState(app, target, tabstate.StateFailed, "run", fmt.Sprintf("exit %d", exitCode))
							return fmt.Errorf("command exited with code %d", exitCode)
						}
						markTabState(app, target, tabstate.StateDone, "run", "")
						return nil
					}

					fmt.Println(line)
				}
			}
		}
	}
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
