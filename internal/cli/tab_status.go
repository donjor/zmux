package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/spf13/cobra"
)

type tabStatusOutput struct {
	Tab            string `json:"tab"`
	Target         string `json:"target"`
	Session        string `json:"session,omitempty"`
	PaneID         string `json:"paneId,omitempty"`
	State          string `json:"state,omitempty"`
	Message        string `json:"message,omitempty"`
	Source         string `json:"source,omitempty"`
	ResolvedState  string `json:"resolvedState,omitempty"`
	ResolvedSource string `json:"resolvedSource,omitempty"`
	StateReason    string `json:"stateReason,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Origin         string `json:"origin,omitempty"`
	Command        string `json:"command,omitempty"`
	CmdState       string `json:"cmdState,omitempty"`
	CmdSeq         string `json:"cmdSeq,omitempty"`
	LastExit       string `json:"lastExit,omitempty"`
	RunID          string `json:"runId,omitempty"`
	TurnState      string `json:"turnState,omitempty"`
	TurnAt         string `json:"turnAt,omitempty"`
	TurnSeq        string `json:"turnSeq,omitempty"`
	PeerRole       string `json:"peerRole,omitempty"`
	PeerTopic      string `json:"peerTopic,omitempty"`
}

func newTabStatusCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status <tab>",
		Short: "Show tab lifecycle and command status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionFlag == "" && !app.Runner.IsInsideTmux() {
				return fmt.Errorf("specify a session: zmux tab status <tab> -s <session>")
			}
			sessionName, err := resolveSessionTarget(app, sessionFlag)
			if err != nil {
				return err
			}

			rt, err := resolveTabTarget(app, sessionName, args[0])
			if err != nil {
				return err
			}
			if !rt.found() {
				return fmt.Errorf("no tab %q in session %q", args[0], sessionName)
			}
			target := rt.Target
			if target == "" {
				target = fmt.Sprintf("%s:%s", sessionName, args[0])
			}
			status, err := buildTabStatus(app, args[0], target, rt)
			if err != nil {
				return err
			}
			if jsonOut {
				b, err := json.MarshalIndent(status, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			printTabStatus(status)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
	return cmd
}

func buildTabStatus(app *apppkg.App, label, target string, rt resolvedTab) (tabStatusOutput, error) {
	pane := target
	status := tabStatusOutput{Tab: label, Target: target}
	if rt.Tab != nil {
		status.Tab = tabs.DisplayName(rt.Tab)
		status.Session = rt.Tab.Session
		status.PaneID = rt.Tab.PaneID
		status.Command = rt.Tab.Command
		pane = rt.Tab.PaneID
	} else if rt.Win != nil {
		status.Tab = rt.Win.Name
	}
	if pane == "" {
		return status, nil
	}
	opts, _ := app.Runner.ShowPaneOptions(pane, []string{
		tabstate.OptState, tabstate.OptMsg, tabstate.OptSource,
		tabs.OptScope, tabs.OptOrigin,
		tabs.OptCmdState, tabs.OptCmdSeq, tabs.OptCmdLastExit, tabs.OptCmdRunID,
		tabs.OptTurnState, tabs.OptTurnAt, tabs.OptTurnSeq, tabs.OptPeerTurns,
		tabs.OptPeerRole, tabs.OptPeerTopic, tabs.OptCmdText,
	})
	status.State = opts[tabstate.OptState]
	status.Message = opts[tabstate.OptMsg]
	status.Source = opts[tabstate.OptSource]
	status.Scope = opts[tabs.OptScope]
	status.Origin = opts[tabs.OptOrigin]
	status.CmdState = opts[tabs.OptCmdState]
	status.CmdSeq = opts[tabs.OptCmdSeq]
	status.LastExit = opts[tabs.OptCmdLastExit]
	status.RunID = opts[tabs.OptCmdRunID]
	status.TurnState = tabs.NormalizeTurnState(opts[tabs.OptTurnState])
	status.TurnAt = opts[tabs.OptTurnAt]
	status.TurnSeq = opts[tabs.OptTurnSeq]
	if status.TurnSeq == "" {
		status.TurnSeq = opts[tabs.OptPeerTurns]
	}
	status.PeerRole = opts[tabs.OptPeerRole]
	status.PeerTopic = opts[tabs.OptPeerTopic]
	if cmd := opts[tabs.OptCmdText]; cmd != "" {
		status.Command = cmd
	}
	res := tabs.ResolveDisplayState(tabs.DisplaySignals{
		ManualState:        status.State,
		ManualSource:       status.Source,
		CommandState:       status.CmdState,
		CommandSource:      "shell",
		CommandInteractive: isInteractiveVenueStatus(status.Scope, status.Command),
		CommandExit:        status.LastExit,
		TurnState:          status.TurnState,
		TurnSource:         status.Source,
	})
	if res.Set {
		status.ResolvedState = string(res.State)
		status.ResolvedSource = res.Source
	}
	status.StateReason = res.Reason
	return status, nil
}

func isInteractiveVenueStatus(scope, command string) bool {
	switch strings.TrimSpace(scope) {
	case tabs.ScopeAgentShell, tabs.ScopePeer, tabs.ScopeWorker, tabs.ScopeDaemon:
		return true
	default:
		return isInteractiveVenueCommand(command)
	}
}

func printTabStatus(status tabStatusOutput) {
	fmt.Printf("tab: %s\n", status.Tab)
	fmt.Printf("target: %s\n", status.Target)
	if status.Session != "" {
		fmt.Printf("session: %s\n", status.Session)
	}
	if status.PaneID != "" {
		fmt.Printf("pane: %s\n", status.PaneID)
	}
	if status.State != "" {
		line := "state: " + status.State
		if status.Message != "" {
			line += " (" + status.Message + ")"
		}
		fmt.Println(line)
	}
	if status.CmdState != "" {
		line := "command-state: " + status.CmdState
		var details []string
		if status.CmdSeq != "" {
			details = append(details, "seq "+status.CmdSeq)
		}
		if status.LastExit != "" {
			details = append(details, "exit "+status.LastExit)
		}
		if len(details) > 0 {
			line += " (" + strings.Join(details, ", ") + ")"
		}
		fmt.Println(line)
	}
	if status.Command != "" {
		fmt.Printf("command: %s\n", status.Command)
	}
	if status.Scope != "" {
		fmt.Printf("scope: %s\n", status.Scope)
	}
	if status.TurnState != "" {
		line := "turn-state: " + status.TurnState
		var details []string
		if status.TurnAt != "" {
			details = append(details, "at "+status.TurnAt)
		}
		if status.TurnSeq != "" {
			details = append(details, "seq "+status.TurnSeq)
		}
		if len(details) > 0 {
			line += " (" + strings.Join(details, ", ") + ")"
		}
		fmt.Println(line)
	}
}
