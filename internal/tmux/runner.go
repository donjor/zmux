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
	NewWindow(session, name, dir string) error
	KillWindow(session string, index int) error
	RenameWindow(session, old, new string) error
	SelectWindow(session string, index int) error
	MoveWindow(srcSession, dstSession string) error
	SwapWindow(session string, idx1, idx2 int) error

	// Panes
	ListPanes(target string) ([]Pane, error)
	ListWindowPanes(target string) ([]Pane, error)
	ListAllPanes() ([]Pane, error)
	SplitWindow(target, direction string) error
	SplitPane(opts SplitPaneOptions) (string, error)
	KillPane(target string) error
	SelectPane(target string) error
	ResizePane(target, axis, size string) error

	// I/O
	SendKeys(target string, keys ...string) error
	DisplayMessage(target, format string) (string, error)
	CapturePane(target string, lines int) (string, error)

	// Config
	SetOption(scope, key, value string) error
	SetSessionOption(target, key, value string) error
	SetWindowOption(target, key, value string) error
	UnsetWindowOption(target, key string) error
	SetEnvironment(key, value string) error
	SourceFile(path string) error

	// Popup
	DisplayPopup(args ...string) error

	// State
	IsInsideTmux() bool
	ServerRunning() bool
	Version() (string, error)
}
