package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/waitfor"
	"github.com/spf13/cobra"
)

type resolvedWaitTarget struct {
	Session string
	Target  string
	PaneID  string
	TabName string
}

func newWaitCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var forSpec string
	var lines int
	var timeoutSec int
	var jsonOut bool
	var allowStale bool
	var freshAfter int

	cmd := &cobra.Command{
		Use:   "wait <tab>",
		Short: "Wait for tab output, idle, command lifecycle, or peer turn lifecycle",
		Long: `Wait for one first-class zmux condition on a tab.

Conditions are closed and explicit:
  turn:ready|attention|failed|running
  cmd:done|failed|running|exit=N
  output:<regex>
  idle:<seconds-or-duration>

Lifecycle waits are fresh by default: stale ready/done state that existed before
this wait started does not satisfy the condition. Use --allow-stale only for
manual diagnostics where current state is intentionally enough.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := waitfor.ParseCondition(forSpec)
			if err != nil {
				return err
			}
			target, err := resolveWaitTarget(app, args[0], sessionFlag, false)
			if err != nil {
				return err
			}
			var baseline *waitfor.Baseline
			if freshAfter > 0 {
				b := waitfor.SnapshotBaseline(app.Runner, target.PaneID)
				switch cond.Kind {
				case waitfor.ConditionTurn:
					b.TurnSeq = freshAfter
				case waitfor.ConditionCmd:
					b.CmdSeq = freshAfter
				}
				baseline = &b
			}
			outcome, err := waitfor.Wait(context.Background(), waitfor.Request{
				Runner:     app.Runner,
				Target:     target.Target,
				PaneID:     target.PaneID,
				Lines:      lines,
				Timeout:    time.Duration(timeoutSec) * time.Second,
				Condition:  cond,
				AllowStale: allowStale,
				Baseline:   baseline,
			})
			if err != nil && !jsonOut {
				return err
			}
			if jsonOut {
				b, merr := json.MarshalIndent(struct {
					Tab     string          `json:"tab"`
					Session string          `json:"session,omitempty"`
					Target  string          `json:"target"`
					Outcome waitfor.Outcome `json:"outcome"`
				}{Tab: target.TabName, Session: target.Session, Target: target.Target, Outcome: outcome}, "", "  ")
				if merr != nil {
					return merr
				}
				fmt.Println(string(b))
				if err != nil {
					return err
				}
				if !outcome.Met {
					return fmt.Errorf("wait condition not met: %s", outcome.FailureKind)
				}
				return nil
			}
			printWaitOutcome(target, outcome)
			if !outcome.Met {
				return fmt.Errorf("wait condition not met: %s", emptyForHuman(outcome.FailureKind, "unproven"))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session (default: current)")
	cmd.Flags().StringVar(&forSpec, "for", "", "condition: turn:ready, cmd:done, cmd:exit=0, output:<regex>, idle:3s")
	_ = cmd.MarkFlagRequired("for")
	cmd.Flags().IntVarP(&timeoutSec, "timeout", "T", 10, "wait timeout in seconds")
	cmd.Flags().IntVarP(&lines, "lines", "l", 120, "output lines to capture")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
	cmd.Flags().BoolVar(&allowStale, "allow-stale", false, "allow current lifecycle state to satisfy turn/cmd waits")
	cmd.Flags().IntVar(&freshAfter, "fresh-after", 0, "generation floor for lifecycle waits (cmd seq or turn seq)")
	return cmd
}

func resolveWaitTarget(app *apppkg.App, tabName, sessionFlag string, mutation bool) (resolvedWaitTarget, error) {
	sessionName, err := resolveSessionTarget(app, sessionFlag)
	if err != nil {
		return resolvedWaitTarget{}, err
	}
	var rt resolvedTab
	if mutation {
		rt, err = resolveTabTargetForMutation(app, sessionName, tabName, tabName)
	} else {
		rt, err = resolveTabTarget(app, sessionName, tabName)
	}
	if err != nil {
		return resolvedWaitTarget{}, err
	}
	if !rt.found() {
		return resolvedWaitTarget{}, fmt.Errorf("no tab %q in session %q", tabName, sessionName)
	}
	target := rt.Target
	paneID := ""
	if rt.Tab != nil {
		paneID = rt.Tab.PaneID
		if tabName == "" {
			tabName = rt.Tab.Label
		}
	} else {
		svc := tabstate.New(app.Runner, os.Getenv)
		st, err := rt.stateTarget(svc)
		if err != nil {
			return resolvedWaitTarget{}, err
		}
		paneID = st.PaneID
	}
	if paneID == "" {
		return resolvedWaitTarget{}, fmt.Errorf("could not resolve pane for tab %q", tabName)
	}
	return resolvedWaitTarget{Session: sessionName, Target: target, PaneID: paneID, TabName: tabName}, nil
}

func printWaitOutcome(target resolvedWaitTarget, outcome waitfor.Outcome) {
	state := outcome.State
	if state == "" {
		state = string(outcome.Basis)
	}
	status := "unproven"
	if outcome.Met {
		status = "met"
	}
	fmt.Printf("wait %s for %s: %s (basis=%s fresh=%s)\n", status, target.TabName, state, outcome.Basis, strconv.FormatBool(outcome.Fresh))
	for _, warning := range outcome.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if strings.TrimSpace(outcome.OutputTail) != "" {
		fmt.Printf("output:\n%s", outcome.OutputTail)
		if !strings.HasSuffix(outcome.OutputTail, "\n") {
			fmt.Println()
		}
	}
}

func emptyForHuman(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
