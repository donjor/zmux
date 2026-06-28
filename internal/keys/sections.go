package keys

// KeySection is a titled block of keybindings for the help surfaces. The section
// SET — which registry slices appear, in what order, under what title — is
// defined here once so `zmux help` and the dashboard Help tab can't drift on
// which keybinding groups they show. Only their styling differs; both render
// these same sections, so a new binding (or a new slice) surfaces in both
// without editing render code.
type KeySection struct {
	Title    string
	Bindings []Binding
}

// DashboardHelpSections is the keybinding-section set for the dashboard Help tab.
// The prefix table is zmux's own bindings (not tmux's), so it is titled plainly
// with the prefix noted once; only the genuinely inherited tmux defaults carry
// the "from tmux" label.
func DashboardHelpSections() []KeySection {
	return []KeySection{
		{Title: "Prefix Keys (Ctrl+Space)", Bindings: PrefixBindings},
		{Title: "Inherited from tmux", Bindings: InheritedBindings},
		{Title: "No-Prefix Keys", Bindings: NoPrefixBindings},
		{Title: "Copy Mode (vi keys)", Bindings: CopyModeBindings},
	}
}
