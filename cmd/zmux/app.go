package main

import (
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// App centralizes service construction and dependency injection.
// All cobra commands receive services through this struct.
type App struct {
	FS             config.FS
	Runner         tmux.Runner
	WorkspaceStore *workspace.Store
	// Config will be set after loading in Phase 2
	ConfigPath string
}

// NewApp creates an App with real implementations.
func NewApp() *App {
	fs := &config.RealFS{}
	return &App{
		FS:             fs,
		Runner:         tmux.NewClient(),
		WorkspaceStore: workspace.NewStore(fs),
	}
}
