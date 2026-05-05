// Package wm exposes window-manager metadata used by strict terminal targeting.
package wm

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("unsupported window manager")

// Window is a desktop window candidate with screenshot geometry.
type Window struct {
	WM              string
	Address         string
	Class           string
	Title           string
	PID             int
	Workspace       string
	Geometry        string
	Visible         bool
	Mapped          bool
	Hidden          bool
	OnActiveMonitor bool
}

// Adapter lists desktop windows for one window manager/compositor.
type Adapter interface {
	Windows(ctx context.Context) ([]Window, error)
}
