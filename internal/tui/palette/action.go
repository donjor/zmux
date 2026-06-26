// Package palette implements the VS Code-style command palette for zmux.
// It provides a fuzzy-searchable action launcher that can execute tmux
// operations and transition to dashboard tabs.
package palette

// ActionKind classifies what an action does.
type ActionKind int

const (
	// ActionExec performs a tmux/config operation (switch session, set theme, etc.).
	ActionExec ActionKind = iota
	// ActionOpenDashboard opens a specific dashboard tab.
	ActionOpenDashboard
)

// Action describes a single executable palette entry.
type Action struct {
	ID       string     // unique identifier, e.g. "session:switch:dev"
	Group    string     // display group, e.g. "Sessions", "Themes"
	Title    string     // display title, e.g. "Switch to dev"
	Subtitle string     // optional secondary text
	Hint     string     // right-aligned hint text (e.g. hotkey)
	Keywords []string   // additional terms for fuzzy matching
	Kind     ActionKind // exec vs open-dashboard
	Payload  any        // action-specific data

	// Covers names the canonical actions.Spec ID this row is the palette
	// surface of (e.g. "tab.hide", "dashboard"). Empty for rows that aren't a
	// registry action's canonical surface (per-session switch/kill, themes,
	// bar presets). The coverage gate derives dynamic/open-surface coverage
	// from these tags on real emitted rows, so declaring coverage requires
	// actually producing the row — a hand-kept list can't rubber-stamp it.
	Covers string
}

// searchText returns the combined text used for fuzzy matching.
func (a Action) searchText() string {
	parts := []string{a.Title}
	if a.Subtitle != "" {
		parts = append(parts, a.Subtitle)
	}
	if a.Group != "" {
		parts = append(parts, a.Group)
	}
	parts = append(parts, a.Keywords...)
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}
