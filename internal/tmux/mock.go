package tmux

import (
	"fmt"
	"strconv"
)

// MockCall records a method invocation on MockRunner.
type MockCall struct {
	Method string
	Args   []string
}

// MockRunner implements Runner with configurable return data and call recording.
type MockRunner struct {
	Sessions    []Session
	Clients     []ClientInfo
	Windows     map[string][]Window // keyed by session name
	Panes       map[string][]Pane   // keyed by session name
	InsideTmux  bool
	ServerUp    bool
	TmuxVersion string
	Calls       []MockCall

	// Endpt is the tmux server endpoint this mock is associated with, returned
	// by the Endpoint() method (Runner interface).
	Endpt Endpoint

	// DisplayMessageResult is the content returned by DisplayMessage.
	DisplayMessageResult string

	// DisplayMessageFunc, if set, overrides DisplayMessageResult with dynamic
	// responses (e.g. keyed on the format string).
	DisplayMessageFunc func(target, format string) (string, error)

	// CapturedPaneContent is the content returned by CapturePane.
	CapturedPaneContent string

	// CapturePaneFunc, if set, overrides CapturedPaneContent with dynamic responses.
	CapturePaneFunc func(target string, lines int) (string, error)

	// CapturePaneOptsFunc, if set, overrides CapturedPaneContent for CapturePaneOpts.
	CapturePaneOptsFunc func(target string, opts CapturePaneOptions) (string, error)

	// RefreshStatusErr is returned by RefreshStatus only — models the
	// best-effort "no current client" failure without poisoning other calls.
	RefreshStatusErr error

	// PaneOptionValues backs ListPaneOptionValues, keyed by option name —
	// one entry per pane, as list-panes -a would report.
	PaneOptionValues map[string][]string

	// LogicalRows backs ListLogicalPaneRows.
	LogicalRows []LogicalPaneRow

	// LogicalRowsByCall, when set, returns successive entries on successive
	// ListLogicalPaneRows calls (last repeats) — for plan-scan vs fresh-scan tests.
	LogicalRowsByCall [][]LogicalPaneRow
	logicalCalls      int

	// NewWindowPaneID is the pane id NewWindow returns ("" by default).
	NewWindowPaneID string

	// NewSessionWindowPaneID is the pane id NewSessionWindow returns ("" by default).
	NewSessionWindowPaneID string

	// BreakPaneWindowID is the window id BreakPane returns ("" by default).
	BreakPaneWindowID string

	// WindowOptions/PaneOptions back the scope-exact Show*Option reads,
	// keyed "target\x00key"; missing keys read as "" (unset).
	WindowOptions map[string]string
	PaneOptions   map[string]string

	// GlobalOptions backs ShowGlobalOption, keyed by option name.
	GlobalOptions map[string]string

	// PaneChildren backs PaneHasLiveChildren, keyed by pane pid.
	PaneChildren map[int]bool

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

// ListClients returns the configured clients.
func (m *MockRunner) ListClients() ([]ClientInfo, error) {
	m.record("ListClients")
	return m.Clients, m.Err
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
	if m.Err != nil {
		return m.Err
	}
	m.Sessions = append(m.Sessions, Session{Name: name, Dir: dir})
	return nil
}

// NewSessionWindow records the call, registers the session (so a later
// HasSession is true), and returns NewSessionWindowPaneID. It deliberately does
// NOT route through NewSession/NewWindow — tests must distinguish "session with
// a named first window" from "session plus a second tab".
func (m *MockRunner) NewSessionWindow(session, window, dir string) (string, error) {
	m.record("NewSessionWindow", session, window, dir)
	if m.Err != nil {
		return "", m.Err
	}
	m.Sessions = append(m.Sessions, Session{Name: session})
	return m.NewSessionWindowPaneID, nil
}

// NewGroupedSession records the call.
func (m *MockRunner) NewGroupedSession(target, name string) error {
	m.record("NewGroupedSession", target, name)
	if m.Err != nil {
		return m.Err
	}
	m.Sessions = append(m.Sessions, Session{Name: name, Group: target})
	return nil
}

// KillSession records the call.
func (m *MockRunner) KillSession(name string) error {
	m.record("KillSession", name)
	if m.Err != nil {
		return m.Err
	}
	for i, s := range m.Sessions {
		if s.Name == name {
			m.Sessions = append(m.Sessions[:i], m.Sessions[i+1:]...)
			break
		}
	}
	return nil
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

// RefreshClient records the call.
func (m *MockRunner) RefreshClient(targetClient, session string) error {
	m.record("RefreshClient", targetClient, session)
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
	if m.Err != nil {
		return m.Err
	}
	for i := range m.Sessions {
		if m.Sessions[i].Name == old {
			m.Sessions[i].Name = new
			break
		}
	}
	return nil
}

// ListWindows returns the configured windows for a session.
func (m *MockRunner) ListWindows(session string) ([]Window, error) {
	m.record("ListWindows", session)
	return m.Windows[session], m.Err
}

// NewWindow records the call and returns NewWindowPaneID. The recorded args
// include detached=<bool> so tests can assert focus-safe creation.
func (m *MockRunner) NewWindow(session, name, dir string, opts ...WindowOpt) (string, error) {
	var o windowOpts
	for _, fn := range opts {
		fn(&o)
	}
	m.record("NewWindow", session, name, dir, fmt.Sprintf("detached=%v", o.detached))
	return m.NewWindowPaneID, m.Err
}

// KillWindowByID records the call.
func (m *MockRunner) KillWindowByID(windowID string) error {
	m.record("KillWindowByID", windowID)
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

// ListWindowPanes returns the configured panes for a target window/pane.
func (m *MockRunner) ListWindowPanes(target string) ([]Pane, error) {
	m.record("ListWindowPanes", target)
	return m.Panes[target], m.Err
}

// ListAllPanes returns all configured panes.
func (m *MockRunner) ListAllPanes() ([]Pane, error) {
	m.record("ListAllPanes")
	var panes []Pane
	for _, group := range m.Panes {
		panes = append(panes, group...)
	}
	return panes, m.Err
}

// ListLogicalPaneRows returns the configured logical scan rows. When
// LogicalRowsByCall is set, successive calls return successive entries (the last
// repeats) — lets tests model state changing between a plan scan and a later
// fresh re-scan; otherwise every call returns LogicalRows.
func (m *MockRunner) ListLogicalPaneRows() ([]LogicalPaneRow, error) {
	m.record("ListLogicalPaneRows")
	if len(m.LogicalRowsByCall) > 0 {
		i := m.logicalCalls
		if i >= len(m.LogicalRowsByCall) {
			i = len(m.LogicalRowsByCall) - 1
		}
		m.logicalCalls++
		return m.LogicalRowsByCall[i], m.Err
	}
	return m.LogicalRows, m.Err
}

// JoinPane records the call.
func (m *MockRunner) JoinPane(opts JoinPaneOptions) error {
	m.record("JoinPane", opts.Source, opts.Target, string(opts.Direction), opts.Size, fmt.Sprintf("detached=%v", opts.Detached))
	return m.Err
}

// BreakPane records the call and returns BreakPaneWindowID.
func (m *MockRunner) BreakPane(opts BreakPaneOptions) (string, error) {
	m.record("BreakPane", opts.Source, opts.Target, opts.Name, fmt.Sprintf("after=%v", opts.After), fmt.Sprintf("detached=%v", opts.Detached))
	if m.Err != nil {
		return "", m.Err
	}
	return m.BreakPaneWindowID, nil
}

// SelectLayout records the call.
func (m *MockRunner) SelectLayout(target, layout string) error {
	m.record("SelectLayout", target, layout)
	return m.Err
}

// ToggleZoom records the call.
func (m *MockRunner) ToggleZoom(target string) error {
	m.record("ToggleZoom", target)
	return m.Err
}

// SplitWindow records the call.
func (m *MockRunner) SplitWindow(target, direction string) error {
	m.record("SplitWindow", target, direction)
	return m.Err
}

// SplitPane records the call and returns a deterministic pane id.
func (m *MockRunner) SplitPane(opts SplitPaneOptions) (string, error) {
	m.record("SplitPane", opts.Target, string(opts.Direction), opts.Size, opts.CWD, opts.Title, fmt.Sprintf("%q", opts.Command))
	if m.Err != nil {
		return "", m.Err
	}
	return "%57", nil
}

// KillPane records the call.
func (m *MockRunner) KillPane(target string) error {
	m.record("KillPane", target)
	return m.Err
}

// SelectPane records the call.
func (m *MockRunner) SelectPane(target string) error {
	m.record("SelectPane", target)
	return m.Err
}

// ResizePane records the call.
func (m *MockRunner) ResizePane(target, axis, size string) error {
	m.record("ResizePane", target, axis, size)
	return m.Err
}

// SwapPane records the directional pane swap.
func (m *MockRunner) SwapPane(dir SplitDirection) error {
	m.record("SwapPane", string(dir))
	return m.Err
}

// FocusPane records the directional focus move.
func (m *MockRunner) FocusPane(dir SplitDirection) error {
	m.record("FocusPane", string(dir))
	return m.Err
}

// EqualizeLayout records the even-spread call.
func (m *MockRunner) EqualizeLayout() error {
	m.record("EqualizeLayout")
	return m.Err
}

// ToggleOrientation records the orient toggle.
func (m *MockRunner) ToggleOrientation() error {
	m.record("ToggleOrientation")
	return m.Err
}

// NextWindow records the next-tab move.
func (m *MockRunner) NextWindow() error {
	m.record("NextWindow")
	return m.Err
}

// PreviousWindow records the previous-tab move.
func (m *MockRunner) PreviousWindow() error {
	m.record("PreviousWindow")
	return m.Err
}

// ReorderWindow records the relative tab reorder (-1/+1).
func (m *MockRunner) ReorderWindow(delta int) error {
	m.record("ReorderWindow", fmt.Sprintf("%+d", delta))
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
	if m.DisplayMessageFunc != nil {
		return m.DisplayMessageFunc(target, format)
	}
	return m.DisplayMessageResult, m.Err
}

// ShowMessage records the flashed status-line message.
func (m *MockRunner) ShowMessage(text string) error {
	m.record("ShowMessage", text)
	return m.Err
}

// CapturePane records the call and returns the configured content, honoring
// the same "at most `lines` logical lines" contract as the real client so
// bounded-capture behavior is testable through the mock.
func (m *MockRunner) CapturePane(target string, lines int) (string, error) {
	m.record("CapturePane", target, fmt.Sprintf("%d", lines))
	if m.CapturePaneFunc != nil {
		out, err := m.CapturePaneFunc(target, lines)
		return tailLines(out, lines), err
	}
	return tailLines(m.CapturedPaneContent, lines), m.Err
}

// CapturePaneOpts records the call and returns the configured content.
func (m *MockRunner) CapturePaneOpts(target string, opts CapturePaneOptions) (string, error) {
	m.record("CapturePaneOpts", target, fmt.Sprintf("lines=%d ansi=%t join=%t", opts.Lines, opts.ANSI, opts.Join))
	if m.CapturePaneOptsFunc != nil {
		return m.CapturePaneOptsFunc(target, opts)
	}
	if m.CapturePaneFunc != nil {
		return m.CapturePaneFunc(target, opts.Lines)
	}
	return m.CapturedPaneContent, m.Err
}

// PipePane records the call. An empty command models closing the pipe.
func (m *MockRunner) PipePane(target, command string) error {
	m.record("PipePane", target, command)
	return m.Err
}

// ListPaneOptionValues returns the configured per-pane values for key.
func (m *MockRunner) ListPaneOptionValues(key string) ([]string, error) {
	m.record("ListPaneOptionValues", key)
	return m.PaneOptionValues[key], m.Err
}

// SetOption records the call.
func (m *MockRunner) SetOption(scope, key, value string) error {
	m.record("SetOption", scope, key, value)
	return m.Err
}

// SetSessionOption records the call.
func (m *MockRunner) SetSessionOption(target, key, value string) error {
	m.record("SetSessionOption", target, key, value)
	return m.Err
}

// SetWindowOption records the call.
func (m *MockRunner) SetWindowOption(target, key, value string) error {
	m.record("SetWindowOption", target, key, value)
	return m.Err
}

// UnsetWindowOption records the call.
func (m *MockRunner) UnsetWindowOption(target, key string) error {
	m.record("UnsetWindowOption", target, key)
	return m.Err
}

// SetPaneOption records the call.
func (m *MockRunner) SetPaneOption(target, key, value string) error {
	m.record("SetPaneOption", target, key, value)
	return m.Err
}

// UnsetPaneOption records the call.
func (m *MockRunner) UnsetPaneOption(target, key string) error {
	m.record("UnsetPaneOption", target, key)
	return m.Err
}

// ApplyOptions records one call per write (method "ApplyOptions") so tests
// can assert batch contents without caring about argv encoding.
func (m *MockRunner) ApplyOptions(writes []OptionWrite) error {
	for _, w := range writes {
		m.record("ApplyOptions", string(w.Scope), w.Target, w.Key, w.Value, fmt.Sprintf("unset=%v", w.Unset))
	}
	return m.Err
}

// ShowWindowOption returns the configured window option ("" when absent).
func (m *MockRunner) ShowWindowOption(target, key string) (string, error) {
	m.record("ShowWindowOption", target, key)
	return m.WindowOptions[target+"\x00"+key], m.Err
}

// ShowPaneOption returns the configured pane option ("" when absent).
func (m *MockRunner) ShowPaneOption(target, key string) (string, error) {
	m.record("ShowPaneOption", target, key)
	return m.PaneOptions[target+"\x00"+key], m.Err
}

// ShowGlobalOption returns the configured global option ("" when absent).
func (m *MockRunner) ShowGlobalOption(key string) (string, error) {
	m.record("ShowGlobalOption", key)
	return m.GlobalOptions[key], m.Err
}

// PaneHasLiveChildren returns the configured value for panePID (false default).
func (m *MockRunner) PaneHasLiveChildren(panePID int) bool {
	m.record("PaneHasLiveChildren", strconv.Itoa(panePID))
	return m.PaneChildren[panePID]
}

// RefreshStatus records the call.
func (m *MockRunner) RefreshStatus() error {
	m.record("RefreshStatus")
	return m.RefreshStatusErr
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

// Endpoint returns the configured endpoint (default zero value = default server).
func (m *MockRunner) Endpoint() Endpoint {
	m.record("Endpoint")
	return m.Endpt
}

// Version returns the configured version string.
func (m *MockRunner) Version() (string, error) {
	m.record("Version")
	return m.TmuxVersion, m.Err
}
