package cli

import (
	"fmt"
	"strings"

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
			sessionName := sendSessionFlag
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

			if err := app.Runner.SendKeys(target, keys...); err != nil {
				return fmt.Errorf("send to %s: %w", target, err)
			}

			fmt.Printf("sent to %s\n", target)
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

			sessionName := sendSessionFlag
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

			if err := app.Runner.SendKeys(target, text, "Enter"); err != nil {
				return fmt.Errorf("type to %s: %w", target, err)
			}

			fmt.Printf("typed to %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sendSessionFlag, "session", "s", "", "target session (default: current)")
	return cmd
}
