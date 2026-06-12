package workspace

import "sort"

func sortSessionRecords(s []WorkspaceSession) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].Label < s[j].Label
	})
}

func removeSessionRecord(slice []WorkspaceSession, key string) []WorkspaceSession {
	out := make([]WorkspaceSession, 0, len(slice))
	for _, s := range slice {
		if !sessionRecordMatches(s, key) {
			out = append(out, s)
		}
	}
	return out
}

func appendSessionRecordUnique(slice []WorkspaceSession, rec WorkspaceSession) []WorkspaceSession {
	if _, _, found := findSessionRecord(slice, rec.ID); found {
		return slice
	}
	if _, _, found := findSessionRecord(slice, rec.TmuxName); found {
		return slice
	}
	if _, _, found := findSessionRecord(slice, rec.Label); found {
		return slice
	}
	return append(slice, rec)
}

func sessionRecordMatches(s WorkspaceSession, key string) bool {
	return s.ID == key || s.Label == key || s.TmuxName == key || (s.LegacyTmuxName != "" && s.LegacyTmuxName == key)
}

func findSessionRecord(slice []WorkspaceSession, key string) (WorkspaceSession, int, bool) {
	for i, s := range slice {
		if sessionRecordMatches(s, key) {
			return s, i, true
		}
	}
	return WorkspaceSession{}, -1, false
}

func sessionLabels(slice []WorkspaceSession) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		out = append(out, s.Label)
	}
	return out
}

func sessionTargets(slice []WorkspaceSession) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		out = append(out, s.TmuxName)
	}
	return out
}
