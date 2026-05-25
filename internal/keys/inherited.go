package keys

// Inherited bindings are tmux defaults that zmux deliberately keeps rather than
// reinventing. zmux does not emit `bind-key` lines for these — they are
// documented here so the help surfaces and generated docs can present the full
// supported workflow from a single registry. All are reached via the prefix.
var (
	Detach       = Binding{Action: "detach", Key: "d", Help: "Detach from session", Category: CatGeneral, Context: Inherited}
	PaneFocusDir = Binding{Action: "pane.focus.direction", Key: "Arrow", Help: "Focus pane in direction", Category: CatPanes, Context: Inherited}
	PaneNext     = Binding{Action: "pane.next", Key: "o", Help: "Focus next pane", Category: CatPanes, Context: Inherited}
	PaneLast     = Binding{Action: "pane.last", Key: ";", Help: "Focus previously active pane", Category: CatPanes, Context: Inherited}
	PaneIDs      = Binding{Action: "pane.ids", Key: "q", Help: "Show pane numbers/ids", Category: CatPanes, Context: Inherited}
	PaneZoom     = Binding{Action: "pane.zoom", Key: "z", Help: "Toggle pane zoom", Category: CatPanes, Context: Inherited}
	PaneResizeS  = Binding{Action: "pane.resize.small", Key: "C-Arrow", Help: "Resize pane by one cell", Category: CatPanes, Context: Inherited}
	PaneResizeL  = Binding{Action: "pane.resize.large", Key: "M-Arrow", Help: "Resize pane by five cells", Category: CatPanes, Context: Inherited}
	PaneSplitR   = Binding{Action: "pane.split.right", Key: "%", Help: "Split pane right", Category: CatPanes, Context: Inherited}
	PaneSplitD   = Binding{Action: "pane.split.down", Key: "\"", Help: "Split pane below", Category: CatPanes, Context: Inherited}
)

// InheritedBindings lists the documented tmux defaults in render order.
var InheritedBindings = []Binding{
	Detach,
	PaneFocusDir, PaneNext, PaneLast, PaneIDs, PaneZoom,
	PaneResizeS, PaneResizeL, PaneSplitR, PaneSplitD,
}
