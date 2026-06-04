package cli

import (
	"fmt"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
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

// labelTab tags a window with zmux's stable @zmux_label overlay so a later
// name-based lookup survives tmux's automatic-rename. Best-effort.
func labelTab(app *apppkg.App, target, name string) {
	_ = app.Runner.SetWindowOption(target, tablabel.Option, name)
	_ = app.Runner.SetWindowOption(target, tablabel.SourceOption, tablabel.SourcePane)
}

// resolveWindowTarget maps a zmux tab name to a stable tmux target
// (session:index) via findWindow, read-only. Falls back to session:name when
// no window matches — so a genuinely-missing tab surfaces tmux's own error and
// callers that create-on-miss keep working. Use this for read-only commands
// (watch); it never mutates window state.
func resolveWindowTarget(app *apppkg.App, session, name string) string {
	if w := findWindow(app, session, name); w != nil {
		return fmt.Sprintf("%s:%d", session, w.Index)
	}
	return fmt.Sprintf("%s:%s", session, name)
}

// resolveTabForMutation is resolveWindowTarget for commands that will mutate the
// target (send/type). On a match by live name against an unlabeled window it
// first *claims* the name as the window's @zmux_label — so the tab stays
// addressable as <name> even after tmux auto-renames it. This is what makes a
// `send X C-c` (which returns the foreground to a shell and drifts the live
// name) keep X reachable for the follow-up `run -n X` / `watch X`.
//
// A window returned by findWindow with an empty Label necessarily matched by
// live name (the label pass requires a non-empty label), so Label=="" is the
// exact "matched by name, not yet claimed" signal. Existing labels are never
// clobbered. Read-only commands must use resolveWindowTarget instead.
func resolveTabForMutation(app *apppkg.App, session, name string) string {
	w := findWindow(app, session, name)
	if w == nil {
		return fmt.Sprintf("%s:%s", session, name)
	}
	target := fmt.Sprintf("%s:%d", session, w.Index)
	if w.Label == "" {
		labelTab(app, target, name)
	}
	return target
}
