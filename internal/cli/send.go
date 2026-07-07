package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/waitfor"
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

type typeCommandOutput struct {
	Tab        string           `json:"tab"`
	Session    string           `json:"session,omitempty"`
	Target     string           `json:"target"`
	PaneID     string           `json:"paneId,omitempty"`
	Typed      bool             `json:"typed"`
	Outcome    *waitfor.Outcome `json:"outcome,omitempty"`
	Status     waitfor.Status   `json:"status"`
	OutputTail string           `json:"outputTail,omitempty"`
	Warnings   []string         `json:"warnings,omitempty"`
}

func newTypeCmd(app *apppkg.App) *cobra.Command {
	var sendSessionFlag string
	var waitTurn string
	var waitCmd string
	var markPeerRunning bool
	var timeoutSec int
	var lines int
	var source string
	var msg string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "type <window> <text>",
		Short: "Type text into a window and press Enter",
		Long: `Convenience command — sends text followed by Enter to a named window.

Examples:
  zmux type git 'git status'
  zmux type server 'npm run dev'
  zmux type claude-peer 'review this' --mark-peer-running --wait-turn ready --json`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			windowName := args[0]
			text := strings.Join(args[1:], " ")
			if waitTurn != "" && waitCmd != "" {
				return fmt.Errorf("--wait-turn and --wait-cmd cannot be combined")
			}

			sessionName, err := resolveSessionTarget(app, sendSessionFlag)
			if err != nil {
				return err
			}

			rt, err := resolveTabTargetForMutation(app, sessionName, windowName, windowName)
			if err != nil {
				return err
			}
			target := rt.Target
			paneID, _ := paneTargetForResolvedTab(app, rt)
			baseline := waitfor.SnapshotBaseline(app.Runner, paneID)

			rt.clearStale(app)
			if markPeerRunning {
				svc := tabstate.New(app.Runner, os.Getenv)
				stateTarget, err := rt.stateTarget(svc)
				if err != nil {
					return err
				}
				if err := runTabPeerAction(app, svc, stateTarget, "running", tabPeerOptions{source: source, msg: msg}); err != nil {
					return err
				}
			}

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

			out := typeCommandOutput{Tab: windowName, Session: sessionName, Target: target, PaneID: paneID, Typed: true}
			var cond waitfor.Condition
			if waitTurn != "" {
				cond, err = waitfor.ParseCondition("turn:" + waitTurn)
			} else if waitCmd != "" {
				cond, err = waitfor.ParseCondition("cmd:" + waitCmd)
			}
			if err != nil {
				return err
			}
			if cond.Kind != "" {
				waited, _ := waitfor.Wait(context.Background(), waitfor.Request{Runner: app.Runner, Target: target, PaneID: paneID, Lines: lines, Timeout: time.Duration(timeoutSec) * time.Second, Condition: cond, Baseline: &baseline})
				out.Outcome = &waited
				if !waited.Met {
					out.Warnings = append(out.Warnings, "wait not proven: "+emptyForHuman(waited.FailureKind, "unproven"))
				}
			}
			out.Status = waitfor.ReadStatus(app.Runner, paneID)
			out.OutputTail, _ = app.Runner.CapturePane(target, lines)
			out.Warnings = uniqueStrings(append(out.Warnings, waitfor.WarningsForStatus(out.Status)...))
			if jsonOut {
				b, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				if out.Outcome != nil && !out.Outcome.Met {
					return fmt.Errorf("type wait not met: %s", emptyForHuman(out.Outcome.FailureKind, "unproven"))
				}
				return nil
			}
			printTypeOutcome(out)
			if out.Outcome != nil && !out.Outcome.Met {
				return fmt.Errorf("type wait not met: %s", emptyForHuman(out.Outcome.FailureKind, "unproven"))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&sendSessionFlag, "session", "s", "", "target session (default: current)")
	cmd.Flags().StringVar(&waitTurn, "wait-turn", "", "fresh peer turn state to wait for after typing")
	cmd.Flags().StringVar(&waitCmd, "wait-cmd", "", "fresh shell command state to wait for after typing")
	cmd.Flags().BoolVar(&markPeerRunning, "mark-peer-running", false, "stamp peer lifecycle as running after typing")
	cmd.Flags().IntVarP(&timeoutSec, "timeout", "T", 8, "wait timeout in seconds")
	cmd.Flags().IntVarP(&lines, "lines", "l", 80, "output lines to include in wait result")
	cmd.Flags().StringVar(&source, "source", "type", "lifecycle source label when marking peer running")
	cmd.Flags().StringVar(&msg, "msg", "", "optional glyph message when marking peer running")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
	return cmd
}

func printTypeOutcome(out typeCommandOutput) {
	fmt.Printf("typed to %s\n", out.Target)
	if out.Outcome != nil {
		fmt.Printf("wait: met=%t state=%s basis=%s fresh=%t\n", out.Outcome.Met, out.Outcome.State, out.Outcome.Basis, out.Outcome.Fresh)
	}
	for _, warning := range out.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if out.Outcome != nil && strings.TrimSpace(out.OutputTail) != "" {
		fmt.Println("output:")
		fmt.Print(out.OutputTail)
		if !strings.HasSuffix(out.OutputTail, "\n") {
			fmt.Println()
		}
	}
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
