package picker

import (
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/sahilm/fuzzy"
)

// parseQuery splits a raw input string into workspace and session parts.
// Rules:
//   - No space: entire string is workspaceQuery, sessionQuery is ""
//   - First space: everything before is workspaceQuery, after is sessionQuery
func parseQuery(raw string) (workspaceQuery, sessionQuery string) {
	idx := strings.IndexByte(raw, ' ')
	if idx < 0 {
		return raw, ""
	}
	return raw[:idx], raw[idx+1:]
}

// matchWorkspaces returns workspaces matching the query (fuzzy).
// Empty query returns all workspaces with cleared MatchedIndexes.
func matchWorkspaces(query string, workspaces []workspaceview.WorkspaceViewModel) []workspaceview.WorkspaceViewModel {
	if query == "" {
		// Clear any stale MatchedIndexes.
		result := make([]workspaceview.WorkspaceViewModel, len(workspaces))
		for i, ws := range workspaces {
			ws.MatchedIndexes = nil
			result[i] = ws
		}
		return result
	}
	names := make([]string, len(workspaces))
	for i, ws := range workspaces {
		names[i] = ws.Name
	}
	matches := fuzzy.Find(query, names)
	result := make([]workspaceview.WorkspaceViewModel, len(matches))
	for i, m := range matches {
		ws := workspaces[m.Index]
		ws.MatchedIndexes = m.MatchedIndexes
		result[i] = ws
	}
	return result
}

// matchSessions returns sessions matching the query (fuzzy).
// Empty query returns all sessions.
func matchSessions(query string, sessions []session.SessionInfo) []session.SessionInfo {
	if query == "" {
		return sessions
	}
	names := make([]string, len(sessions))
	for i, s := range sessions {
		names[i] = s.Name
	}
	matches := fuzzy.Find(query, names)
	result := make([]session.SessionInfo, len(matches))
	for i, m := range matches {
		result[i] = sessions[m.Index]
	}
	return result
}
