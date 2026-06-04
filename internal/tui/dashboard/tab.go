// Package dashboard implements the tabbed dashboard TUI for zmux.
// It runs as a tmux popup (prefix+Space) and provides a premium,
// Charm CLI-aesthetic interface for session, theme, and config management.
package dashboard

import tea "charm.land/bubbletea/v2"

// TabID identifies a dashboard tab.
type TabID string

const (
	TabSession    TabID = "session"    // current session + workspace context
	TabWorkspaces TabID = "workspaces" // global workspace management
	TabThemes     TabID = "themes"
	TabBar        TabID = "bar"
	TabSettings   TabID = "settings"
	TabHelp       TabID = "help"
)

// CLI back-compat for the --dashboard-tab flag (and the corresponding
// palette action arguments) is handled as a string-level alias by
// internal/cli/root.go:resolveDashboardTab. The old Go constants
// (TabCurrent / TabSessions) have been removed — call sites in the
// tree were all audited and none relied on them.

// TabOrder defines the canonical ordering of tabs.
var TabOrder = []TabID{TabSession, TabWorkspaces, TabThemes, TabBar, TabSettings, TabHelp}

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

	// CapturesEscape reports whether the tab is currently in a mode that owns
	// the Escape key (an inline rename/create/search/edit input, a y/N
	// confirm, etc.). When true, the dashboard routes Esc to the tab so it can
	// cancel that mode instead of closing the whole popup. List/idle state
	// returns false, so Esc falls through to the global "close dashboard"
	// binding. Ctrl+C always closes regardless.
	CapturesEscape() bool
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
