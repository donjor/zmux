package palette

import (
	"fmt"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/overmind"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tabs"
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

	case PaneActionPayload:
		if err := e.runPaneOp(payload.Op); err != nil {
			return PostAction{Kind: PostError, Err: err}
		}
		return PostAction{Kind: PostClose}

	case TabActionPayload:
		if err := e.runTabOp(payload.Op); err != nil {
			return PostAction{Kind: PostError, Err: err}
		}
		return PostAction{Kind: PostClose}

	case TabHidePayload:
		return e.runTabPlacement(payload.TabID, func(t *tabs.LogicalTab) error {
			return tabs.HideTab(e.Runner, t)
		})

	case TabShowPayload:
		return e.runTabPlacement(payload.TabID, func(t *tabs.LogicalTab) error {
			if _, err := tabs.ShowTab(e.Runner, t); err != nil {
				return err
			}
			return e.focusLogicalTab(t.ID)
		})

	case TabPromotePayload:
		return e.runTabPlacement(payload.TabID, func(t *tabs.LogicalTab) error {
			_, _, err := tabs.PromoteTab(e.Runner, t, false)
			return err
		})

	case TabJoinPayload:
		return e.runTabPlacementFromScan(payload.TabID, func(t *tabs.LogicalTab, all []tabs.LogicalTab) error {
			host, err := tabs.CurrentHostFrom(all, e.Runner)
			if err != nil {
				return err
			}
			if _, err = tabs.JoinTab(e.Runner, t, host, tabs.JoinOptions{Direction: tmux.SplitRight}); err != nil {
				return err
			}
			return e.focusLogicalTab(t.ID)
		})

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

// runPaneOp dispatches a pane-layout op to the typed tmux.Runner method that
// runs it on the active pane/window.
func (e *Executor) runPaneOp(op PaneOp) error {
	switch op {
	case PaneSwapLeft:
		return e.Runner.SwapPane(tmux.SplitLeft)
	case PaneSwapRight:
		return e.Runner.SwapPane(tmux.SplitRight)
	case PaneSwapUp:
		return e.Runner.SwapPane(tmux.SplitUp)
	case PaneSwapDown:
		return e.Runner.SwapPane(tmux.SplitDown)
	case PaneEqualize:
		return e.Runner.EqualizeLayout()
	case PaneOrient:
		return e.Runner.ToggleOrientation()
	case PaneFocusLeft:
		return e.Runner.FocusPane(tmux.SplitLeft)
	case PaneFocusRight:
		return e.Runner.FocusPane(tmux.SplitRight)
	case PaneFocusUp:
		return e.Runner.FocusPane(tmux.SplitUp)
	case PaneFocusDown:
		return e.Runner.FocusPane(tmux.SplitDown)
	}
	return fmt.Errorf("unknown pane op %d", op)
}

// runTabPlacement re-resolves a logical tab by its stable id at run time, then
// runs op against it through the shared placement service. Re-resolving (rather
// than trusting the payload's snapshot) handles a tab that moved or closed since
// the palette opened.
func (e *Executor) runTabPlacement(tabID string, op func(*tabs.LogicalTab) error) PostAction {
	return e.runTabPlacementFromScan(tabID, func(t *tabs.LogicalTab, _ []tabs.LogicalTab) error {
		return op(t)
	})
}

func (e *Executor) runTabPlacementFromScan(tabID string, op func(*tabs.LogicalTab, []tabs.LogicalTab) error) PostAction {
	all, err := tabs.ListLogicalTabs(e.Runner)
	if err != nil {
		return PostAction{Kind: PostError, Err: fmt.Errorf("scan tabs: %w", err)}
	}
	t := tabs.ByID(all, tabID)
	if t == nil {
		return PostAction{Kind: PostError, Err: fmt.Errorf("tab no longer exists")}
	}
	if err := op(t, all); err != nil {
		return PostAction{Kind: PostError, Err: err}
	}
	return PostAction{Kind: PostClose}
}

func (e *Executor) focusLogicalTab(tabID string) error {
	all, err := tabs.ListLogicalTabs(e.Runner)
	if err != nil {
		return fmt.Errorf("focus pane: scan tabs: %w", err)
	}
	t := tabs.ByID(all, tabID)
	if t == nil {
		return fmt.Errorf("focus pane: tab no longer exists")
	}
	if t.Placement == tabs.PlacementDock {
		return fmt.Errorf("focus pane: tab %q is hidden", tabs.DisplayName(t))
	}
	if t.Session != "" {
		if err := e.Runner.SelectWindow(t.Session, t.WindowIndex); err != nil {
			return fmt.Errorf("focus pane: select window: %w", err)
		}
	}
	if t.PaneID == "" {
		return fmt.Errorf("focus pane: tab %q has no pane id", tabs.DisplayName(t))
	}
	if err := e.Runner.SelectPane(t.PaneID); err != nil {
		return fmt.Errorf("focus pane: select pane: %w", err)
	}
	return nil
}

// runTabOp dispatches a tab op to the typed tmux.Runner method that runs it
// relative to the active tab.
func (e *Executor) runTabOp(op TabOp) error {
	switch op {
	case TabNext:
		return e.Runner.NextWindow()
	case TabPrev:
		return e.Runner.PreviousWindow()
	case TabReorderLeft:
		return e.Runner.ReorderWindow(-1)
	case TabReorderRight:
		return e.Runner.ReorderWindow(+1)
	}
	return fmt.Errorf("unknown tab op %d", op)
}
