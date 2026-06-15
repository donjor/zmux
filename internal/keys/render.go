package keys

import "strings"

// Humanize returns a human-facing key label for help and docs, translating
// tmux modifier syntax (M-, C-, S-) into the conventional Alt/Ctrl/Shift form
// and arrow names into glyphs.
//
//	"M-S-Left" -> "Alt+Shift+←"
//	"M-`"      -> "Alt+`"
//	"M-1..9"   -> "Alt+1-9"
//	"C-v"      -> "Ctrl+v"
func (b Binding) Humanize() string {
	return humanizeKey(b.Key)
}

func humanizeKey(k string) string {
	r := strings.NewReplacer(
		"M-S-", "Alt+Shift+",
		"M-", "Alt+",
		"C-", "Ctrl+",
		"S-", "Shift+",
		"Left", "←",
		"Right", "→",
		"Up", "↑",
		"Down", "↓",
		"1..9", "1-9",
	)
	return r.Replace(k)
}

// Matches reports whether key activates this binding, checking the primary Key
// and any Aliases by exact match. Dashboard bindings store Bubble Tea key
// strings ("c", "enter", "up", "ctrl+c"), so the dashboard TUI can route a
// tea.KeyMsg.String() straight through this without redefining the letters.
func (b Binding) Matches(key string) bool {
	if key == b.Key {
		return true
	}
	for _, alias := range b.Aliases {
		if key == alias {
			return true
		}
	}
	return false
}

// DisplayKeys renders the primary key plus any aliases joined with " / " using
// the raw labels. The component-local TUI tables (dashboard) store Bubble Tea
// key strings that are already human-facing ("up / k", "esc / ctrl+c"), so they
// render verbatim rather than through Humanize (which translates tmux syntax).
func (b Binding) DisplayKeys() string {
	if len(b.Aliases) == 0 {
		return b.Key
	}
	return b.Key + " / " + strings.Join(b.Aliases, " / ")
}

// DisplayKey returns the key label decorated for its context: prefix-table and
// inherited tmux bindings are reached via the prefix, so they are shown as
// "prefix + <key>"; instant and copy-mode bindings stand alone.
func (b Binding) DisplayKey() string {
	switch b.Context {
	case Prefix, Inherited:
		return "prefix + " + b.Humanize()
	default:
		return b.Humanize()
	}
}

// ByCategory groups bindings by their Category, preserving the input order
// within each category and returning categories in first-seen order.
func ByCategory(bindings []Binding) ([]Category, map[Category][]Binding) {
	order := []Category{}
	groups := map[Category][]Binding{}
	for _, b := range bindings {
		if _, ok := groups[b.Category]; !ok {
			order = append(order, b.Category)
		}
		groups[b.Category] = append(groups[b.Category], b)
	}
	return order, groups
}
