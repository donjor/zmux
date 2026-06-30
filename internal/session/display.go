package session

import "strings"

// LocalDisplayName returns the workspace-local label for a managed session.
// Raw tmux names remain the target; this is only for user-facing text.
func LocalDisplayName(s SessionInfo) string {
	base := s.Label
	if base == "" {
		base = s.Name
	}
	if s.PinnedView {
		if suffix := pinnedViewSuffix(s); suffix != "" {
			return base + " · view " + suffix
		}
		return base + " · view"
	}
	return base
}

func pinnedViewSuffix(s SessionInfo) string {
	root := s.ViewRoot
	if root == "" {
		root = RootName(s.Name)
	}
	prefix := root + "__clone_"
	if strings.HasPrefix(s.Name, prefix) {
		return strings.TrimPrefix(s.Name, prefix)
	}
	prefix = root + "-"
	if strings.HasPrefix(s.Name, prefix) {
		return strings.TrimPrefix(s.Name, prefix)
	}
	return ""
}

// QualifiedDisplayName returns the user-facing workspace/session address for a
// managed session. Flat lists use this to avoid leaking generated tmux names
// while keeping duplicate local labels unambiguous.
func QualifiedDisplayName(s SessionInfo) string {
	if s.Workspace != "" && s.Label != "" {
		base := s.Workspace + "/" + s.Label
		if s.PinnedView {
			if suffix := pinnedViewSuffix(s); suffix != "" {
				return base + " · view " + suffix
			}
			return base + " · view"
		}
		return base
	}
	return LocalDisplayName(s)
}
