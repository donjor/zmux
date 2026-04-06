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
}

// Window represents a tmux window.
type Window struct {
	Index  int
	Name   string
	Active bool
	Dir    string
}

// Pane represents a tmux pane within a window.
type Pane struct {
	Index       int
	WindowIndex int    // which window this pane belongs to
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
