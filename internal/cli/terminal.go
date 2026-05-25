package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/procfs"
	"github.com/donjor/zmux/internal/terminal"
	"github.com/donjor/zmux/internal/wm"
	"github.com/spf13/cobra"
)

type terminalCurrentFlags struct {
	json bool
}

type terminalCapabilitiesFlags struct {
	json bool
}

type terminalRefreshFlags struct {
	targetClient string
	session      string
}

type terminalCapabilitiesResult struct {
	SchemaVersion  string                     `json:"schemaVersion"`
	OK             bool                       `json:"ok"`
	Status         string                     `json:"status"`
	TmuxVersion    string                     `json:"tmuxVersion,omitempty"`
	InsideTmux     bool                       `json:"insideTmux"`
	InsideEnv      terminalInsideEnv          `json:"insideEnv"`
	CurrentTTY     string                     `json:"currentTTY,omitempty"`
	Clients        []terminalCapabilityClient `json:"clients"`
	Recommendation string                     `json:"recommendation,omitempty"`
}

type terminalInsideEnv struct {
	TERM        string `json:"TERM,omitempty"`
	COLORTERM   string `json:"COLORTERM,omitempty"`
	TERMProgram string `json:"TERM_PROGRAM,omitempty"`
}

type terminalCapabilityClient struct {
	TTY         string   `json:"tty"`
	Current     bool     `json:"current"`
	Focused     bool     `json:"focused"`
	TermName    string   `json:"termName"`
	Features    []string `json:"features"`
	RGB         bool     `json:"rgb"`
	Flags       []string `json:"flags,omitempty"`
	SessionName string   `json:"sessionName,omitempty"`
	WindowID    string   `json:"windowID,omitempty"`
	WindowName  string   `json:"windowName,omitempty"`
	PaneID      string   `json:"paneID,omitempty"`
}

func newTerminalCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Inspect the desktop terminal window for the current tmux client",
	}
	// Production desktop-window deps; tests inject fakes via newTerminalCurrentCmd.
	cmd.AddCommand(newTerminalCurrentCmd(app, wm.NewHyprlandAdapter(), procfs.LinuxInspector{}))
	cmd.AddCommand(newTerminalCapabilitiesCmd(app))
	cmd.AddCommand(newTerminalRefreshCmd(app))
	return cmd
}

func newTerminalCurrentCmd(app *apppkg.App, adapter wm.Adapter, process procfs.Inspector) *cobra.Command {
	flags := &terminalCurrentFlags{}
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Resolve the visible desktop terminal window for the current tmux client",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminalCurrent(app, cmd, flags, adapter, process)
		},
	}
	cmd.Flags().BoolVar(&flags.json, "json", false, "print terminal target data as JSON")
	return cmd
}

func newTerminalCapabilitiesCmd(app *apppkg.App) *cobra.Command {
	flags := &terminalCapabilitiesFlags{}
	cmd := &cobra.Command{
		Use:     "capabilities",
		Aliases: []string{"caps"},
		Short:   "Diagnose tmux outer-terminal color capabilities",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminalCapabilities(app, cmd, flags)
		},
	}
	cmd.Flags().BoolVar(&flags.json, "json", false, "print terminal capability data as JSON")
	return cmd
}

func newTerminalRefreshCmd(app *apppkg.App) *cobra.Command {
	flags := &terminalRefreshFlags{}
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Reattach the current tmux client to refresh terminal RGB features",
		Long:  "Replace the current attached tmux client with a freshly attached client using RGB terminal features. This avoids a manual detach/reattach after zmux changes tmux terminal-features.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTerminalRefresh(app, cmd, flags)
		},
	}
	cmd.Flags().StringVar(&flags.targetClient, "target-client", "", "target tmux client tty (defaults to current client)")
	cmd.Flags().StringVar(&flags.session, "session", "", "session to reattach (defaults to current client session)")
	return cmd
}

func runTerminalCurrent(app *apppkg.App, cmd *cobra.Command, flags *terminalCurrentFlags, adapter wm.Adapter, process procfs.Inspector) error {
	resolver := terminal.Resolver{
		Runner:        app.Runner,
		Adapter:       adapter,
		Process:       process,
		CurrentPaneID: os.Getenv("TMUX_PANE"),
	}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		return err
	}
	if flags.json {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	if result.OK && result.Target != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", result.Target.Geometry)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", result.Status, result.Reason)
	return nil
}

