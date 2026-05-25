package dashboard

import (
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/styles"
)

// SwitchTabIntent requests the app switch to a specific tab.
type SwitchTabIntent struct {
	Tab TabID
}

func (SwitchTabIntent) AppIntent() {}

// SetStatusIntent requests the app display a status flash message.
type SetStatusIntent struct {
	Text    string
	IsError bool
}

func (SetStatusIntent) AppIntent() {}

// QuitIntent requests the app terminate.
type QuitIntent struct {
	// Action and Chosen carry the result for the caller.
	Action string
	Chosen string
}

func (QuitIntent) AppIntent() {}

// ThemeChangedMsg is broadcast to ALL tabs when the active theme changes.
// Not a TargetedMsg — the dashboard intercepts and forwards to every tab.
type ThemeChangedMsg struct {
	Palette theme.Palette
	Styles  styles.Styles
}

// ThemeChangeIntent is emitted by a tab to request a theme change.
// The dashboard converts it to ThemeChangedMsg and broadcasts.
type ThemeChangeIntent struct {
	Palette theme.Palette
	Styles  styles.Styles
}

func (ThemeChangeIntent) AppIntent() {}
