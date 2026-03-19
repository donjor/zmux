package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var watchLines int
var watchSessionFlag string
var watchFollow bool
var watchUntil string
var watchTimeout int

var watchCmd = &cobra.Command{
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
  zmux watch git -s myproject                    # from specific session`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		windowName := args[0]

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

		target := fmt.Sprintf("%s:%s", sessionName, windowName)

		if watchUntil != "" {
			return watchUntilPattern(target, watchUntil, watchTimeout)
		}

		if watchFollow {
			runFollowLines = watchLines
			return followOutput(target)
		}

		output, err := app.Runner.CapturePane(target, watchLines)
		if err != nil {
			return fmt.Errorf("capture %s: %w", target, err)
		}

		fmt.Print(output)
		return nil
	},
}

// watchUntilPattern polls a tab's output until a regex pattern is found in NEW output.
// Takes a baseline snapshot first — only matches against lines that appear AFTER the watch starts.
// This prevents false matches from stale output of previous commands.
func watchUntilPattern(target, pattern string, timeoutSec int) error {
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

func init() {
	watchCmd.Flags().IntVarP(&watchLines, "lines", "l", 50, "number of lines to capture")
	watchCmd.Flags().BoolVarP(&watchFollow, "follow", "f", false, "follow output (tail -f style)")
	watchCmd.Flags().StringVar(&watchUntil, "until", "", "wait for regex pattern in output")
	watchCmd.Flags().IntVarP(&watchTimeout, "timeout", "T", 120, "timeout in seconds for --until (default 120)")
	watchCmd.Flags().StringVarP(&watchSessionFlag, "session", "s", "", "target session (default: current)")
	rootCmd.AddCommand(watchCmd)
}