func runTerminalRefresh(app *apppkg.App, cmd *cobra.Command, flags *terminalRefreshFlags) error {
	targetClient := strings.TrimSpace(flags.targetClient)
	session := strings.TrimSpace(flags.session)
	if targetClient == "" || session == "" {
		if !app.Runner.IsInsideTmux() {
			return fmt.Errorf("terminal refresh must run inside tmux unless --target-client and --session are provided")
		}
		out, err := app.Runner.DisplayMessage("", "#{client_tty}\t#{client_session}")
		if err != nil {
			return fmt.Errorf("resolve current tmux client: %w", err)
		}
		fields := strings.SplitN(strings.TrimRight(out, "\r\n"), "\t", 2)
		if len(fields) != 2 {
			return fmt.Errorf("resolve current tmux client: expected tty and session, got %q", out)
		}
		if targetClient == "" {
			targetClient = strings.TrimSpace(fields[0])
		}
		if session == "" {
			session = strings.TrimSpace(fields[1])
		}
	}
	if targetClient == "" {
		return fmt.Errorf("target client is required")
	}
	if session == "" {
		return fmt.Errorf("session is required")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "refreshing tmux client %s -> %s\n", targetClient, session)
	return app.Runner.RefreshClient(targetClient, session)
}

func runTerminalCapabilities(app *apppkg.App, cmd *cobra.Command, flags *terminalCapabilitiesFlags) error {
	result := terminalCapabilitiesResult{
		SchemaVersion: "zmux-terminal-capabilities/v1",
		InsideTmux:    app.Runner.IsInsideTmux(),
		InsideEnv: terminalInsideEnv{
			TERM:        os.Getenv("TERM"),
			COLORTERM:   os.Getenv("COLORTERM"),
			TERMProgram: os.Getenv("TERM_PROGRAM"),
		},
	}
	if version, err := app.Runner.Version(); err == nil {
		result.TmuxVersion = version
	}
	if result.InsideTmux {
		if tty, err := app.Runner.DisplayMessage("", "#{client_tty}"); err == nil {
			result.CurrentTTY = tty
		}
	}
	clients, err := app.Runner.ListClients()
	if err != nil {
		return fmt.Errorf("list tmux clients: %w", err)
	}
	for _, client := range clients {
		features := splitCommaList(client.TermFeatures)
		flagsList := splitCommaList(client.Flags)
		capClient := terminalCapabilityClient{
			TTY:         client.TTY,
			Current:     result.CurrentTTY != "" && client.TTY == result.CurrentTTY,
			Focused:     containsToken(flagsList, "focused"),
			TermName:    client.TermName,
			Features:    features,
			RGB:         containsToken(features, "RGB") || containsToken(features, "Tc"),
			Flags:       flagsList,
			SessionName: client.SessionName,
			WindowID:    client.WindowID,
			WindowName:  client.WindowName,
			PaneID:      client.PaneID,
		}
		result.Clients = append(result.Clients, capClient)
	}
	result.OK, result.Status = terminalCapabilitiesStatus(result)
	if !result.OK {
		result.Recommendation = "run `zmux refresh` so zmux applies config and tmux re-resolves terminal-features; expected generated config includes xterm-256color:RGB:extkeys and xterm-ghostty:RGB:extkeys"
	}
	if flags.json {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	renderTerminalCapabilities(cmd, result)
	return nil
}

func terminalCapabilitiesStatus(result terminalCapabilitiesResult) (bool, string) {
	if len(result.Clients) == 0 {
		return false, "no_clients"
	}
	if result.CurrentTTY != "" {
		for _, client := range result.Clients {
			if client.Current {
				if client.RGB {
					return true, "ok"
				}
				return false, "rgb_missing_current_client"
			}
		}
		return false, "current_client_not_found"
	}
	for _, client := range result.Clients {
		if client.RGB {
			return true, "ok"
		}
	}
	return false, "rgb_missing_all_clients"
}

func renderTerminalCapabilities(cmd *cobra.Command, result terminalCapabilitiesResult) {
	out := cmd.OutOrStdout()
	status := "✗"
	if result.OK {
		status = "✓"
	}
	fmt.Fprintf(out, "%s tmux truecolor: %s\n", status, result.Status)
	if result.TmuxVersion != "" {
		fmt.Fprintf(out, "tmux: %s\n", result.TmuxVersion)
	}
	fmt.Fprintf(out, "inside: TERM=%s COLORTERM=%s TERM_PROGRAM=%s\n", result.InsideEnv.TERM, result.InsideEnv.COLORTERM, result.InsideEnv.TERMProgram)
	for _, client := range result.Clients {
		marker := " "
		if client.Current {
			marker = "*"
		}
		rgb := "missing"
		if client.RGB {
			rgb = "RGB"
		}
		features := strings.Join(client.Features, ",")
		if features == "" {
			features = "(none)"
		}
		fmt.Fprintf(out, "%s client %s term=%s truecolor=%s features=%s\n", marker, client.TTY, client.TermName, rgb, features)
	}
	if result.Recommendation != "" {
		fmt.Fprintf(out, "recommendation: %s\n", result.Recommendation)
	}
}

func splitCommaList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsToken(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
