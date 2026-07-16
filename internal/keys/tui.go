package keys

import "charm.land/bubbles/v2/key"

// Cross-surface TUI keymap.
//
// These are the runtime Bubble Tea bindings shared by zmux's list/scroll TUI
// surfaces (dashboard tabs, palette, help viewer, colour picker). They are built
// ONCE at package init and matched in Update paths via key.Matches(msg, …),
// replacing the historical anti-pattern of rebuilding a throwaway
// key.NewBinding(key.WithKeys(…)) on every key event (idiom C).
//
// Only actions whose spelling is identical across surfaces live here. Keys with
// per-surface variants — the ctrl+j/ctrl+k scroll aliases in the help viewer and
// palette, space-to-toggle in bar/settings, ctrl+c-to-quit, and the colour
// picker's hue arrows — stay as component-local declared keymaps in their own
// packages; folding them here would either duplicate a spelling or silently
// change one surface's behaviour.
//
// Unlike the tmux-owned and dashboard-shell Binding tables above, these are NOT
// emitted into tmux.conf and NOT rendered in docs/reference/keybindings.md; they
// are component-internal navigation, documented by each surface's own help line.
var (
	// TUIListUp / TUIListDown move a list or scroll cursor by one row. `k`/`j`
	// are the vi aliases shared by every list surface.
	TUIListUp   = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	TUIListDown = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))

	// TUIListTop / TUIListBottom jump to the first/last row (vi g/G).
	TUIListTop    = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top"))
	TUIListBottom = key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom"))

	// TUIConfirm is the bare Enter activation shared by surfaces that do not bind
	// an extra activation key. Surfaces that also toggle on Space (bar, settings)
	// keep their own composite binding.
	TUIConfirm = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm"))

	// TUICancel is the bare Esc dismissal shared by surfaces without an extra
	// quit alias. The help viewer, which also quits on ctrl+c, keeps its own.
	TUICancel = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))

	// TUIFilter opens fuzzy filtering.
	TUIFilter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))
)
