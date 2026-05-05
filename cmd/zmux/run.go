package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var runWindowName string
var runSessionFlag string
var runDetach bool
var runFollow bool
var runTimeout int
var runFollowLines int

const zmuxSentinel = ":::AGENT_DONE "

var runCmd = &cobra.Command{
	Use:   "run <command>",
	Short: "Run a command in a named tab",
	Long: `Run a command in a named tab. Waits for completion by default.

By default, zmux run:
  1. Creates/reuses a named tab
  2. Runs the command
  3. Waits for it to finish (prints output live)
  4. Returns the command's exit code

For long-running processes (servers), use --detach or --follow:
  --detach/-d  Fire and forget. Don't wait for completion.
  --follow/-f  Tail output live. Ctrl+C stops following (process keeps running).

Examples:
  zmux run 'npm test' -n test                 # wait for completion (default)
  zmux run 'make build' -n build -T 60        # wait with 60s timeout
  zmux run 'npm run dev' -n server -d         # detach (server, don't wait)
  zmux run 'npm run dev' -n server -f         # follow output live
  zmux run 'npm test' -n tests -s myproject   # target specific session`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		command := strings.Join(args, " ")

		// Determine target session.
		sessionName := runSessionFlag
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

		// Unless detached or following, wrap command with sentinel for completion detection.
		shouldWait := !runDetach && !runFollow
		actualCommand := command
		if shouldWait {
			actualCommand = fmt.Sprintf("%s; echo \"%s$?:::\"", command, zmuxSentinel)
		}

		// Write command to a temp script to avoid shell quoting issues
		// with send-keys (newlines, quotes, special chars all break).
		scriptPath, cleanup, err := writeCommandScript(actualCommand)
		if err != nil {
			return fmt.Errorf("write command script: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}
		sendCmd := fmt.Sprintf("bash %s", scriptPath)

		target := fmt.Sprintf("%s:%s", sessionName, name)

		// Check if tab already exists.
		windows, err := app.Runner.ListWindows(sessionName)
		if err == nil {
			for _, w := range windows {
				if w.Name == name {
					if err := app.Runner.SendKeys(target, sendCmd, "Enter"); err != nil {
						return fmt.Errorf("send to %s: %w", target, err)
					}
					fmt.Fprintf(os.Stderr, "sent to %s:%s\n", sessionName, name)
					if shouldWait {
						return waitForSentinel(target, runTimeout)
					}
					if runFollow {
						return followOutput(target)
					}
					return nil
				}
			}
		}

		// Create new tab and run command.
		dir, _ := os.Getwd()
		if err := app.Runner.NewWindow(sessionName, name, dir); err != nil {
			return fmt.Errorf("create tab: %w", err)
		}

		if err := app.Runner.SendKeys(target, sendCmd, "Enter"); err != nil {
			return fmt.Errorf("send to %s: %w", target, err)
		}

		fmt.Fprintf(os.Stderr, "running in %s:%s\n", sessionName, name)

		if shouldWait {
			return waitForSentinel(target, runTimeout)
		}
		if runFollow {
			return followOutput(target)
		}
		return nil
	},
}

// writeCommandScript writes a command to a temp script file for safe execution
// via send-keys. Returns the script path and a cleanup function.
// This avoids shell quoting issues with newlines, quotes, and special characters.
func writeCommandScript(command string) (string, func(), error) {
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
	// Capture the package-level timeout before launching the delayed cleanup
	// goroutine; tests and future commands may mutate runTimeout after return.
	cleanupDelay := time.Duration(runTimeout+10) * time.Second
	cleanup := func() {
		// Give it time to execute before cleaning up.
		// The script self-deletes, so this is just a safety net.
		go func(delay time.Duration) {
			time.Sleep(delay)
			os.Remove(f.Name())
		}(cleanupDelay)
	}

	return f.Name(), cleanup, nil
}

// waitForSentinel watches a tab for the AGENT_DONE sentinel and returns the exit code.
func waitForSentinel(target string, timeoutSec int) error {
	sentinelRe := regexp.MustCompile(`:::AGENT_DONE (\d+):::`)

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
				output, _ := app.Runner.CapturePane(target, runFollowLines)
				if output != "" {
					fmt.Print(output)
				}
				return fmt.Errorf("timeout after %ds", timeoutSec)
			}

			output, err := app.Runner.CapturePane(target, runFollowLines)
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

					// Check for sentinel — don't print it.
					if m := sentinelRe.FindStringSubmatch(trimmed); m != nil {
						exitCode, _ := strconv.Atoi(m[1])
						if exitCode != 0 {
							return fmt.Errorf("command exited with code %d", exitCode)
						}
						return nil
					}

					fmt.Println(line)
				}
			}
		}
	}
}

// followOutput tails the output of a tmux pane, printing new content as it appears.
func followOutput(target string) error {
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
			output, err := app.Runner.CapturePane(target, runFollowLines)
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

func init() {
	runCmd.Flags().StringVarP(&runWindowName, "name", "n", "", "tab name (default: derived from command)")
	runCmd.Flags().StringVarP(&runSessionFlag, "session", "s", "", "target session (default: current session)")
	runCmd.Flags().BoolVarP(&runDetach, "detach", "d", false, "fire and forget (don't wait for completion)")
	runCmd.Flags().BoolVarP(&runFollow, "follow", "f", false, "tail output live (Ctrl+C to stop)")
	runCmd.Flags().IntVarP(&runTimeout, "timeout", "T", 120, "timeout in seconds (default 120)")
	runCmd.Flags().IntVar(&runFollowLines, "lines", 50, "lines to capture when tailing")
	rootCmd.AddCommand(runCmd)
}
