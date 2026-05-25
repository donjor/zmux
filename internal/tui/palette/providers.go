package palette

import (
	"fmt"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// ── Payload types ──

// SessionSwitchPayload is the payload for "switch to session" actions.
type SessionSwitchPayload struct {
	Name string
}

// SessionCreatePayload is the payload for "create session" actions.
type SessionCreatePayload struct{}

// SessionKillPayload is the payload for "kill session" actions.
type SessionKillPayload struct {
	Name string
}

// ThemeSetPayload is the payload for "set theme" actions.
type ThemeSetPayload struct {
	Name string
}

// BarSetPayload is the payload for "set bar preset" actions.
type BarSetPayload struct {
	Preset string
}

// DashboardTabPayload is the payload for "open dashboard tab" actions.
type DashboardTabPayload struct {
	Tab string // "current", "sessions", "settings", "help"
}

// ── Sessions Provider ──

// SessionsProvider generates actions for session switching and management.
type SessionsProvider struct {
	Runner tmux.Runner
}

func (p *SessionsProvider) Actions() ([]Action, error) {
	sessions, err := session.ListSessions(p.Runner)
	if err != nil {
		return nil, err
	}

	var actions []Action

	// "New session" action.
	actions = append(actions, Action{
		ID:       "session:new",
		Group:    "Sessions",
		Title:    "New session",
		Hint:     "n",
		Keywords: []string{"create", "tmp"},
		Kind:     ActionExec,
		Payload:  SessionCreatePayload{},
	})

	// "Switch to <name>" for each session.
	for _, s := range sessions {
		subtitle := ""
		if s.Attached {
			subtitle = "attached"
		}
		actions = append(actions, Action{
			ID:       fmt.Sprintf("session:switch:%s", s.Name),
			Group:    "Sessions",
			Title:    fmt.Sprintf("Switch to %s", s.Name),
			Subtitle: subtitle,
			Keywords: []string{"session", "attach", s.Name},
			Kind:     ActionExec,
			Payload:  SessionSwitchPayload{Name: s.Name},
		})
	}

	// "Kill <name>" for each session.
	for _, s := range sessions {
		actions = append(actions, Action{
			ID:       fmt.Sprintf("session:kill:%s", s.Name),
			Group:    "Sessions",
			Title:    fmt.Sprintf("Kill %s", s.Name),
			Keywords: []string{"session", "delete", "remove", s.Name},
			Kind:     ActionExec,
			Payload:  SessionKillPayload{Name: s.Name},
		})
	}

	return actions, nil
}

// ── Themes Provider ──

// ThemesProvider generates "Set theme: <name>" actions for each available theme.
type ThemesProvider struct {
	Resolver *theme.Resolver
}

func (p *ThemesProvider) Actions() ([]Action, error) {
	if p.Resolver == nil {
		return nil, nil
	}
	themes := p.Resolver.List()

	actions := make([]Action, 0, len(themes))
	for _, ti := range themes {
		darkLight := "dark"
		if !ti.IsDark {
			darkLight = "light"
		}
		actions = append(actions, Action{
			ID:       fmt.Sprintf("theme:set:%s", ti.Name),
			Group:    "Themes",
			Title:    fmt.Sprintf("Set theme: %s", ti.Name),
			Subtitle: darkLight,
			Keywords: []string{"theme", "color", "colour", ti.Name, string(ti.Source)},
			Kind:     ActionExec,
			Payload:  ThemeSetPayload{Name: ti.Name},
		})
	}
	return actions, nil
}

// ── Bar Provider ──

// BarProvider generates "Set bar: <preset>" actions for each bar preset.
type BarProvider struct{}

func (p *BarProvider) Actions() ([]Action, error) {
	presets := bar.AllPresets()
	actions := make([]Action, 0, len(presets))
	for _, preset := range presets {
		name := preset.String()
		actions = append(actions, Action{
			ID:       fmt.Sprintf("bar:set:%s", name),
			Group:    "Bar",
			Title:    fmt.Sprintf("Set bar: %s", name),
			Keywords: []string{"bar", "status", "preset", name},
			Kind:     ActionExec,
			Payload:  BarSetPayload{Preset: name},
		})
	}
	return actions, nil
}

// ── Dashboard Provider ──

// DashboardProvider generates actions to open dashboard tabs.
type DashboardProvider struct{}

func (p *DashboardProvider) Actions() ([]Action, error) {
	return []Action{
		{
			ID:       "dashboard:current",
			Group:    "Dashboard",
			Title:    "Open This Session tab",
			Keywords: []string{"tab", "current", "session", "windows"},
			Kind:     ActionOpenDashboard,
			Payload:  DashboardTabPayload{Tab: "current"},
		},
		{
			ID:       "dashboard:sessions",
			Group:    "Dashboard",
			Title:    "Open Sessions tab",
			Keywords: []string{"tab", "sessions", "list"},
			Kind:     ActionOpenDashboard,
			Payload:  DashboardTabPayload{Tab: "sessions"},
		},
		{
			ID:       "dashboard:settings",
			Group:    "Dashboard",
			Title:    "Open Settings tab",
			Keywords: []string{"tab", "settings", "themes", "config", "preferences"},
			Kind:     ActionOpenDashboard,
			Payload:  DashboardTabPayload{Tab: "settings"},
		},
		{
			ID:       "dashboard:help",
			Group:    "Dashboard",
			Title:    "Open Help tab",
			Keywords: []string{"tab", "help", "keys", "bindings"},
			Kind:     ActionOpenDashboard,
			Payload:  DashboardTabPayload{Tab: "help"},
		},
	}, nil
}

// ── Help Provider ──

// HelpProvider generates a single "Show keybindings" action.
type HelpProvider struct{}

func (p *HelpProvider) Actions() ([]Action, error) {
	return []Action{
		{
			ID:       "help:keybindings",
			Group:    "Help",
			Title:    "Show keybindings",
			Keywords: []string{"help", "keys", "bindings", "shortcuts"},
			Kind:     ActionOpenDashboard,
			Payload:  DashboardTabPayload{Tab: "help"},
		},
	}, nil
}

// ── Overmind Provider ──

// OvermindConnectPayload is the payload for overmind connect actions.
type OvermindConnectPayload struct {
	ControlSocket string
	Process       string
}

// OvermindRestartPayload is the payload for overmind restart actions.
type OvermindRestartPayload struct {
	ControlSocket string
	Process       string
}

// OvermindStopPayload is the payload for overmind stop actions.
type OvermindStopPayload struct {
	ControlSocket string
	Process       string
}

// OvermindProvider generates actions for overmind process management.
// It runs discovery each time the palette opens.
type OvermindProvider struct {
	// Local is the active profile's server endpoint, so discovery treats the
	// invoking binary's own server as local (default for zmux, -L zzmux for zzmux).
	Local tmux.Endpoint
}

func (p *OvermindProvider) Actions() ([]Action, error) {
	cat, err := source.Discover(p.Local)
	if err != nil || cat == nil {
		return nil, nil
	}

	var actions []Action
	for _, g := range cat.External {
		if g.Source.Kind != source.SourceOvermind || g.Source.Overmind == nil {
			continue
		}
		cs := g.Source.Overmind.ControlSocket
		for _, entry := range g.Entries {
			proc := entry.Session
			label := g.Source.Label

			// Connect.
			actions = append(actions, Action{
				ID:       fmt.Sprintf("overmind:connect:%s:%s", label, proc),
				Group:    "Overmind",
				Title:    fmt.Sprintf("Connect to %s", proc),
				Subtitle: label,
				Keywords: []string{"overmind", "connect", proc, label},
				Kind:     ActionExec,
				Payload:  OvermindConnectPayload{ControlSocket: cs, Process: proc},
			})

			// Restart.
			actions = append(actions, Action{
				ID:       fmt.Sprintf("overmind:restart:%s:%s", label, proc),
				Group:    "Overmind",
				Title:    fmt.Sprintf("Restart %s", proc),
				Subtitle: label,
				Keywords: []string{"overmind", "restart", proc, label},
				Kind:     ActionExec,
				Payload:  OvermindRestartPayload{ControlSocket: cs, Process: proc},
			})

			// Stop.
			actions = append(actions, Action{
				ID:       fmt.Sprintf("overmind:stop:%s:%s", label, proc),
				Group:    "Overmind",
				Title:    fmt.Sprintf("Stop %s", proc),
				Subtitle: label,
				Keywords: []string{"overmind", "stop", proc, label},
				Kind:     ActionExec,
				Payload:  OvermindStopPayload{ControlSocket: cs, Process: proc},
			})
		}
	}

	return actions, nil
}

// ── Convenience constructor ──

// NewDefaultRegistry creates a registry with all standard providers.
func NewDefaultRegistry(runner tmux.Runner, resolver *theme.Resolver, _ config.FS) *Registry {
	return NewRegistry(
		&SessionsProvider{Runner: runner},
		&ThemesProvider{Resolver: resolver},
		&BarProvider{},
		&DashboardProvider{},
		&HelpProvider{},
		&OvermindProvider{Local: runner.Endpoint()},
	)
}
