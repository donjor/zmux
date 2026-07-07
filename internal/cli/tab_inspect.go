package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/waitfor"
	"github.com/spf13/cobra"
)

type tabInspectOutput struct {
	Tab        string          `json:"tab"`
	Session    string          `json:"session,omitempty"`
	Target     string          `json:"target"`
	PaneID     string          `json:"paneId,omitempty"`
	Status     tabStatusOutput `json:"status"`
	OutputTail string          `json:"outputTail,omitempty"`
	Warnings   []string        `json:"warnings,omitempty"`
}

func newTabInspectCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var lines int
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "inspect <tab>",
		Short: "Inspect lifecycle status plus output for a tab",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveWaitTarget(app, args[0], sessionFlag, false)
			if err != nil {
				return err
			}
			rt, err := resolveTabTarget(app, target.Session, args[0])
			if err != nil {
				return err
			}
			status, err := buildTabStatus(app, args[0], target.Target, rt)
			if err != nil {
				return err
			}
			output, err := app.Runner.CapturePane(target.Target, lines)
			warnings := waitfor.WarningsForStatus(waitfor.ReadStatus(app.Runner, target.PaneID))
			if err != nil {
				warnings = append(warnings, "output capture unavailable: "+err.Error())
			}
			result := tabInspectOutput{
				Tab:        status.Tab,
				Session:    target.Session,
				Target:     target.Target,
				PaneID:     target.PaneID,
				Status:     status,
				OutputTail: output,
				Warnings:   uniqueStrings(warnings),
			}
			if jsonOut {
				b, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			printTabInspect(result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session")
	cmd.Flags().IntVarP(&lines, "lines", "l", 120, "output lines to capture")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print JSON")
	return cmd
}

func printTabInspect(out tabInspectOutput) {
	fmt.Printf("tab: %s\n", out.Tab)
	if out.Session != "" {
		fmt.Printf("session: %s\n", out.Session)
	}
	fmt.Printf("target: %s\n", out.Target)
	if out.PaneID != "" {
		fmt.Printf("pane: %s\n", out.PaneID)
	}
	if out.Status.ResolvedState != "" {
		fmt.Printf("resolved-state: %s\n", out.Status.ResolvedState)
	}
	if out.Status.CmdState != "" {
		fmt.Printf("command-state: %s\n", out.Status.CmdState)
	}
	if out.Status.TurnState != "" {
		line := "turn-state: " + out.Status.TurnState
		if out.Status.TurnSeq != "" {
			line += " (seq " + out.Status.TurnSeq + ")"
		}
		fmt.Println(line)
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

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
