package palette

import (
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

	return NewExecutor(mock, fs, noopOvermind{}), mock, fs
}

func TestExecutorSessionSwitch(t *testing.T) {
	exe, mock, _ := newTestExecutor(t)
	// Mock must pretend the target exists so Switch doesn't short-circuit.
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
