package session

// LocalDisplayName returns the workspace-local label for a managed session.
// Raw tmux names remain the target; this is only for user-facing text.
func LocalDisplayName(s SessionInfo) string {
	if s.Label != "" {
		return s.Label
	}
	return s.Name
}

// QualifiedDisplayName returns the user-facing workspace/session address for a
// managed session. Flat lists use this to avoid leaking generated tmux names
// while keeping duplicate local labels unambiguous.
func QualifiedDisplayName(s SessionInfo) string {
	if s.Workspace != "" && s.Label != "" {
		return s.Workspace + "/" + s.Label
	}
	return LocalDisplayName(s)
}
