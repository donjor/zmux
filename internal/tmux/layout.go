package tmux

// Shared split-orientation toggle definition. The prefix+s keybind
// (GenerateConf) and the palette "Toggle split orientation" action both flip the
// active window between an even left-right and an even top-bottom split; these
// constants are the one place the predicate and the two target layouts live, so
// the keybind and the palette action can't drift apart.
const (
	// OrientHorizontalMatch is a tmux format predicate that is non-empty when the
	// active window's layout is a left-right (horizontal) split: tmux nests a
	// left-right split as a `{...}` group in window_layout. Used with `if -F` /
	// `if-shell -F` to select the opposite layout.
	OrientHorizontalMatch = `#{m:*{*,#{window_layout}}`
	// LayoutEvenVertical stacks panes top-to-bottom; LayoutEvenHorizontal places
	// them side-by-side. Nested/multi-pane layouts flatten to the even spread —
	// the documented v1 ceiling.
	LayoutEvenVertical   = "even-vertical"
	LayoutEvenHorizontal = "even-horizontal"
)
