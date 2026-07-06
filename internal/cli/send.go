package cli

import (
	"fmt"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

func newSendCmd(app *apppkg.App) *cobra.Command {
	var sendSessionFlag string

	cmd := &cobra.Command{
		Use:   "send <window> <keys...>",
		Short: "Send keystrokes to a named window",
		Long: `Send keystrokes to a specific window in the current (or specified) session.
Useful for agents to type commands into shared terminals.

Examples:
  zmux send server C-c                    # Ctrl+C to stop server
  zmux send git 'git push origin main' Enter
  zmux send admin 'sudo apt update' Enter
  zmux send devserver 'npm run build' Enter --session myproject`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			windowName := args[0]
			keys := args[1:]

			// Determine target session.
			sessionName, err := resolveSessionTarget(app, sendSessionFlag)
			if err != nil {
				return err
			}

			rt, err := resolveTabTargetForMutation(app, sessionName, windowName, windowName)
			if err != nil {
				return err
			}

			// Input acknowledges an answer/finished tab: clear ready|done|failed first
			// (ratified clear table — attention/running stay).
			rt.clearStale(app)

			if err := app.Runner.SendKeys(rt.Target, keys...); err != nil {
				return fmt.Errorf("send to %s: %w", rt.Target, err)
			}

			fmt.Printf("sent to %s\n", rt.Target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sendSessionFlag, "session", "s", "", "target session (default: current)")
	return cmd
}

func newTypeCmd(app *apppkg.App) *cobra.Command {
	var sendSessionFlag string

	cmd := &cobra.Command{
		Use:   "type <window> <text>",
		Short: "Type text into a window and press Enter",
		Long: `Convenience command — sends text followed by Enter to a named window.

Examples:
  zmux type git 'git status'
  zmux type server 'npm run dev'`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			windowName := args[0]
			text := strings.Join(args[1:], " ")

			sessionName, err := resolveSessionTarget(app, sendSessionFlag)
			if err != nil {
				return err
			}

			rt, err := resolveTabTargetForMutation(app, sessionName, windowName, windowName)
			if err != nil {
				return err
			}
			target := rt.Target

			rt.clearStale(app)

			// Send text and Enter as separate key events with a gap between
			// them. TUI input boxes (agent CLIs like codex) detect rapid
			// input as a paste burst; an Enter glued to the text in the same
			// send-keys call gets absorbed into the paste as a newline and
			// the message silently never submits. Shells don't care about
			// the gap.
			if err := app.Runner.SendKeys(target, "-l", text); err != nil {
				return fmt.Errorf("type to %s: %w", target, err)
			}
			time.Sleep(typeGap(len(text)))
			if err := app.Runner.SendKeys(target, "Enter"); err != nil {
				return fmt.Errorf("type to %s: %w", target, err)
			}

			fmt.Printf("typed to %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sendSessionFlag, "session", "s", "", "target session (default: current)")
	return cmd
}

// typeGap returns how long to wait between the text and the Enter key.
// The TUI paste-burst window scales with paste size: a large paste takes
// longer to ingest, and an Enter arriving mid-ingest is absorbed into the
// paste as a newline (observed live: ~1KB prompts into codex with a fixed
// 200ms gap silently never submit; Claude first-turn composer also needed
// a wider gap during agent-surface E2E). Shells don't care about the gap,
// so bias toward reliable TUI submission and cap the wait at a still-bounded
// interactive delay.
func typeGap(textLen int) time.Duration {
	gap := 750*time.Millisecond + time.Duration(textLen)*2*time.Millisecond
	if gap > 2500*time.Millisecond {
		return 2500 * time.Millisecond
	}
	return gap
}
