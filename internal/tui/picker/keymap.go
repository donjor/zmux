package picker

import "charm.land/bubbles/v2/key"

type keymap struct {
	Quit    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Filter  key.Binding
	Up      key.Binding
	Down    key.Binding
	New     key.Binding
	Delete  key.Binding
	Rename  key.Binding
	Cleanup key.Binding
	MoveTab key.Binding
	Preview key.Binding
}

// Keys defines the default keybindings for TUI components.
var Keys = keymap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "attach"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	New: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new session"),
	),
	Delete: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "kill"),
	),
	Rename: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "rename"),
	),
	Cleanup: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "cleanup tmp"),
	),
	MoveTab: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "move tab"),
	),
	Preview: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "preview"),
	),
}
