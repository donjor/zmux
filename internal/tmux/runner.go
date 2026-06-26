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
	// NewSessionWindow creates a detached session whose first window is named
	// `window`, returning that window's initial pane id (%N). No client is
	// attached or switched, so worker/background session birth never steals
	// focus; naming the first window at creation avoids a leftover blank shell
	// tab (no follow-up NewWindow needed).
	NewSessionWindow(session, window, dir string) (string, error)
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
	// Pane/tab layout ops on the active pane/window — the palette executor and
	// the prefix keybinds share these so the two surfaces stay in lockstep.
	// SwapPane/FocusPane take a direction; ReorderWindow takes -1/+1; orient
	// shares OrientHorizontalMatch with the prefix+s bind.
	SwapPane(dir SplitDirection) error
	FocusPane(dir SplitDirection) error
	EqualizeLayout() error
	ToggleOrientation() error
	NextWindow() error
	PreviousWindow() error
	ReorderWindow(delta int) error

	// I/O
	SendKeys(target string, keys ...string) error
	DisplayMessage(target, format string) (string, error)
	// ShowMessage flashes a transient message on the current client's status
	// line (display-message with no -p). Unlike DisplayMessage it shows text
	// rather than reading a format value — used by keybind run-shell wrappers to
	// report an outcome without tmux's view-mode takeover.
	ShowMessage(text string) error
	CapturePane(target string, lines int) (string, error)
	CapturePaneOpts(target string, opts CapturePaneOptions) (string, error)
	// PipePane streams a pane's output to a shell command continuously via tmux
	// pipe-pane (server-side, survives client detach). A non-empty command opens
	// the pipe — tmux runs it through /bin/sh -c and feeds the pane's raw output
	// to its stdin until the pipe closes or the pane dies. An empty command
	// closes any pipe open on the target. The on/off bit is queryable via
	// DisplayMessage(target, "#{pane_pipe}").
	PipePane(target, command string) error

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
	// ShowGlobalOption reads a global option (show-options -gqv): "" when unset.
	// Client-independent, unlike DisplayMessage format expansion — safe to call
	// from a run-shell hook with no attached client (the reaper throttle stamp).
	ShowGlobalOption(key string) (string, error)
	// PaneHasLiveChildren reports whether the pane's foreground shell (pane_pid)
	// has any child process — a backgrounded job under an idle prompt that
	// pane_current_command can't see. False on lookup failure (never block on a
	// guess). The reaper vetoes a kill when true.
	PaneHasLiveChildren(panePID int) bool
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
