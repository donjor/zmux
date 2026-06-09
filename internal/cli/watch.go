package cli

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

func newWatchCmd(app *apppkg.App) *cobra.Command {
	var watchLines int
	var watchSessionFlag string
	var watchFollow bool
	var watchUntil string
	var watchIdle int
	var watchTimeout int

	cmd := &cobra.Command{
		Use:   "watch <tab>",
		Short: "Capture output from a named tab",
		Long: `Capture and print the recent output from a tmux tab.
Useful for agents to read terminal output without attaching.

Examples:
  zmux watch server                              # last 50 lines
  zmux watch server --lines 100                  # last 100 lines
  zmux watch server -f                           # follow (tail -f)
  zmux watch server --until "ready on port"      # wait for pattern
  zmux watch server --until "error|failed" -T 30 # wait with timeout
  zmux watch buddy --idle 3                      # wait until quiet for 3s
  zmux watch git -s myproject                    # from specific session`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			windowName := args[0]

			if cmd.Flags().Changed("idle") {
				if watchIdle <= 0 {
					return fmt.Errorf("--idle must be a positive number of seconds")
				}
				if watchUntil != "" || watchFollow {
					return fmt.Errorf("--idle cannot be combined with --until or --follow")
				}
			}

			sessionName := watchSessionFlag
			if sessionName == "" {
				if app.Runner.IsInsideTmux() {
					name, err := app.Runner.DisplayMessage("", "#{session_name}")
					if err != nil {
						return fmt.Errorf("not inside a tmux session")
					}
					sessionName = name
				} else {
					return fmt.Errorf("not inside tmux — use --session to specify target")
				}
			}

			rt, err := resolveTabTarget(app, sessionName, windowName)
			if err != nil {
				return err
			}
			target := rt.Target

			if watchUntil != "" {
				return watchUntilPattern(app, target, watchUntil, watchTimeout, watchLines)
			}

			if watchIdle > 0 {
				return watchUntilIdle(app, target, watchIdle, watchTimeout, watchLines)
			}

			if watchFollow {
				return followOutput(app, target, watchLines)
			}

			output, err := app.Runner.CapturePane(target, watchLines)
			if err != nil {
				return fmt.Errorf("capture %s: %w", target, err)
			}

			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().IntVarP(&watchLines, "lines", "l", 50, "number of lines to capture")
	cmd.Flags().BoolVarP(&watchFollow, "follow", "f", false, "follow output (tail -f style)")
	cmd.Flags().StringVar(&watchUntil, "until", "", "wait for regex pattern in output")
	cmd.Flags().IntVar(&watchIdle, "idle", 0, "wait until output is unchanged for N seconds, then print it")
	cmd.Flags().IntVarP(&watchTimeout, "timeout", "T", 120, "timeout in seconds for --until / --idle (default 120)")
	cmd.Flags().StringVarP(&watchSessionFlag, "session", "s", "", "target session (default: current)")
	return cmd
}

// watchUntilIdle polls a tab's output until the capture is byte-stable for
// idleSec seconds, then prints the final capture and returns nil.
//
// Stability means the SCREEN stopped changing — not that the process is done.
// A TUI thinking quietly (e.g. an agent CLI waiting on a remote model) goes
// byte-stable mid-turn by design; the caller reads the printed capture and
// judges what state it represents. Contract: exit 0 = stable for idleSec;
// timeout returns non-zero with a best-effort capture (fresh, falling back
// to the last good one); interrupts and other errors are also non-zero.
func watchUntilIdle(app *apppkg.App, target string, idleSec, timeoutSec, watchLines int) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	idleDur := time.Duration(idleSec) * time.Second
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "waiting for %s to be stable for %ds (timeout %ds)...\n", target, idleSec, timeoutSec)

	var last string
	var stableSince time.Time // zero = no trusted capture yet
	var lastCaptureErr error

	// Seed the stability streak immediately — without this, the first sample
	// would only land on the first tick (+500ms) and an already-stable pane
	// could never prove "stable for N seconds" inside --timeout N. A failed
	// seed just leaves the streak unstarted; the first tick retries.
	if output, err := app.Runner.CapturePane(target, watchLines); err == nil {
		last = output
		stableSince = time.Now()
	}

	for {
		select {
		case <-sig:
			return fmt.Errorf("interrupted")

		case <-ticker.C:
			// Evaluate stability BEFORE the deadline: a pane that proves
			// stable on the same tick the timeout expires is a success —
			// the caller wants the capture, not a coin-flip on ordering.
			// (CapturePane is a local tmux call; it is not itself bounded
			// by --timeout.)
			output, err := app.Runner.CapturePane(target, watchLines)
			if err != nil {
				// A failed capture proves nothing about stability — distrust
				// the streak entirely rather than risk a false "stable".
				stableSince = time.Time{}
				lastCaptureErr = err
			} else {
				lastCaptureErr = nil
				if stableSince.IsZero() || output != last {
					last = output
					stableSince = time.Now()
				} else if time.Since(stableSince) >= idleDur {
					fmt.Print(output)
					return nil
				}
			}

			if time.Now().After(deadline) {
				// Best-effort capture so the caller can still judge content.
				if err == nil {
					fmt.Print(output)
				} else if last != "" {
					fmt.Print(last)
				}
				if lastCaptureErr != nil {
					return fmt.Errorf("timeout after %ds — captures failing (%v)", timeoutSec, lastCaptureErr)
				}
				return fmt.Errorf("timeout after %ds — output never stable for %ds", timeoutSec, idleSec)
			}
		}
	}
}

// watchUntilPattern polls a tab's output until a regex pattern is found in NEW output.
// Takes a baseline snapshot first — only matches against lines that appear AFTER the watch starts.
// This prevents false matches from stale output of previous commands.
func watchUntilPattern(app *apppkg.App, target, pattern string, timeoutSec int, watchLines int) error {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	// Capture baseline — everything currently visible is "old" output.
	baseline := map[string]bool{}
	if initial, err := app.Runner.CapturePane(target, watchLines); err == nil {
		for _, line := range strings.Split(initial, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				baseline[trimmed] = true
			}
		}
	}

	timeout := time.Duration(timeoutSec) * time.Second
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	printed := map[string]bool{} // deduplicate output lines

	fmt.Fprintf(os.Stderr, "waiting for /%s/ in %s (timeout %ds)...\n", pattern, target, timeoutSec)

	for {
		select {
		case <-sig:
			return fmt.Errorf("interrupted")

		case <-ticker.C:
			if time.Now().After(deadline) {
				output, _ := app.Runner.CapturePane(target, watchLines)
				if output != "" {
					fmt.Print(output)
				}
				return fmt.Errorf("timeout after %ds — pattern /%s/ not found", timeoutSec, pattern)
			}

			output, err := app.Runner.CapturePane(target, watchLines)
			if err != nil {
				continue
			}

			lines := strings.Split(output, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}

				isNew := !baseline[trimmed]

				// Print new lines as they appear.
				if isNew && !printed[trimmed] {
					printed[trimmed] = true
					fmt.Println(line)
				}

				// Only match pattern against NEW output (not baseline).
				if isNew && re.MatchString(line) {
					return nil
				}
			}
		}
	}
}
