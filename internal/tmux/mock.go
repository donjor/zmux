package tmux

import "fmt"

// MockCall records a method invocation on MockRunner.
type MockCall struct {
	Method string
	Args   []string
}

// MockRunner implements Runner with configurable return data and call recording.
type MockRunner struct {
	Sessions    []Session
	Windows     map[string][]Window // keyed by session name
	Panes       map[string][]Pane   // keyed by session name
	InsideTmux  bool
	ServerUp    bool
	TmuxVersion string
	Calls       []MockCall

	// Endpoint is the tmux server endpoint this mock is associated with.
	Endpoint Endpoint

	// DisplayMessageResult is the content returned by DisplayMessage.
	DisplayMessageResult string

	// CapturedPaneContent is the content returned by CapturePane.
	CapturedPaneContent string

	// CapturePaneFunc, if set, overrides CapturedPaneContent with dynamic responses.
	CapturePaneFunc func(target string, lines int) (string, error)

	// Optional error to return from any method.
	Err error
}

// NewMockRunner creates a MockRunner with sensible defaults.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Windows:     make(map[string][]Window),
		Panes:       make(map[string][]Pane),
		TmuxVersion: "3.4",
		ServerUp:    true,
	}
}

func (m *MockRunner) record(method string, args ...string) {
	m.Calls = append(m.Calls, MockCall{Method: method, Args: args})
}

// ListSessions returns the configured sessions.
func (m *MockRunner) ListSessions() ([]Session, error) {
	m.record("ListSessions")
	return m.Sessions, m.Err
}

// HasSession returns true if a session with the given name is in the configured list.
func (m *MockRunner) HasSession(name string) bool {
	m.record("HasSession", name)
	for _, s := range m.Sessions {
		if s.Name == name {
			return true
		}
	}
	return false
}

// NewSession records the call.
func (m *MockRunner) NewSession(name, dir string) error {
	m.record("NewSession", name, dir)
	return m.Err
}

// NewGroupedSession records the call.
func (m *MockRunner) NewGroupedSession(target, name string) error {
	m.record("NewGroupedSession", target, name)
	return m.Err
}

// KillSession records the call.
func (m *MockRunner) KillSession(name string) error {
	m.record("KillSession", name)
	return m.Err
}

// AttachSession records the call.
func (m *MockRunner) AttachSession(name string) error {
	m.record("AttachSession", name)
	return m.Err
}

// AttachSessionDetach records the call.
func (m *MockRunner) AttachSessionDetach(name string) error {
	m.record("AttachSessionDetach", name)
	return m.Err
}

// SwitchClient records the call.
func (m *MockRunner) SwitchClient(target string) error {
	m.record("SwitchClient", target)
	return m.Err
}

// RenameSession records the call.
func (m *MockRunner) RenameSession(old, new string) error {
	m.record("RenameSession", old, new)
	return m.Err
}

// ListWindows returns the configured windows for a session.
func (m *MockRunner) ListWindows(session string) ([]Window, error) {
	m.record("ListWindows", session)
	return m.Windows[session], m.Err
}

// NewWindow records the call.
func (m *MockRunner) NewWindow(session, name, dir string) error {
	m.record("NewWindow", session, name, dir)
	return m.Err
}

// KillWindow records the call.
func (m *MockRunner) KillWindow(session string, index int) error {
	m.record("KillWindow", session, fmt.Sprintf("%d", index))
	return m.Err
}

// RenameWindow records the call.
func (m *MockRunner) RenameWindow(session, old, new string) error {
	m.record("RenameWindow", session, old, new)
	return m.Err
}

// SelectWindow records the call.
func (m *MockRunner) SelectWindow(session string, index int) error {
	m.record("SelectWindow", session, fmt.Sprintf("%d", index))
	return m.Err
}

// MoveWindow records the call.
func (m *MockRunner) MoveWindow(srcSession, dstSession string) error {
	m.record("MoveWindow", srcSession, dstSession)
	return m.Err
}

// SwapWindow records the call.
func (m *MockRunner) SwapWindow(session string, idx1, idx2 int) error {
	m.record("SwapWindow", session, fmt.Sprintf("%d", idx1), fmt.Sprintf("%d", idx2))
	return m.Err
}

// ListPanes returns the configured panes for a session.
func (m *MockRunner) ListPanes(session string) ([]Pane, error) {
	m.record("ListPanes", session)
	return m.Panes[session], m.Err
}

// SplitWindow records the call.
func (m *MockRunner) SplitWindow(target, direction string) error {
	m.record("SplitWindow", target, direction)
	return m.Err
}

// SendKeys records the call.
func (m *MockRunner) SendKeys(target string, keys ...string) error {
	m.record("SendKeys", append([]string{target}, keys...)...)
	return m.Err
}

// DisplayMessage records the call and returns the configured result.
func (m *MockRunner) DisplayMessage(target, format string) (string, error) {
	m.record("DisplayMessage", target, format)
	return m.DisplayMessageResult, m.Err
}

// CapturePane records the call and returns the configured content.
func (m *MockRunner) CapturePane(target string, lines int) (string, error) {
	m.record("CapturePane", target, fmt.Sprintf("%d", lines))
	if m.CapturePaneFunc != nil {
		return m.CapturePaneFunc(target, lines)
	}
	return m.CapturedPaneContent, m.Err
}

// SetOption records the call.
func (m *MockRunner) SetOption(scope, key, value string) error {
	m.record("SetOption", scope, key, value)
	return m.Err
}

// SetEnvironment records the call.
func (m *MockRunner) SetEnvironment(key, value string) error {
	m.record("SetEnvironment", key, value)
	return m.Err
}

// SourceFile records the call.
func (m *MockRunner) SourceFile(path string) error {
	m.record("SourceFile", path)
	return m.Err
}

// DisplayPopup records the call.
func (m *MockRunner) DisplayPopup(args ...string) error {
	m.record("DisplayPopup", args...)
	return m.Err
}

// IsInsideTmux returns the configured value.
func (m *MockRunner) IsInsideTmux() bool {
	m.record("IsInsideTmux")
	return m.InsideTmux
}

// ServerRunning returns the configured value.
func (m *MockRunner) ServerRunning() bool {
	m.record("ServerRunning")
	return m.ServerUp
}

// Version returns the configured version string.
func (m *MockRunner) Version() (string, error) {
	m.record("Version")
	return m.TmuxVersion, m.Err
}
