package tabs

// Per-tab overlay wrappers for the Session & Workspace tab. The actual
// rename and confirm renderers live in shared_overlay.go (shared with the
// Workspaces tab) — only the create-overlay's label is tab-specific.

// renderRenameOverlay renders the inline rename input prompt. Used for
// workspace / session / window renames.
func (t *CurrentTab) renderRenameOverlay() string {
	kind := ""
	if t.rename != nil {
		kind = t.rename.kind
	}
	return renderRenameOverlayShared(t.styles, kind, t.renameInput)
}

// renderCreateOverlay renders the "new session in workspace" input prompt.
func (t *CurrentTab) renderCreateOverlay() string {
	prompt := t.styles.Accent.Render("  new session in " + t.wsName + " ▸ ")
	return prompt + t.createInput.View() + "\n\n"
}

// renderConfirmOverlay renders the y/N kill prompt. step 1 is the normal
// confirmation; step 2 is the red "this will detach you" warning shown
// when killing an attached workspace.
func (t *CurrentTab) renderConfirmOverlay(step int) string {
	return renderConfirmOverlayShared(t.styles, t.confirm, step)
}
