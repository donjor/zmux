package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// TestDefaultRootEntryOutsideTmuxAlwaysPicker is the regression guard for the
// bare-`zmux` entry: outside tmux it must open the picker even with no live
// sessions. Plan 032 wrongly routed the no-session case to the sessionless
// dashboard; that dashboard is the attach-fallback surface only
// (session_fallback.go), not the explicit invocation.
func TestDefaultRootEntryOutsideTmuxAlwaysPicker(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false

	mock.Sessions = nil // cold start / fresh reboot: no live sessions
	if got := defaultRootEntry(app); got != entryPicker {
		t.Fatalf("defaultRootEntry outside tmux, no sessions = %v; want entryPicker", got)
	}

	mock.Sessions = []tmux.Session{{Name: "dev"}} // live session present
	if got := defaultRootEntry(app); got != entryPicker {
		t.Fatalf("defaultRootEntry outside tmux, live session = %v; want entryPicker", got)
	}
}

func TestDefaultRootEntryInsideTmuxIsDashboardPopup(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = true
	if got := defaultRootEntry(app); got != entryDashboardPopup {
		t.Fatalf("defaultRootEntry inside tmux = %v; want entryDashboardPopup", got)
	}
}
