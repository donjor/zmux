package tmux

import "time"

// Session represents a tmux session with enriched metadata.
type Session struct {
	Name     string
	Windows  int
	Attached bool
	Activity time.Time
	Dir      string
}

// Window represents a tmux window.
type Window struct {
	Index  int
	Name   string
	Active bool
	Dir    string
}
