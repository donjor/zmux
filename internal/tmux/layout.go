package tmux

import "fmt"

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

// swapPaneTarget maps a direction to tmux's relative pane target token
// ('{left-of}' …), matching the prefix+Shift+Arrow swap binds.
func swapPaneTarget(d SplitDirection) string {
	switch d {
	case SplitLeft:
		return "{left-of}"
	case SplitRight:
		return "{right-of}"
	case SplitUp:
		return "{up-of}"
	case SplitDown:
		return "{down-of}"
	}
	return ""
}

// selectPaneFlag maps a direction to select-pane's directional flag (-L …).
func selectPaneFlag(d SplitDirection) string {
	switch d {
	case SplitLeft:
		return "-L"
	case SplitRight:
		return "-R"
	case SplitUp:
		return "-U"
	case SplitDown:
		return "-D"
	}
	return ""
}

// NextWindow / PreviousWindow move to the adjacent tab in the current session.
func (c *Client) NextWindow() error     { return c.runSilent("next-window") }
func (c *Client) PreviousWindow() error { return c.runSilent("previous-window") }

// ReorderWindow swaps the current tab with its neighbor (delta -1 left, +1 right)
// and follows it, matching the prefix+</> reorder binds.
func (c *Client) ReorderWindow(delta int) error {
	t := fmt.Sprintf("%+d", delta) // "-1" / "+1"
	if err := c.runSilent("swap-window", "-t", t); err != nil {
		return err
	}
	return c.runSilent("select-window", "-t", t)
}

// SwapPane swaps the active pane with its neighbor in the given direction
// (swap-pane -t '{left-of}'), matching the prefix+Shift+Arrow binds.
func (c *Client) SwapPane(dir SplitDirection) error {
	target := swapPaneTarget(dir)
	if target == "" {
		return fmt.Errorf("swap pane: invalid direction %q", dir)
	}
	return c.runSilent("swap-pane", "-t", target)
}

// FocusPane moves focus to the adjacent pane (select-pane -L/-R/-U/-D), matching
// the Alt+Shift+Arrow focus binds.
func (c *Client) FocusPane(dir SplitDirection) error {
	flag := selectPaneFlag(dir)
	if flag == "" {
		return fmt.Errorf("focus pane: invalid direction %q", dir)
	}
	return c.runSilent("select-pane", flag)
}

// EqualizeLayout spreads the active window's panes evenly (select-layout -E).
func (c *Client) EqualizeLayout() error { return c.runSilent("select-layout", "-E") }

// ToggleOrientation flips the active window between an even left-right and an
// even top-bottom split. It runs the same if-shell predicate the prefix+s
// keybind emits (OrientHorizontalMatch) so the two can't diverge.
func (c *Client) ToggleOrientation() error {
	return c.runSilent("if-shell", "-F", OrientHorizontalMatch,
		"select-layout "+LayoutEvenVertical,
		"select-layout "+LayoutEvenHorizontal)
}
