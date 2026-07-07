package tabs

// Per-tab overlay wrappers for the Workspaces tab. The actual rename and
// confirm renderers live in shared_overlay.go (shared with the Session &
// Workspace tab) — only the create-overlay's label is tab-specific.

// renderRenameOverlay renders the inline rename input prompt. Used during
// sessionsModeRename for both workspace and session renames.
func (t *SessionsTab) renderRenameOverlay() string {
	kind := ""
	if t.rename != nil {
		kind = t.rename.kind
	}
	return renderRenameOverlayShared(t.styles, kind, t.renameInput)
}

// renderCreateOverlay renders the inline create-workspace input prompt.
func (t *SessionsTab) renderCreateOverlay() string {
	prompt := t.styles.Accent.Render("  new workspace ▸ ")
	return prompt + t.createInput.View() + "\n\n"
}

// renderConfirmOverlay renders the y/N kill prompt. step 1 is the normal
// confirmation; step 2 is the red "this will detach you" warning shown
// when killing an attached workspace.
func (t *SessionsTab) renderConfirmOverlay(step int) string {
	return renderConfirmOverlayShared(t.styles, t.confirm, step)
}

func (t *SessionsTab) renderSearchOverlay() string {
	return renderSearchOverlayShared(t.styles, t.searchInput)
}
