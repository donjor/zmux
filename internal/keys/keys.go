// Package keys is the single source of truth for zmux keybindings.
//
// It models the bindings zmux generates and owns — the tmux prefix table,
// no-prefix (root) table, and copy-mode — plus the component-local keys used by
// the TUI surfaces for documentation. The tmux-owned bindings drive three
// consumers that historically drifted apart:
//
//   - internal/tmux/conf.go    — emits the actual `bind-key` lines
//   - `zmux help` + dashboard  — render the live keybinding reference
//   - docs/keybindings.md       — generated via `zmux keys gen` (CI golden check)
//
// Component-local TUI keys (picker/tabpicker cursor nav, etc.) remain defined in
// their own packages; this registry documents them but does not own them at
// runtime.
package keys

// Context identifies where a binding is active.
type Context string

const (
	// Prefix bindings live in tmux's prefix table (pressed after the prefix key).
	Prefix Context = "prefix"
	// NoPrefix bindings are bound with `-n` in tmux's root table (instant).
	NoPrefix Context = "no-prefix"
	// CopyMode bindings live in the copy-mode-vi table.
	CopyMode Context = "copy-mode"
	// Inherited bindings are tmux defaults zmux deliberately keeps (documented,
	// not generated).
	Inherited Context = "inherited"
)

// Category groups bindings for help and documentation rendering.
type Category string

const (
	CatPopups   Category = "Popups"
	CatTabs     Category = "Tabs"
	CatSessions Category = "Sessions"
	CatPanes    Category = "Panes"
	CatGeneral  Category = "General"
	CatCopyMode Category = "Copy mode"
)

// Binding is one canonical, documented keybinding.
//
// Key holds the key label exactly as tmux expects it for generated bindings
// ("Space", "p", "<", "M-`", "M-S-Left"), so conf.go can emit it verbatim. For
// dynamic families (Alt+1-9) Key carries the human range label and the generator
// in conf.go expands it.
type Binding struct {
	// Action is the canonical dotted name, e.g. "dashboard", "tab.next".
	Action string
	// Key is the key label (tmux syntax for generated bindings).
	Key string
	// Aliases are additional keys bound to the same action (e.g. session.picker
	// is bound to both "w" and "s"). conf.go emits a bind for each; help/docs
	// render the primary Key.
	Aliases []string
	// Help is a short human-readable description.
	Help string
	// Category groups the binding in help/docs output.
	Category Category
	// Context is where the binding applies.
	Context Context
}
