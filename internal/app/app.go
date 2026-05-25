// Package app is the zmux composition root. It holds the injected
// dependencies (tmux runner, config filesystem, workspace store) that the CLI
// command layer and TUI surfaces are built against, constructed once in main
// and passed down explicitly rather than reached for through a package global.
package app

import (
	"path/filepath"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/overmind"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// App centralizes service construction and dependency injection.
// Cobra commands receive services through this struct.
type App struct {
	FS             config.FS
	Runner         tmux.Runner
	WorkspaceStore *workspace.Store
	Overmind       overmind.Client
	ConfigPath     string
	Profile        config.Profile
}

// New creates an App with real implementations, wired to the active isolation
// profile (resolved from the invoking binary name — see config.Profile). When
// invoked as `zzmux`, the runner targets the `-L zzmux` server and config/state
// paths point at the zzmux profile, so nothing collides with the live `zmux`.
func New() *App {
	fs := &config.RealFS{}
	profile := config.ActiveProfile(fs)

	runner := tmux.NewClient()
	if profile.Socket != "" {
		runner = tmux.NewClientFor(tmux.NamedEndpoint(profile.Socket))
	}

	return &App{
		FS:             fs,
		Runner:         runner,
		WorkspaceStore: workspace.NewStoreAt(fs, filepath.Join(profile.StateDir, "workspaces.toml")),
		Overmind:       overmind.CLI{},
		ConfigPath:     profile.ConfigFile,
		Profile:        profile,
	}
}
