package cli

import (
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

// newTestApp creates a mock-backed App for testing, backed by an in-memory FS
// so tests stay hermetic (no real home/disk access).
func newTestApp(t *testing.T) (*apppkg.App, *tmux.MockRunner) {
	t.Helper()
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.DisplayMessageResult = "test-session"

	fs := newMemFS("/home/user")
	a := &apppkg.App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: workspace.NewStore(fs),
		Overmind:       noopOvermind{},
	}
	return a, mock
}

// noopOvermind is an inert overmind.Client so test-built Apps don't carry a nil
// Overmind — guards against a future cli test driving an overmind-connect path.
type noopOvermind struct{}

func (noopOvermind) Connect(string, string) error        { return nil }
func (noopOvermind) Restart(string, string) error        { return nil }
func (noopOvermind) Stop(string, string) error           { return nil }
func (noopOvermind) StopAll(string) error                { return nil }
func (noopOvermind) Logs(string, string) (string, error) { return "", nil }

// withMockApp creates a mock-backed App and a root command for testing.
// Returns the root command (for Execute/SetArgs) and the mock runner.
func withMockApp(t *testing.T) (*cobra.Command, *tmux.MockRunner) {
	t.Helper()
	a, mock := newTestApp(t)
	return NewRootCmd(a, testVersion), mock
}
