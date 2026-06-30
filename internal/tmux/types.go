package tmux

import "time"

// Session represents a tmux session with enriched metadata.
type Session struct {
	Name         string
	Windows      int
	Attached     bool
	Activity     time.Time
	Created      time.Time
	LastAttached time.Time
	Dir          string
	Group        string // session group name (empty if ungrouped)
	Clone        bool   // zmux-created grouped viewport (@zmux_clone); ephemeral unless PinnedView is true
	PinnedView   bool   // user-pinned grouped viewport (@zmux_pinned_view)
	ViewRoot     string // root session for a pinned grouped viewport (@zmux_view_root)
	Managed      bool
	Workspace    string
	SessionLabel string
	SessionID    string
}

// ClientInfo represents an attached tmux client and its current view.
type ClientInfo struct {
	TTY          string // client_tty, e.g. /dev/pts/13
	SessionName  string // client_session / raw tmux session name
	SessionID    string // session_id, e.g. $28
	SessionGroup string // session_group, empty if ungrouped
	WindowID     string // window_id, e.g. @50
	WindowIndex  int
	WindowName   string
	PaneID       string // current pane for this client, e.g. %139
	PID          int    // client_pid
	ControlMode  bool   // client_control_mode
	TermName     string // client_termname, e.g. xterm-256color
	TermFeatures string // client_termfeatures, comma-delimited resolved features
	Flags        string // client_flags, comma-delimited flags such as attached,focused,UTF-8
}

// Window represents a tmux window.
type Window struct {
	Index  int
	Name   string
	Active bool
	Dir    string
	// Label is the @zmux_label overlay — a stable name zmux assigns that
	// survives tmux's automatic-rename (which retitles the window to the
	// running process). Empty when unset.
	Label string
}

// Pane represents a tmux pane within a window.
type Pane struct {
	Session     string // session_name
	ID          string // opaque tmux pane id, e.g. %57
	Index       int
	WindowIndex int    // which window this pane belongs to
	WindowName  string // window_name (the tab-like window identity)
	Active      bool
	Command     string // pane_current_command
	PID         int    // pane_pid
	Dir         string // pane_current_path
	Width       int
	Height      int
	Title       string // pane_title
}

// ProcessStats holds aggregated CPU/memory statistics for a process tree.
type ProcessStats struct {
	CPU    float64 // total CPU % (process + children)
	MemMB  float64 // total RSS in MB
	Uptime string  // etime from ps
}
