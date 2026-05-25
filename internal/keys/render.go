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
