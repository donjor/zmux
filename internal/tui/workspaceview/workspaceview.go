// Package workspaceview builds the enriched workspace+session view models
// shared by the picker and the dashboard. It is a data-adapter leaf: it
// depends only on the session and workspace domain packages, never on TUI
// rendering code, so both surfaces can import it without a cycle.
package workspaceview

import (
	"sort"
	"time"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
)

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

		for _, wsSess := range ws.Sessions {
			// Check for the session itself and any grouped clones (e.g., dev-b).
			root := session.RootName(wsSess.TmuxName)
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
