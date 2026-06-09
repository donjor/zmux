package cli

import (
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
)

// findWindow returns the window in session matching name by its stable
// @zmux_label overlay first, then its live name — or nil if none match.
//
// tmux's automatic-rename retitles a window to its running process
// (e.g. "server" → "node"), so matching on the live name alone silently fails
// once a long-running command starts. The label, which zmux sets and tmux's
// rename never touches, bridges that gap.
//
// A label match is preferred over a name match across the whole window list,
// so a labeled tab always wins over a different tab that merely shares the
// mutable, auto-renamed live name.
func findWindow(app *apppkg.App, session, name string) *tmux.Window {
	windows, err := app.Runner.ListWindows(session)
	if err != nil {
		return nil
	}
	for i := range windows {
		if windows[i].Label == name {
			return &windows[i]
		}
	}
	for i := range windows {
		if windows[i].Name == name {
			return &windows[i]
		}
	}
	return nil
}
