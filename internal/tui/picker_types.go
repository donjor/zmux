package tui

import (
	"sort"
	"time"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/workspace"
)

// pickerMode tracks overlay modes (modal states on top of the flat list).
type pickerMode int

const (
	modeNormal                pickerMode = iota // unified: type to filter/name, enter to attach/create
	modeConfirmDelete                           // first-step y/N to confirm delete
	modeConfirmDeleteAttached                   // second-step y/N for workspaces with live clients
	modeTemplateSelect                          // picking a template
	modeTemplateName                            // text input for template session name
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
	showEmpty      bool   // show workspaces with 0 live sessions?
}

// PickerResult holds the outcome of the picker interaction.
type PickerResult struct {
	Action    string // "attach", "hijack", "new", "template", "overmind-connect", "external-attach", "workspace-create", "workspace-focus", ""
	Session   string // session name to attach
	Name      string // name for new session (may be "" for auto tmp-N)
	Template  string // template name if action is "template"
	Workspace string // workspace name for workspace-level actions

	// External source fields (overmind-connect, external-attach).
	ExternalSource *source.Source // source owning the session/process
}

// WorkspaceViewModel enriches a workspace with live session data.
// Used by both the picker and dashboard for rendering.
type WorkspaceViewModel struct {
	workspace.Workspace
	LiveSessionCount int                   // number of live tmux sessions
	HasAttached      bool                  // any session currently attached?
	LastActivity     time.Time             // most recent activity across sessions
	LiveSessions     []session.SessionInfo // live sessions (sorted)
	IsPseudo         bool                  // true for "temporary", "external" pseudo-workspaces
	MatchedIndexes   []int                 // indexes of matched chars in Name (fuzzy search)
}

// WorkspaceDataLoader returns enriched workspace view models.
// Called on init and after mutations.
type WorkspaceDataLoader func() []WorkspaceViewModel

// WorkspaceMutator is the minimal interface for workspace mutations the
// picker needs (e.g., for delete). Implemented by *workspace.Store.
type WorkspaceMutator interface {
	DeleteWorkspace(name string) error
}

// BuildWorkspaceViewModels enriches workspace metadata with live session data.
// Groups tmp-N sessions under a "temporary" pseudo-workspace.
func BuildWorkspaceViewModels(
	workspaces []workspace.Workspace,
	liveSessions []session.SessionInfo,
) []WorkspaceViewModel {
	// Build lookup: session name → SessionInfo.
	sessionMap := make(map[string]session.SessionInfo, len(liveSessions))
	for _, s := range liveSessions {
		sessionMap[s.Name] = s
	}

	// Track which live sessions are claimed by a workspace.
	claimed := make(map[string]bool)

	var models []WorkspaceViewModel

	for _, ws := range workspaces {
		vm := WorkspaceViewModel{
			Workspace: ws,
		}

		for _, sessName := range ws.Sessions {
			// Check for the session itself and any grouped clones (e.g., dev-b).
			root := session.RootName(sessName)
			for _, ls := range liveSessions {
				if session.RootName(ls.Name) == root {
					vm.LiveSessions = append(vm.LiveSessions, ls)
					claimed[ls.Name] = true
					if ls.Attached {
						vm.HasAttached = true
					}
					if ls.Activity.After(vm.LastActivity) {
						vm.LastActivity = ls.Activity
					}
				}
			}
		}
		vm.LiveSessionCount = len(vm.LiveSessions)
		models = append(models, vm)
	}

	// Sort by last activity (MRU), then alphabetical.
	sort.Slice(models, func(i, j int) bool {
		if !models[i].LastActivity.Equal(models[j].LastActivity) {
			return models[i].LastActivity.After(models[j].LastActivity)
		}
		return models[i].Name < models[j].Name
	})

	// Collect unclaimed tmp-N sessions into "temporary" pseudo-workspace.
	var tmpSessions []session.SessionInfo
	var tmpLastActivity time.Time
	for _, ls := range liveSessions {
		if claimed[ls.Name] {
			continue
		}
		if ls.IsTmp {
			tmpSessions = append(tmpSessions, ls)
			if ls.Activity.After(tmpLastActivity) {
				tmpLastActivity = ls.Activity
			}
		}
	}

	if len(tmpSessions) > 0 {
		models = append(models, WorkspaceViewModel{
			Workspace: workspace.Workspace{
				Name: "temporary",
			},
			LiveSessionCount: len(tmpSessions),
			LastActivity:     tmpLastActivity,
			LiveSessions:     tmpSessions,
			IsPseudo:         true,
		})
	}

	return models
}
