package palette

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
)

// noopOvermind is an inert overmind.Client for tests that don't exercise the
// overmind action paths.
type noopOvermind struct{}

func (noopOvermind) Connect(string, string) error        { return nil }
func (noopOvermind) Restart(string, string) error        { return nil }
func (noopOvermind) Stop(string, string) error           { return nil }
func (noopOvermind) StopAll(string) error                { return nil }
func (noopOvermind) Logs(string, string) (string, error) { return "", nil }

func newTestExecutor(t *testing.T) (*Executor, *tmux.MockRunner, *fakeFS) {
	t.Helper()
	mock := tmux.NewMockRunner()
	fs := newFakeFS("/home/user")

	// Seed a minimal config so ThemeSetPayload / BarSetPayload can
	// Load + Save.
	cfg := config.DefaultConfig()
	if err := config.Save(fs, "/home/user/.zmux.toml", cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	return NewExecutor(mock, fs, noopOvermind{}, nil), mock, fs
}

func TestExecutorSessionSwitch(t *testing.T) {
	exe, mock, _ := newTestExecutor(t)
	// Target exists and is unattached, so SwitchView resolves to a plain switch
	// (no clone) and the action closes.
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	mock.InsideTmux = true

	post := exe.Run(Action{Payload: SessionSwitchPayload{Name: "dev"}})
	if post.Kind != PostClose {
		t.Errorf("kind = %v, want PostClose", post.Kind)
	}
}

func TestExecutorSessionSwitchError(t *testing.T) {
	exe, mock, _ := newTestExecutor(t)
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	mock.InsideTmux = true
	mock.Err = &testErr{"switch failed"}

	post := exe.Run(Action{Payload: SessionSwitchPayload{Name: "dev"}})
	if post.Kind != PostError {
		t.Errorf("kind = %v, want PostError", post.Kind)
	}
	if post.Err == nil {
		t.Error("expected error to be propagated")
	}
}

func TestExecutorSessionSwitchClonesAttached(t *testing.T) {
	exe, mock, _ := newTestExecutor(t)
	// Target attached by another client → SwitchView must clone for an
	// independent viewport instead of collapsing onto the shared view.
	mock.Sessions = []tmux.Session{{Name: "dev", Attached: true}}
	mock.InsideTmux = true

	post := exe.Run(Action{Payload: SessionSwitchPayload{Name: "dev"}})
	if post.Kind != PostClose {
		t.Fatalf("kind = %v, want PostClose", post.Kind)
	}

	var cloned, switchedToClone bool
	for _, c := range mock.Calls {
		if c.Method == "NewGroupedSession" && len(c.Args) == 2 && c.Args[0] == "dev" && c.Args[1] == "dev-b" {
			cloned = true
		}
		if c.Method == "SwitchClient" && len(c.Args) == 1 && c.Args[0] == "dev-b" {
			switchedToClone = true
		}
	}
	if !cloned {
		t.Errorf("expected NewGroupedSession(dev, dev-b), calls = %v", mock.Calls)
	}
	if !switchedToClone {
		t.Errorf("expected SwitchClient(dev-b), calls = %v", mock.Calls)
	}
}

func TestExecutorSessionCreate(t *testing.T) {
	exe, _, _ := newTestExecutor(t)

	post := exe.Run(Action{Payload: SessionCreatePayload{}})
	// Create then switch; success → PostClose, failure → PostError.
	if post.Kind != PostClose && post.Kind != PostError {
		t.Errorf("unexpected kind = %v", post.Kind)
	}
}

func TestExecutorSessionKill(t *testing.T) {
	exe, _, _ := newTestExecutor(t)

	post := exe.Run(Action{Payload: SessionKillPayload{Name: "dev"}})
	if post.Kind != PostClose {
		t.Errorf("kind = %v, want PostClose", post.Kind)
	}
}

func TestExecutorThemeSetPersistsInConfig(t *testing.T) {
	exe, _, fs := newTestExecutor(t)

	post := exe.Run(Action{Payload: ThemeSetPayload{Name: "catppuccin-mocha"}})
	if post.Kind != PostClose {
		t.Errorf("kind = %v, want PostClose", post.Kind)
	}

	cfg, err := config.Load(fs, "/home/user/.zmux.toml")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.Theme != "catppuccin-mocha" {
		t.Errorf("persisted theme = %q, want catppuccin-mocha", cfg.Theme)
	}
}

func TestExecutorBarSetPersistsInConfig(t *testing.T) {
	exe, _, fs := newTestExecutor(t)

	post := exe.Run(Action{Payload: BarSetPayload{Preset: "rounded"}})
	if post.Kind != PostClose {
		t.Errorf("kind = %v, want PostClose", post.Kind)
	}

	cfg, err := config.Load(fs, "/home/user/.zmux.toml")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.Bar.Preset != "rounded" {
		t.Errorf("persisted bar = %q, want rounded", cfg.Bar.Preset)
	}
}

func TestExecutorDashboardTab(t *testing.T) {
	exe, _, _ := newTestExecutor(t)
	post := exe.Run(Action{Payload: DashboardTabPayload{Tab: "help"}})
	if post.Kind != PostOpenDashboard {
		t.Errorf("kind = %v, want PostOpenDashboard", post.Kind)
	}
	if post.Tab != "help" {
		t.Errorf("tab = %q, want help", post.Tab)
	}
}

func TestExecutorUnknownPayloadClosesQuietly(t *testing.T) {
	exe, _, _ := newTestExecutor(t)
	post := exe.Run(Action{Payload: "some random thing"})
	if post.Kind != PostClose {
		t.Errorf("unknown payload: kind = %v, want PostClose", post.Kind)
	}
}

// lastCall returns the most recent recorded call with the given method name.
func lastCall(mock *tmux.MockRunner, method string) (tmux.MockCall, bool) {
	for i := len(mock.Calls) - 1; i >= 0; i-- {
		if mock.Calls[i].Method == method {
			return mock.Calls[i], true
		}
	}
	return tmux.MockCall{}, false
}

func TestExecutorTabJoinUsesExecutionScanForHost(t *testing.T) {
	exe, mock, _ := newTestExecutor(t)
	first := []tmux.LogicalPaneRow{
		paletteRow("%1", "work", "@1", "ztab_current", "current"),
		paletteRow("%2", "work", "@2", "ztab_target", "target"),
	}
	second := []tmux.LogicalPaneRow{
		paletteRow("%9", "work", "@1", "ztab_wrong", "wrong"),
		paletteRow("%2", "work", "@2", "ztab_target", "target"),
	}
	mock.InsideTmux = true
	mock.LogicalRowsByCall = [][]tmux.LogicalPaneRow{first, second}
	mock.DisplayMessageFunc = func(_, format string) (string, error) {
		switch {
		case format == "#{pane_id}":
			return "%1\n", nil
		case format == "#{window_id}":
			return "@1\n", nil
		case strings.Contains(format, "session_group"):
			return "\t1\t1\n", nil
		case format == "#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}":
			return "L\t0\t1\t%1\n", nil
		case format == "#{window_panes}":
			return "2\n", nil
		default:
			return "", nil
		}
	}

	post := exe.Run(Action{Payload: TabJoinPayload{TabID: "ztab_target"}})
	if post.Kind != PostClose {
		t.Fatalf("kind = %v, err = %v; want PostClose", post.Kind, post.Err)
	}
	call, ok := lastCall(mock, "JoinPane")
	if !ok {
		t.Fatalf("expected JoinPane call, got %v", mock.Calls)
	}
	if call.Args[0] != "%2" || call.Args[1] != "%1" {
		t.Fatalf("JoinPane args = %v, want source %%2 target current-pane host %%1", call.Args)
	}
	focus, ok := lastCall(mock, "SelectPane")
	if !ok || focus.Args[0] != "%2" {
		t.Fatalf("expected palette join to focus moved pane %%2, got %v (calls=%v)", focus.Args, mock.Calls)
	}
}

func TestExecutorPaneAndTabOpsDispatch(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		method  string
		arg     string // "" = method takes no arg
	}{
		{"swap left", PaneActionPayload{Op: PaneSwapLeft}, "SwapPane", "left"},
		{"swap right", PaneActionPayload{Op: PaneSwapRight}, "SwapPane", "right"},
		{"swap up", PaneActionPayload{Op: PaneSwapUp}, "SwapPane", "up"},
		{"swap down", PaneActionPayload{Op: PaneSwapDown}, "SwapPane", "down"},
		{"equalize", PaneActionPayload{Op: PaneEqualize}, "EqualizeLayout", ""},
		{"orient", PaneActionPayload{Op: PaneOrient}, "ToggleOrientation", ""},
		{"focus left", PaneActionPayload{Op: PaneFocusLeft}, "FocusPane", "left"},
		{"focus right", PaneActionPayload{Op: PaneFocusRight}, "FocusPane", "right"},
		{"focus up", PaneActionPayload{Op: PaneFocusUp}, "FocusPane", "up"},
		{"focus down", PaneActionPayload{Op: PaneFocusDown}, "FocusPane", "down"},
		{"tab next", TabActionPayload{Op: TabNext}, "NextWindow", ""},
		{"tab prev", TabActionPayload{Op: TabPrev}, "PreviousWindow", ""},
		{"reorder left", TabActionPayload{Op: TabReorderLeft}, "ReorderWindow", "-1"},
		{"reorder right", TabActionPayload{Op: TabReorderRight}, "ReorderWindow", "+1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exe, mock, _ := newTestExecutor(t)
			post := exe.Run(Action{Payload: tc.payload})
			if post.Kind != PostClose {
				t.Fatalf("kind = %v, want PostClose", post.Kind)
			}
			call, ok := lastCall(mock, tc.method)
			if !ok {
				t.Fatalf("expected %s call, got %v", tc.method, mock.Calls)
			}
			if tc.arg != "" {
				if len(call.Args) != 1 || call.Args[0] != tc.arg {
					t.Errorf("%s args = %v, want [%s]", tc.method, call.Args, tc.arg)
				}
			} else if len(call.Args) != 0 {
				t.Errorf("%s args = %v, want none", tc.method, call.Args)
			}
		})
	}
}

func TestExecutorPaneTabOpErrorPropagates(t *testing.T) {
	for _, payload := range []any{
		PaneActionPayload{Op: PaneSwapLeft},
		PaneActionPayload{Op: PaneOrient},
		TabActionPayload{Op: TabReorderRight},
	} {
		exe, mock, _ := newTestExecutor(t)
		mock.Err = &testErr{"tmux boom"}
		post := exe.Run(Action{Payload: payload})
		if post.Kind != PostError {
			t.Errorf("%#v: kind = %v, want PostError", payload, post.Kind)
		}
		if post.Err == nil {
			t.Errorf("%#v: expected error propagated", payload)
		}
	}
}
