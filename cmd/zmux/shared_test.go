package main

import (
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// withMockApp swaps the global app with a mock-backed one for testing,
// restoring the original after the test completes.
func withMockApp(t *testing.T) *tmux.MockRunner {
	t.Helper()
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.DisplayMessageResult = "test-session"

	orig := app
	app = &App{
		FS:             &config.RealFS{},
		Runner:         mock,
		WorkspaceStore: workspace.NewStore(&config.RealFS{}),
	}
	t.Cleanup(func() { app = orig })
	return mock
}
