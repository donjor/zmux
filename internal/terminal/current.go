// Package terminal resolves strict screenshot targets for the current tmux client.
package terminal

import (
	"context"
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/procfs"
	"github.com/donjor/zmux/internal/termtitle"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/wm"
)

const SchemaVersion = "zmux-terminal-current/v1"

type Status string

const (
	StatusOK          Status = "ok"
	StatusNotInTmux   Status = "not_in_tmux"
	StatusUnsupported Status = "unsupported"
	StatusNotFound    Status = "not_found"
	StatusAmbiguous   Status = "ambiguous"
	StatusHidden      Status = "hidden"
)

type Confidence string

const ConfidenceHigh Confidence = "high"

// Result is the stable machine-readable response for `zmux terminal current --json`.
type Result struct {
	SchemaVersion string         `json:"schemaVersion"`
	OK            bool           `json:"ok"`
	Status        Status         `json:"status"`
	Reason        string         `json:"reason"`
	Target        *Target        `json:"target,omitempty"`
	Terminal      *Terminal      `json:"terminal,omitempty"`
	Tmux          *TmuxContext   `json:"tmux,omitempty"`
	Candidates    []CandidateRef `json:"candidates,omitempty"`
}

type Target struct {
	WM            string     `json:"wm"`
	WindowAddress string     `json:"windowAddress"`
	Geometry      string     `json:"geometry,omitempty"`
	Workspace     string     `json:"workspace"`
	Visible       bool       `json:"visible"`
	Confidence    Confidence `json:"confidence"`
}

type Terminal struct {
	Class string `json:"class"`
	Title string `json:"title"`
	PID   int    `json:"pid,omitempty"`
}

type TmuxContext struct {
	ClientTty     string `json:"clientTty"`
	ClientSession string `json:"clientSession"`
	SessionID     string `json:"sessionId"`
	WindowID      string `json:"windowId"`
	WindowIndex   int    `json:"windowIndex"`
	WindowName    string `json:"windowName"`
	PaneID        string `json:"paneId"`
}

type CandidateRef struct {
	WindowAddress string `json:"windowAddress"`
	Workspace     string `json:"workspace"`
	Visible       bool   `json:"visible"`
}

type Resolver struct {
	Runner        tmux.Runner
	Adapter       wm.Adapter
	Process       procfs.Inspector
	CurrentPaneID string
}

func (r Resolver) Resolve(ctx context.Context) (Result, error) {
	result := Result{SchemaVersion: SchemaVersion}
	if r.Runner == nil {
		return result.with(StatusUnsupported, "tmux runner is not configured"), nil
	}
	if !r.Runner.IsInsideTmux() || r.CurrentPaneID == "" {
		return result.with(StatusNotInTmux, "terminal current requires an invoking tmux pane"), nil
	}

	facts, err := currentFacts(r.Runner)
	if err != nil {
		return result.with(StatusUnsupported, fmt.Sprintf("tmux current context unavailable: %v", err)), nil
	}
	if reason := facts.unsupportedReason(); reason != "" {
		return result.with(StatusUnsupported, reason), nil
	}
	if facts.PaneID != r.CurrentPaneID {
		return result.with(StatusUnsupported, "TMUX_PANE does not match live tmux current pane"), nil
	}
	result.Tmux = &TmuxContext{
		ClientTty:     facts.ClientTty,
		ClientSession: facts.ClientSession,
		SessionID:     facts.SessionID,
		WindowID:      facts.WindowID,
		WindowIndex:   facts.WindowIndex,
		WindowName:    facts.WindowName,
		PaneID:        facts.PaneID,
	}

	clients, err := r.Runner.ListClients()
	if err != nil {
		return result.with(StatusUnsupported, fmt.Sprintf("tmux clients unavailable: %v", err)), nil
	}
	client, ok := findLiveClient(clients, facts)
	if !ok {
		return result.with(StatusUnsupported, "current tmux client was not found in live client list"), nil
	}
	process := r.Process
	if process == nil {
		process = procfs.LinuxInspector{}
	}

	if r.Adapter == nil {
		return result.with(StatusUnsupported, "window-manager adapter is not configured"), nil
	}
	windows, err := r.Adapter.Windows(ctx)
	if err != nil {
		return result.with(StatusUnsupported, fmt.Sprintf("window-manager metadata unavailable: %v", err)), nil
	}

	var matches []wm.Window
	for _, win := range windows {
		meta, err := termtitle.Parse(win.Title)
		if err != nil {
			continue
		}
		if !meta.Matches(facts.ClientTty, facts.SessionID, facts.WindowID, facts.PaneID) {
			continue
		}
		if win.PID <= 0 || client.PID <= 0 {
			return result.with(StatusUnsupported, "window-manager or tmux client pid is unavailable for strict validation"), nil
		}
		ancestor, err := process.IsAncestor(win.PID, client.PID)
		if err != nil {
			return result.with(StatusUnsupported, fmt.Sprintf("process ancestry validation unavailable: %v", err)), nil
		}
		if ancestor {
			matches = append(matches, win)
		}
	}
	if len(matches) == 0 {
		return result.with(StatusNotFound, "no visible terminal window exposes matching zmux:v1 title metadata"), nil
	}

	visible := make([]wm.Window, 0, len(matches))
	for _, win := range matches {
		result.Candidates = append(result.Candidates, CandidateRef{WindowAddress: win.Address, Workspace: win.Workspace, Visible: win.Visible})
		if win.Visible {
			visible = append(visible, win)
		}
	}
	if len(visible) == 0 {
		win := matches[0]
		result.Terminal = &Terminal{Class: win.Class, Title: win.Title, PID: win.PID}
		result.Target = &Target{WM: win.WM, WindowAddress: win.Address, Workspace: win.Workspace, Visible: false, Confidence: ConfidenceHigh}
		return result.with(StatusHidden, "matched terminal window is not visible on an active monitor; refusing screenshot geometry"), nil
	}
	if len(visible) > 1 {
		return result.with(StatusAmbiguous, "multiple visible terminal windows expose matching zmux:v1 title metadata"), nil
	}

	win := visible[0]
	result.Terminal = &Terminal{Class: win.Class, Title: win.Title, PID: win.PID}
	result.Target = &Target{WM: win.WM, WindowAddress: win.Address, Geometry: win.Geometry, Workspace: win.Workspace, Visible: true, Confidence: ConfidenceHigh}
	return result.with(StatusOK, "matched visible terminal window with validated zmux:v1 title metadata"), nil
}

