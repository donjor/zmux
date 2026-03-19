// Package dashboard implements the tabbed dashboard TUI for zmux.
// It runs as a tmux popup (prefix+Space) and provides a premium,
// Charm CLI-aesthetic interface for session, theme, and config management.
package dashboard

import tea "github.com/charmbracelet/bubbletea"

// TabID identifies a dashboard tab.
type TabID string

const (
	TabCurrent  TabID = "current"
	TabSessions TabID = "sessions"
	TabSettings TabID = "settings"
	TabHelp     TabID = "help"

	// Deprecated: kept for backward compatibility in tests.
	TabThemes TabID = "themes"
	TabConfig TabID = "config"
)

// TabOrder defines the canonical ordering of tabs.
var TabOrder = []TabID{TabCurrent, TabSessions, TabSettings, TabHelp}

// ActivateReason indicates why a tab is being activated.
type ActivateReason int

const (
	// ActivateInit is the first activation at startup.
	ActivateInit ActivateReason = iota
	// ActivateTabSwitch occurs when switching tabs.
	ActivateTabSwitch
	// ActivateAfterMutation occurs after a data mutation.
	ActivateAfterMutation
)

// Tab is the interface every dashboard tab must implement.
type Tab interface {
	// ID returns the tab's unique identifier.
	ID() TabID

	// Title returns the display title for the tab bar.
	Title() string

	// Init performs initial setup and returns a command to run.
	Init() tea.Cmd

	// Update processes a message and returns the updated tab and command.
	Update(tea.Msg) (Tab, tea.Cmd)

	// View renders the tab's content (without chrome).
	View() string

	// Resize updates the tab's available content area.
	Resize(width, height int)

	// Activate is called when a tab becomes active.
	// The reason indicates why activation is happening.
	// Returns a command for async data fetching.
	Activate(reason ActivateReason) tea.Cmd

	// Deactivate is called when a tab loses focus.
	// Drop transient overlays, blur text inputs.
	Deactivate()

	// ShortHelp returns a compact help string for the footer.
	ShortHelp() string
}

// TargetedMsg is a message destined for a specific tab.
// Async results implement this so the app can route them correctly.
type TargetedMsg interface {
	TargetTab() TabID
}

// AppIntentMsg is a message from a tab to its parent app.
// Used for cross-cutting concerns like tab switching or status flashes.
type AppIntentMsg interface {
	AppIntent()
}
