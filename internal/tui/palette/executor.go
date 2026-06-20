package palette

import (
	"fmt"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/overmind"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// PostActionKind classifies what should happen after executing an action.
type PostActionKind int

const (
	// PostClose closes the palette (and the popup).
	PostClose PostActionKind = iota
	// PostOpenDashboard opens a specific dashboard tab.
	PostOpenDashboard
	// PostError signals the action failed.
	PostError
)

// PostAction describes what the caller should do after executing an action.
type PostAction struct {
	Kind PostActionKind
	Tab  string // for PostOpenDashboard
	Err  error  // for PostError
}

// Executor runs a chosen Action against the real tmux/config backends.
type Executor struct {
	Runner   tmux.Runner
	FS       config.FS
	Overmind overmind.Client
	WSStore  workspace.MembershipRemover
}

// NewExecutor creates an executor with the given dependencies. wsStore may
// be nil; kill-session actions fall back to a plain tmux kill in that case.
func NewExecutor(runner tmux.Runner, fs config.FS, om overmind.Client, wsStore workspace.MembershipRemover) *Executor {
	return &Executor{Runner: runner, FS: fs, Overmind: om, WSStore: wsStore}
}

// Run executes the given action and returns what the caller should do next.
func (e *Executor) Run(action Action) PostAction {
	switch payload := action.Payload.(type) {
	case SessionSwitchPayload:
		// Switching to an existing session: route through SwitchView so a target
		// attached by another client gets an independent viewport (clone) rather
		// than collapsing onto the shared view — same as the workspace/dashboard
		// switch paths. No follow-up window/pane select here, so the landed
		// session is discarded.
		if _, err := session.SwitchView(e.Runner, payload.Name); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("switch session: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case SessionCreatePayload:
		name := session.NextTmpName(e.Runner)
		if err := session.Create(e.Runner, name, "."); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("create session: %w", err)}
		}
		if err := session.Switch(e.Runner, name); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("switch to new session: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case SessionKillPayload:
		if err := workspace.KillSession(e.Runner, e.WSStore, payload.Name); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("kill session: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case ThemeSetPayload:
		cfgPath, err := config.ConfigPath(e.FS)
		if err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("config path: %w", err)}
		}
		cfg, err := config.Load(e.FS, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		cfg.Theme = payload.Name
		if err := config.Save(e.FS, cfgPath, cfg); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("save config: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case BarSetPayload:
		cfgPath, err := config.ConfigPath(e.FS)
		if err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("config path: %w", err)}
		}
		cfg, err := config.Load(e.FS, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		cfg.Bar.Preset = payload.Preset
		if err := config.Save(e.FS, cfgPath, cfg); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("save config: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case DashboardTabPayload:
		return PostAction{Kind: PostOpenDashboard, Tab: payload.Tab}

	case OvermindConnectPayload:
		if err := e.Overmind.Connect(payload.ControlSocket, payload.Process); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("overmind connect: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case OvermindRestartPayload:
		if err := e.Overmind.Restart(payload.ControlSocket, payload.Process); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("overmind restart: %w", err)}
		}
		return PostAction{Kind: PostClose}

	case OvermindStopPayload:
		if err := e.Overmind.Stop(payload.ControlSocket, payload.Process); err != nil {
			return PostAction{Kind: PostError, Err: fmt.Errorf("overmind stop: %w", err)}
		}
		return PostAction{Kind: PostClose}

	default:
		return PostAction{Kind: PostClose}
	}
}
