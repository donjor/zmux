// Package tmux provides a typed wrapper over the tmux CLI.
package tmux

// Runner is the interface for all tmux operations.
// All tmux interactions in zmux go through this interface,
// enabling testability via mock implementations.
type SplitDirection string

const (
	SplitRight SplitDirection = "right"
	SplitLeft  SplitDirection = "left"
	SplitDown  SplitDirection = "down"
	SplitUp    SplitDirection = "up"
)

// SplitPaneOptions describes a tmux split-window call that creates a new pane.
// Pane IDs returned by tmux (for example %57) are opaque and should be stored
// verbatim by callers. Size is a tmux -l value such as "40%" or "80".
type SplitPaneOptions struct {
	Target    string
	Direction SplitDirection
	Size      string
	CWD       string
	Title     string
	Command   []string
}

type Runner interface {
	// Sessions
	ListSessions() ([]Session, error)
	ListClients() ([]ClientInfo, error)
	HasSession(name string) bool
	NewSession(name, dir string) error
	NewGroupedSession(target, name string) error
	KillSession(name string) error
	AttachSession(name string) error
	AttachSessionDetach(name string) error
	RefreshClient(targetClient, session string) error
	SwitchClient(target string) error
	RenameSession(old, new string) error

	// Windows
	ListWindows(session string) ([]Window, error)
	// NewWindow returns the new window's initial pane id (%N) so identity
	// can be stamped at create without a lookup race.
	NewWindow(session, name, dir string, opts ...WindowOpt) (string, error)
	KillWindow(session string, index int) error
	// KillWindowByID kills a window by opaque id (@N) — stable across
	// moves between sessions, unlike session:index targets.
	KillWindowByID(windowID string) error
	RenameWindow(session, old, new string) error
	SelectWindow(session string, index int) error
	MoveWindow(srcSession, dstSession string) error
	SwapWindow(session string, idx1, idx2 int) error

	// Panes
	ListPanes(target string) ([]Pane, error)
	ListWindowPanes(target string) ([]Pane, error)
	ListAllPanes() ([]Pane, error)
	// ListLogicalPaneRows is the one-call logical-tab scan (list-panes -a):
	// every pane on the server with the placement/identity facts the tabs
	// layer computes logical tabs from.
	ListLogicalPaneRows() ([]LogicalPaneRow, error)
	SplitWindow(target, direction string) error
	SplitPane(opts SplitPaneOptions) (string, error)
	// JoinPane relocates a pane into another window; BreakPane promotes a
	// pane to its own window (returning the new window id). Both auto-unzoom
	// affected windows; SelectLayout restores a #{window_layout} snapshot —
	// silently best-effort on pane-count mismatch, so callers compare counts.
	JoinPane(opts JoinPaneOptions) error
	BreakPane(opts BreakPaneOptions) (string, error)
	SelectLayout(target, layout string) error
	ToggleZoom(target string) error
	KillPane(target string) error
	SelectPane(target string) error
	ResizePane(target, axis, size string) error

	// I/O
	SendKeys(target string, keys ...string) error
	DisplayMessage(target, format string) (string, error)
	CapturePane(target string, lines int) (string, error)
	CapturePaneOpts(target string, opts CapturePaneOptions) (string, error)

	// Config
	SetOption(scope, key, value string) error
	SetSessionOption(target, key, value string) error
	SetWindowOption(target, key, value string) error
	UnsetWindowOption(target, key string) error
	SetPaneOption(target, key, value string) error
	UnsetPaneOption(target, key string) error
	// Show*Option read an option scope-exactly (show-options -w/-p, -q):
	// "" when unset, never the merged-scope fallback that format reads do.
	ShowWindowOption(target, key string) (string, error)
	ShowPaneOption(target, key string) (string, error)
	// ApplyOptions batches set-option calls into one tmux invocation
	// (";"-chained) — state writes touch many options; one spawn, not nine.
	ApplyOptions(writes []OptionWrite) error
	// ListPaneOptionValues returns key's value for every pane on the server
	// (list-panes -a), one entry per pane; unset options yield empty strings.
	// Lets tabstate ask "does any running state remain?" in one round-trip.
	ListPaneOptionValues(key string) ([]string, error)
	// RefreshStatus forces a status-line redraw (refresh-client -S). Errors
	// with "no current client" when nothing is attached — callers treat it
	// as best-effort and must not fail state writes on it.
	RefreshStatus() error
	SetEnvironment(key, value string) error
	SourceFile(path string) error

	// Popup
	DisplayPopup(args ...string) error

	// State
	IsInsideTmux() bool
	ServerRunning() bool
	Version() (string, error)
	// Endpoint reports the tmux server socket this runner targets (default,
	// -L name, or -S path). Used by source discovery to treat the runner's own
	// server as "local" rather than hardcoding the default socket.
	Endpoint() Endpoint
}
