package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabs"
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
			sessionName := sessionFlag
			if sessionName == "" {
				if app.Runner.IsInsideTmux() {
					out, err := app.Runner.DisplayMessage("", "#{session_name}")
					if err != nil {
						return fmt.Errorf("could not get current session")
					}
					sessionName = strings.TrimSpace(out)
				} else {
					return fmt.Errorf("specify a session: zmux tab status <tab> -s <session>")
				}
			}
			sessionName, err := resolveSessionTarget(app, sessionName)
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
	status.State, _ = app.Runner.ShowPaneOption(pane, "@zmux_state")
	status.Message, _ = app.Runner.ShowPaneOption(pane, "@zmux_state_msg")
	status.Source, _ = app.Runner.ShowPaneOption(pane, "@zmux_state_source")
	status.Scope, _ = app.Runner.ShowPaneOption(pane, tabs.OptScope)
	status.Origin, _ = app.Runner.ShowPaneOption(pane, tabs.OptOrigin)
	status.CmdState, _ = app.Runner.ShowPaneOption(pane, tabs.OptCmdState)
	status.CmdSeq, _ = app.Runner.ShowPaneOption(pane, tabs.OptCmdSeq)
	status.LastExit, _ = app.Runner.ShowPaneOption(pane, tabs.OptCmdLastExit)
	status.RunID, _ = app.Runner.ShowPaneOption(pane, tabs.OptCmdRunID)
	status.TurnState, _ = app.Runner.ShowPaneOption(pane, tabs.OptTurnState)
	status.TurnState = tabs.NormalizeTurnState(status.TurnState)
	status.TurnAt, _ = app.Runner.ShowPaneOption(pane, tabs.OptTurnAt)
	status.TurnSeq, _ = app.Runner.ShowPaneOption(pane, tabs.OptTurnSeq)
	if status.TurnSeq == "" {
		status.TurnSeq, _ = app.Runner.ShowPaneOption(pane, tabs.OptPeerTurns)
	}
	status.PeerRole, _ = app.Runner.ShowPaneOption(pane, tabs.OptPeerRole)
	status.PeerTopic, _ = app.Runner.ShowPaneOption(pane, tabs.OptPeerTopic)
	if cmd, _ := app.Runner.ShowPaneOption(pane, tabs.OptCmdText); cmd != "" {
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
