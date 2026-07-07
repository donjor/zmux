package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/waitfor"
	"github.com/spf13/cobra"
)

type peerEnsureOutput struct {
	Tab         string           `json:"tab"`
	Session     string           `json:"session,omitempty"`
	Target      string           `json:"target"`
	PaneID      string           `json:"paneId"`
	Created     bool             `json:"created"`
	Reused      bool             `json:"reused"`
	Restarted   bool             `json:"restarted"`
	CommandSent bool             `json:"commandSent"`
	Outcome     *waitfor.Outcome `json:"outcome,omitempty"`
	Readiness   *waitfor.Outcome `json:"readiness,omitempty"`
	Status      waitfor.Status   `json:"status"`
	OutputTail  string           `json:"outputTail,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
}

func newTabPeerEnsureCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var commandFlag string
	var roleFlag string
	var hostTabFlag string
	var hostPaneFlag string
	var topicFlag string
	var sourceFlag string
	var msgFlag string
	var readinessFlag string
	var waitTurnFlag string
	var timeoutSec int
	var lines int
	var restart bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "ensure <tab>",
		Short: "Create or reuse a peer tab and optionally wait for readiness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := runTabPeerEnsure(app, peerEnsureOptions{
				tab:        args[0],
				session:    sessionFlag,
				command:    commandFlag,
				role:       roleFlag,
				hostTab:    hostTabFlag,
				hostPane:   hostPaneFlag,
				topic:      topicFlag,
				source:     sourceFlag,
				msg:        msgFlag,
				readiness:  readinessFlag,
				waitTurn:   waitTurnFlag,
				timeoutSec: timeoutSec,
				lines:      lines,
				restart:    restart,
			})
			if jsonOut {
				b, merr := json.MarshalIndent(out, "", "  ")
				if merr != nil {
					return merr
				}
				fmt.Println(string(b))
				return err
			}
			printPeerEnsure(out)
			return err
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&commandFlag, "command", "", "command to start when the peer tab is missing or --restart is set")
	cmd.Flags().StringVar(&roleFlag, "role", "", "peer role/CLI label")
	cmd.Flags().StringVar(&hostTabFlag, "host-tab", "", "stable host logical tab id")
	cmd.Flags().StringVar(&hostPaneFlag, "host-pane", "", "host pane id")
	cmd.Flags().StringVar(&topicFlag, "topic", "", "sanitized display topic/title")
	cmd.Flags().StringVar(&sourceFlag, "source", "peer", "lifecycle source label")
	cmd.Flags().StringVar(&msgFlag, "msg", "", "optional glyph message")
	cmd.Flags().StringVar(&readinessFlag, "readiness", "", "regex to wait for in new output")
	cmd.Flags().StringVar(&waitTurnFlag, "wait-turn", "", "fresh peer turn state to wait for, e.g. ready|attention|failed|running")
	cmd.Flags().IntVarP(&timeoutSec, "timeout", "T", 10, "wait timeout in seconds")
	cmd.Flags().IntVarP(&lines, "lines", "l", 120, "output lines to capture")
	cmd.Flags().BoolVar(&restart, "restart", false, "send C-c and run --command even when the tab already exists")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
	return cmd
}

type peerEnsureOptions struct {
	tab        string
	session    string
	command    string
	role       string
	hostTab    string
	hostPane   string
	topic      string
	source     string
	msg        string
	readiness  string
	waitTurn   string
	timeoutSec int
	lines      int
	restart    bool
}

func runTabPeerEnsure(app *apppkg.App, o peerEnsureOptions) (peerEnsureOutput, error) {
	if o.lines <= 0 {
		o.lines = 120
	}
	if o.timeoutSec <= 0 {
		o.timeoutSec = 10
	}
	sessionName, err := resolveSessionTarget(app, o.session)
	if err != nil {
		return peerEnsureOutput{}, err
	}
	rt, err := resolveTabTargetForMutation(app, sessionName, o.tab, o.tab)
	if err != nil {
		return peerEnsureOutput{}, err
	}
	exists := rt.found()
	baseline := waitfor.Baseline{}
	if exists {
		paneID, _ := paneForResolvedTab(app, rt)
		baseline = waitfor.SnapshotBaseline(app.Runner, paneID)
	}
	out := peerEnsureOutput{Tab: o.tab, Session: sessionName, Reused: exists && !o.restart}
	var warnings []string

	if exists && o.restart {
		_ = app.Runner.SendKeys(rt.Target, "C-c")
		out.Restarted = true
	}

	if !exists {
		if strings.TrimSpace(o.command) == "" {
			return out, fmt.Errorf("peer tab %q does not exist in %q and --command was not provided", o.tab, sessionName)
		}
		paneID, target, err := createPeerCommandTab(app, sessionName, o.tab, o.command, o)
		if err != nil {
			return out, err
		}
		out.Created = true
		out.CommandSent = true
		out.Target = target
		out.PaneID = paneID
		baseline = waitfor.Baseline{}
	} else {
		paneID, target := paneTargetForResolvedTab(app, rt)
		out.Target = target
		out.PaneID = paneID
		if o.restart && strings.TrimSpace(o.command) != "" {
			if err := markPeerRunning(app, paneID, target, o); err != nil {
				warnings = append(warnings, "peer lifecycle start failed: "+err.Error())
			}
			if err := sendPreparedCommand(app, target, o.command, o.timeoutSec); err != nil {
				return out, err
			}
			out.CommandSent = true
		} else if strings.TrimSpace(o.command) != "" {
			warnings = append(warnings, "reused existing peer tab; launch command not sent without --restart")
		} else {
			if err := ensureTurnLifecyclePane(app, paneID, tabPeerOptions{role: o.role, hostTab: o.hostTab, hostPane: o.hostPane, topic: o.topic, source: o.source, msg: o.msg}, time.Now()); err != nil {
				warnings = append(warnings, "peer metadata ensure failed: "+err.Error())
			}
		}
	}

	if o.readiness != "" {
		cond, err := waitfor.ParseCondition("output:" + o.readiness)
		if err != nil {
			return out, err
		}
		readiness, _ := waitfor.Wait(context.Background(), waitfor.Request{Runner: app.Runner, Target: out.Target, PaneID: out.PaneID, Lines: o.lines, Timeout: time.Duration(o.timeoutSec) * time.Second, Condition: cond})
		out.Readiness = &readiness
		if !readiness.Met {
			warnings = append(warnings, "readiness pattern not proven")
		}
	}
	if o.waitTurn != "" {
		cond, err := waitfor.ParseCondition("turn:" + o.waitTurn)
		if err != nil {
			return out, err
		}
		waited, _ := waitfor.Wait(context.Background(), waitfor.Request{Runner: app.Runner, Target: out.Target, PaneID: out.PaneID, Lines: o.lines, Timeout: time.Duration(o.timeoutSec) * time.Second, Condition: cond, Baseline: &baseline})
		out.Outcome = &waited
		if !waited.Met {
			warnings = append(warnings, "turn state not proven: "+emptyForHuman(waited.FailureKind, "unproven"))
		}
	}
	out.Status = waitfor.ReadStatus(app.Runner, out.PaneID)
	out.OutputTail, _ = app.Runner.CapturePane(out.Target, o.lines)
	warnings = append(warnings, waitfor.WarningsForStatus(out.Status)...)
	out.Warnings = uniqueStrings(warnings)
	if out.Outcome != nil && !out.Outcome.Met {
		return out, fmt.Errorf("peer ensure turn wait not met: %s", emptyForHuman(out.Outcome.FailureKind, "unproven"))
	}
	if out.Readiness != nil && !out.Readiness.Met {
		return out, fmt.Errorf("peer ensure readiness not met: %s", emptyForHuman(out.Readiness.FailureKind, "unproven"))
	}
	return out, nil
}

func createPeerCommandTab(app *apppkg.App, sessionName, name, command string, o peerEnsureOptions) (paneID, target string, err error) {
	dir, _ := os.Getwd()
	paneID, err = app.Runner.NewWindow(sessionName, name, dir, tmux.Detached())
	if err != nil {
		return "", "", fmt.Errorf("create peer tab: %w", err)
	}
	target = fmt.Sprintf("%s:%s", sessionName, name)
	if paneID != "" {
		target = paneID
	}
	if paneID != "" {
		if _, err := tabs.Stamp(app.Runner, paneID, paneID, name, tablabel.SourcePane); err != nil {
			return "", "", fmt.Errorf("stamp peer tab: %w", err)
		}
		if err := markPeerRunning(app, paneID, target, o); err != nil {
			return "", "", err
		}
		_ = tabs.TouchInput(app.Runner, paneID, time.Now())
	}
	if err := sendPreparedCommand(app, target, command, o.timeoutSec); err != nil {
		return "", "", err
	}
	return paneID, target, nil
}

func markPeerRunning(app *apppkg.App, paneID, target string, o peerEnsureOptions) error {
	svc := tabstate.New(app.Runner, os.Getenv)
	stateTarget := tabstate.Target{PaneID: paneID, Window: target}
	if resolved, err := svc.Resolve(target); err == nil {
		stateTarget = resolved
	}
	return runTabPeerAction(app, svc, stateTarget, "start", tabPeerOptions{role: o.role, hostTab: o.hostTab, hostPane: o.hostPane, topic: o.topic, source: o.source, msg: o.msg})
}

func sendPreparedCommand(app *apppkg.App, target, command string, timeoutSec int) error {
	sendCmd := command
	if !isSimpleCommand(command) {
		scriptPath, cleanup, err := writeCommandScript(command, timeoutSec)
		if err != nil {
			return fmt.Errorf("write command script: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}
		sendCmd = fmt.Sprintf("bash %s", scriptPath)
	}
	if err := sendShellLine(app.Runner, target, sendCmd); err != nil {
		return fmt.Errorf("send peer command to %s: %w", target, err)
	}
	return nil
}

func paneForResolvedTab(app *apppkg.App, rt resolvedTab) (string, error) {
	pane, _ := paneTargetForResolvedTab(app, rt)
	if pane == "" {
		return "", fmt.Errorf("could not resolve pane")
	}
	return pane, nil
}

func paneTargetForResolvedTab(app *apppkg.App, rt resolvedTab) (paneID, target string) {
	target = rt.Target
	if rt.Tab != nil {
		return rt.Tab.PaneID, target
	}
	svc := tabstate.New(app.Runner, os.Getenv)
	if st, err := rt.stateTarget(svc); err == nil {
		return st.PaneID, target
	}
	return target, target
}

func printPeerEnsure(out peerEnsureOutput) {
	action := "reused"
	if out.Created {
		action = "created"
	} else if out.Restarted {
		action = "restarted"
	}
	fmt.Printf("peer %s: %s\n", action, out.Tab)
	fmt.Printf("target: %s\n", out.Target)
	if out.PaneID != "" {
		fmt.Printf("pane: %s\n", out.PaneID)
	}
	if out.CommandSent {
		fmt.Println("command: sent")
	}
	if out.Readiness != nil {
		fmt.Printf("readiness: met=%t basis=%s\n", out.Readiness.Met, out.Readiness.Basis)
	}
	if out.Outcome != nil {
		fmt.Printf("turn-wait: met=%t state=%s basis=%s fresh=%t\n", out.Outcome.Met, out.Outcome.State, out.Outcome.Basis, out.Outcome.Fresh)
	}
	for _, warning := range out.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	if strings.TrimSpace(out.OutputTail) != "" {
		fmt.Println("output:")
		fmt.Print(out.OutputTail)
		if !strings.HasSuffix(out.OutputTail, "\n") {
			fmt.Println()
		}
	}
}
