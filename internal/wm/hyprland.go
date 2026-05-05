package wm

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// CommandRunner executes a command and returns stdout.
type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// HyprlandAdapter reads Hyprland client and monitor state via hyprctl.
type HyprlandAdapter struct {
	Hyprctl string
	Runner  CommandRunner
}

func NewHyprlandAdapter() *HyprlandAdapter {
	return &HyprlandAdapter{Hyprctl: "hyprctl", Runner: execRunner{}}
}

func (a *HyprlandAdapter) Windows(ctx context.Context) ([]Window, error) {
	hyprctl := a.Hyprctl
	if hyprctl == "" {
		hyprctl = "hyprctl"
	}
	runner := a.Runner
	if runner == nil {
		runner = execRunner{}
	}
	clients, err := runner.Output(ctx, hyprctl, "clients", "-j")
	if err != nil {
		if isExecNotFound(err) {
			return nil, ErrUnsupported
		}
		return nil, fmt.Errorf("hyprctl clients: %w", err)
	}
	monitors, err := runner.Output(ctx, hyprctl, "monitors", "-j")
	if err != nil {
		if isExecNotFound(err) {
			return nil, ErrUnsupported
		}
		return nil, fmt.Errorf("hyprctl monitors: %w", err)
	}
	return parseHyprlandWindows(clients, monitors)
}

func isExecNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
		return true
	}
	return false
}

type hyprClient struct {
	Address   string `json:"address"`
	Class     string `json:"class"`
	Title     string `json:"title"`
	PID       int    `json:"pid"`
	Workspace struct {
		Name string `json:"name"`
	} `json:"workspace"`
	At     []int `json:"at"`
	Size   []int `json:"size"`
	Mapped bool  `json:"mapped"`
	Hidden bool  `json:"hidden"`
}

type hyprMonitor struct {
	ActiveWorkspace struct {
		Name string `json:"name"`
	} `json:"activeWorkspace"`
}

func parseHyprlandWindows(clientsJSON, monitorsJSON []byte) ([]Window, error) {
	var clients []hyprClient
	if err := json.Unmarshal(clientsJSON, &clients); err != nil {
		return nil, fmt.Errorf("parse hyprland clients: %w", err)
	}
	var monitors []hyprMonitor
	if err := json.Unmarshal(monitorsJSON, &monitors); err != nil {
		return nil, fmt.Errorf("parse hyprland monitors: %w", err)
	}
	activeWorkspaces := make(map[string]bool)
	for _, m := range monitors {
		if m.ActiveWorkspace.Name != "" {
			activeWorkspaces[m.ActiveWorkspace.Name] = true
		}
	}
	windows := make([]Window, 0, len(clients))
	for _, c := range clients {
		geometry := ""
		if len(c.At) >= 2 && len(c.Size) >= 2 {
			geometry = fmt.Sprintf("%d,%d %dx%d", c.At[0], c.At[1], c.Size[0], c.Size[1])
		}
		onActiveMonitor := activeWorkspaces[c.Workspace.Name]
		windows = append(windows, Window{
			WM:              "hyprland",
			Address:         c.Address,
			Class:           c.Class,
			Title:           c.Title,
			PID:             c.PID,
			Workspace:       c.Workspace.Name,
			Geometry:        geometry,
			Visible:         c.Mapped && !c.Hidden && onActiveMonitor,
			Mapped:          c.Mapped,
			Hidden:          c.Hidden,
			OnActiveMonitor: onActiveMonitor,
		})
	}
	return windows, nil
}
