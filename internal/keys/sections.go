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

// TmuxHelpSections is the keybinding-section set for `zmux help`: the tmux prefix
// table, the instant no-prefix table, and the inherited tmux defaults. Copy-mode
// keys are intentionally omitted — the CLI help stays tmux-control-focused.
func TmuxHelpSections() []KeySection {
	return []KeySection{
		{Title: "tmux Prefix Keys (Ctrl+Space)", Bindings: PrefixBindings},
		{Title: "No-Prefix Keys", Bindings: NoPrefixBindings},
		{Title: "Inherited tmux Defaults (Ctrl+Space)", Bindings: InheritedBindings},
	}
}

// DashboardHelpSections is the keybinding-section set for the dashboard Help tab.
// Same registry source as the CLI help, plus copy-mode. Titles match the rendered
// reference so consuming this leaves the dashboard output stable.
func DashboardHelpSections() []KeySection {
	return []KeySection{
		{Title: "tmux Prefix Keys (Ctrl+Space)", Bindings: PrefixBindings},
		{Title: "Inherited tmux Defaults (Ctrl+Space)", Bindings: InheritedBindings},
		{Title: "No-Prefix Keys", Bindings: NoPrefixBindings},
		{Title: "Copy Mode (vi keys)", Bindings: CopyModeBindings},
	}
}
