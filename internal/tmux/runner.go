// Package tmux provides a typed wrapper over the tmux CLI.
package tmux

// Runner is the interface for all tmux operations.
// All tmux interactions in zmux go through this interface,
// enabling testability via mock implementations.
type Runner interface {
	// Sessions
	ListSessions() ([]Session, error)
	HasSession(name string) bool
	NewSession(name, dir string) error
	NewGroupedSession(target, name string) error
	KillSession(name string) error
	AttachSession(name string) error
	AttachSessionDetach(name string) error
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
	ListPanes(session string) ([]Pane, error)
	SplitWindow(target, direction string) error

	// I/O
	SendKeys(target string, keys ...string) error
	DisplayMessage(target, format string) (string, error)
	CapturePane(target string, lines int) (string, error)

	// Config
	SetOption(scope, key, value string) error
	SetEnvironment(key, value string) error
	SourceFile(path string) error

	// Popup
	DisplayPopup(args ...string) error

	// State
	IsInsideTmux() bool
	ServerRunning() bool
	Version() (string, error)
}