func (r Result) with(status Status, reason string) Result {
	r.Status = status
	r.OK = status == StatusOK
	r.Reason = reason
	return r
}

type tmuxFacts struct {
	ClientTty     string
	ClientSession string
	SessionID     string
	WindowID      string
	WindowIndex   int
	WindowName    string
	PaneID        string
}

const currentFormat = "#{client_tty}\t#{client_session}\t#{session_id}\t#{window_id}\t#{window_index}\t#{window_name}\t#{pane_id}"

func currentFacts(runner tmux.Runner) (tmuxFacts, error) {
	out, err := runner.DisplayMessage("", currentFormat)
	if err != nil {
		return tmuxFacts{}, err
	}
	fields := strings.Split(strings.TrimRight(out, "\r\n"), "\t")
	if len(fields) < 7 {
		if len(fields) >= 5 {
			// tmux client_* fields are empty for detached/headless panes, and the
			// shared runner trims leading whitespace from command output. Treat the
			// remaining fields as enough context to refuse precisely rather than as
			// a parser error.
			var index int
			if _, err := fmt.Sscanf(fields[2], "%d", &index); err != nil {
				return tmuxFacts{}, fmt.Errorf("invalid window index %q", fields[2])
			}
			return tmuxFacts{
				SessionID:   fields[0],
				WindowID:    fields[1],
				WindowIndex: index,
				WindowName:  strings.Join(fields[3:len(fields)-1], "\t"),
				PaneID:      fields[len(fields)-1],
			}, nil
		}
		return tmuxFacts{}, fmt.Errorf("expected 7 current tmux fields, got %d", len(fields))
	}
	var index int
	if _, err := fmt.Sscanf(fields[4], "%d", &index); err != nil {
		return tmuxFacts{}, fmt.Errorf("invalid window index %q", fields[4])
	}
	return tmuxFacts{
		ClientTty:     fields[0],
		ClientSession: fields[1],
		SessionID:     fields[2],
		WindowID:      fields[3],
		WindowIndex:   index,
		WindowName:    strings.Join(fields[5:len(fields)-1], "\t"),
		PaneID:        fields[len(fields)-1],
	}, nil
}

func (f tmuxFacts) unsupportedReason() string {
	if f.ClientTty == "" || f.ClientSession == "" {
		return "tmux current client metadata is unavailable; terminal current requires an attached tmux client"
	}
	values := []string{f.ClientTty, f.ClientSession, f.SessionID, f.WindowID, f.PaneID}
	for _, value := range values {
		if value == "" || strings.Contains(value, "#{") {
			return "tmux does not expose required terminal metadata formats"
		}
	}
	return ""
}

func findLiveClient(clients []tmux.ClientInfo, facts tmuxFacts) (tmux.ClientInfo, bool) {
	for _, c := range clients {
		if c.ControlMode {
			continue
		}
		if c.TTY == facts.ClientTty && c.SessionName == facts.ClientSession && c.SessionID == facts.SessionID && c.WindowID == facts.WindowID && c.PaneID == facts.PaneID {
			return c, true
		}
	}
	return tmux.ClientInfo{}, false
}
