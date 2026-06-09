package picker

import (
	"github.com/donjor/zmux/internal/source"
)

// pickerMode tracks overlay modes (modal states on top of the flat list).
type pickerMode int

const (
	modeNormal                pickerMode = iota // unified: type to filter/name, enter to attach/create
	modeConfirmDelete                           // first-step y/N to confirm delete
	modeConfirmDeleteAttached                   // second-step y/N for workspaces with live clients
)

// pickerConfirmTarget snapshots what's being deleted at the moment the user
// pressed ctrl+x, so the two-step confirm flow shows consistent copy even if
// the cursor is bumped by an async refresh.
type pickerConfirmTarget struct {
	kind      string // "workspace" or "session"
	name      string // workspace or session name
	attached  bool   // workspace with at least one attached client (triggers second-step)
	liveCount int    // number of live sessions in the workspace (for the prompt)
}

// pickerState holds the canonical state that drives all rendering. The
// cursor itself lives on the outline.Tree the picker owns; the rest of
// the state is search + flags.
type pickerState struct {
	workspaceQuery string // filter text before the space separator
	sessionQuery   string // filter text after the space separator
	showAll        bool   // expand the list past the default cap (reveal all workspaces)
}

// PickerResult holds the outcome of the picker interaction.
type PickerResult struct {
	Action    string // "attach", "hijack", "new", "overmind-connect", "external-attach", "workspace-create", "workspace-focus", ""
	Session   string // session name to attach
	Name      string // name for new session (may be "" for auto tmp-N)
	Workspace string // workspace name for workspace-level actions

	// External source fields (overmind-connect, external-attach).
	ExternalSource *source.Source // source owning the session/process
}

// WorkspaceMutator is the minimal interface for workspace mutations the
// picker needs (e.g., for delete). Implemented by *workspace.Store.
type WorkspaceMutator interface {
	DeleteWorkspace(name string) error
	RemoveSession(rootSession string) error
}
