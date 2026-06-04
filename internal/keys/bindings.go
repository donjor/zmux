package keys

// Tmux-owned bindings. These are the canonical definitions consumed by
// internal/tmux/conf.go (to emit `bind-key` lines), by the help surfaces, and
// by the generated docs/keybindings.md. Keys are written exactly as tmux
// expects them so conf.go can emit Key verbatim.
//
// Changing a Key here changes the generated tmux config, the help output, and
// the docs together — that is the point of the registry.
var (
	// Popups (prefix).
	Dashboard    = Binding{Action: "dashboard", Key: "Space", Help: "Open zmux dashboard", Category: CatPopups, Context: Prefix}
	Palette      = Binding{Action: "palette", Key: "p", Help: "Open command palette", Category: CatPopups, Context: Prefix}
	Help         = Binding{Action: "help", Key: "?", Help: "Show help", Category: CatPopups, Context: Prefix}
	ScratchShell = Binding{Action: "scratch.shell", Key: "!", Help: "Throwaway $SHELL popup (cwd from active pane)", Category: CatPopups, Context: Prefix}

	// Tabs (prefix).
	NewTab          = Binding{Action: "new", Key: "c", Help: "New tab", Category: CatTabs, Context: Prefix}
	TabNext         = Binding{Action: "tab.next", Key: "n", Help: "Next tab", Category: CatTabs, Context: Prefix}
	TabPrev         = Binding{Action: "tab.prev", Key: "N", Help: "Previous tab", Category: CatTabs, Context: Prefix}
	TabReorderLeft  = Binding{Action: "reorder.left", Key: "<", Help: "Move tab left", Category: CatTabs, Context: Prefix}
	TabReorderRight = Binding{Action: "reorder.right", Key: ">", Help: "Move tab right", Category: CatTabs, Context: Prefix}
	TabKill         = Binding{Action: "kill", Key: "x", Help: "Close tab (with confirm)", Category: CatTabs, Context: Prefix}
	LabelTab        = Binding{Action: "label.tab", Key: ".", Help: "Set stable tab label (blank clears)", Category: CatTabs, Context: Prefix}

	// Sessions (prefix).
	RenameSession = Binding{Action: "rename", Key: ",", Help: "Rename session", Category: CatSessions, Context: Prefix}
	NewSession    = Binding{Action: "session.new", Key: "C", Help: "New session in current workspace", Category: CatSessions, Context: Prefix}
	SessionPicker = Binding{Action: "session.picker", Key: "w", Aliases: []string{"s"}, Help: "Workspace + session picker", Category: CatSessions, Context: Prefix}
	SessionPrev   = Binding{Action: "session.prev", Key: "[", Help: "Previous session in workspace", Category: CatSessions, Context: Prefix}
	SessionNext   = Binding{Action: "session.next", Key: "]", Help: "Next session in workspace", Category: CatSessions, Context: Prefix}

	// Panes & general (prefix).
	PaneRespawn = Binding{Action: "pane.respawn", Key: "R", Help: "Respawn stopped/dead pane", Category: CatPanes, Context: Prefix}
	CopyModeKey = Binding{Action: "copy.mode", Key: "v", Help: "Enter vi copy mode", Category: CatGeneral, Context: Prefix}
	Paste       = Binding{Action: "paste", Key: "P", Help: "Paste buffer", Category: CatGeneral, Context: Prefix}
	Reload      = Binding{Action: "reload", Key: "r", Help: "Reload zmux config (zmux apply)", Category: CatGeneral, Context: Prefix}

	// No-prefix (instant) bindings. Dynamic families carry a range label in Key;
	// conf.go expands them.
	TabGoto         = Binding{Action: "tab.goto.N", Key: "M-1..9", Help: "Switch to tab N directly", Category: CatTabs, Context: NoPrefix}
	TabSwitch       = Binding{Action: "tab.switch", Key: "M-`", Help: "Session + tab switcher popup", Category: CatTabs, Context: NoPrefix}
	WorkspaceSwitch = Binding{Action: "workspace.switch", Key: "M-w", Help: "Workspace switcher popup", Category: CatSessions, Context: NoPrefix}
	PaneFocusL      = Binding{Action: "pane.focus.left", Key: "M-S-Left", Help: "Focus pane left", Category: CatPanes, Context: NoPrefix}
	PaneFocusR      = Binding{Action: "pane.focus.right", Key: "M-S-Right", Help: "Focus pane right", Category: CatPanes, Context: NoPrefix}
	PaneFocusU      = Binding{Action: "pane.focus.up", Key: "M-S-Up", Help: "Focus pane up", Category: CatPanes, Context: NoPrefix}
	PaneFocusD      = Binding{Action: "pane.focus.down", Key: "M-S-Down", Help: "Focus pane down", Category: CatPanes, Context: NoPrefix}

	// Copy mode (copy-mode-vi). Yank is bound by the clipboard integration
	// (see ClipboardBinding); it is documented here but not emitted from this set.
	CopyBeginSelection = Binding{Action: "copy.begin", Key: "v", Help: "Begin selection", Category: CatCopyMode, Context: CopyMode}
	CopyRectangle      = Binding{Action: "copy.rectangle", Key: "C-v", Help: "Toggle rectangle selection", Category: CatCopyMode, Context: CopyMode}
	CopyYank           = Binding{Action: "copy.yank", Key: "y", Help: "Yank selection to clipboard", Category: CatCopyMode, Context: CopyMode}
	CopySearchForward  = Binding{Action: "copy.search.forward", Key: "/", Help: "Search forward", Category: CatCopyMode, Context: CopyMode}
	CopySearchBackward = Binding{Action: "copy.search.backward", Key: "?", Help: "Search backward", Category: CatCopyMode, Context: CopyMode}
	CopyCancel         = Binding{Action: "copy.cancel", Key: "Escape", Help: "Cancel copy mode", Category: CatCopyMode, Context: CopyMode}
)

// PrefixBindings lists the prefix-table bindings in help/doc render order.
var PrefixBindings = []Binding{
	Dashboard, Palette, ScratchShell, Help,
	NewTab, TabNext, TabPrev, TabReorderLeft, TabReorderRight, TabKill, LabelTab,
	SessionPicker, SessionPrev, SessionNext, RenameSession, NewSession,
	PaneRespawn, CopyModeKey, Paste, Reload,
}

// NoPrefixBindings lists the instant (root-table) bindings in render order.
var NoPrefixBindings = []Binding{
	TabGoto, TabSwitch, WorkspaceSwitch, PaneFocusL, PaneFocusR, PaneFocusU, PaneFocusD,
}

// CopyModeBindings lists the copy-mode-vi bindings in render order.
var CopyModeBindings = []Binding{
	CopyBeginSelection, CopyRectangle, CopyYank, CopyCancel, CopySearchForward, CopySearchBackward,
}

// All returns every tmux-owned binding in render order (prefix, no-prefix,
// copy-mode). Used by the docs generator and golden check.
func All() []Binding {
	out := make([]Binding, 0, len(PrefixBindings)+len(NoPrefixBindings)+len(CopyModeBindings))
	out = append(out, PrefixBindings...)
	out = append(out, NoPrefixBindings...)
	out = append(out, CopyModeBindings...)
	return out
}
