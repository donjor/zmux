// Package wspicker is a lightweight workspace switcher popup (Alt+w).
//
// It is a thin standalone tea.Model wrapper over the shared
// internal/tui/workspacelist component: it wires only OnSelect (the popup's
// single purpose is to switch), and owns the standalone-program concerns the
// embeddable component deliberately leaves to its host — window sizing,
// AltScreen, and quit semantics. Enter on a row switches to that workspace's
// last-active session (the same behaviour as the `zmux <workspace>` shorthand).
package wspicker

import (
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspacelist"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// Result holds the outcome of a workspace pick.
type Result struct {
	Action    string // "switch", or "" if the user cancelled
	Workspace string
}

// Loader returns the current workspaces to display. It is invoked once at
// Init; the popup is short-lived so live refresh is not warranted here.
type Loader func() []workspaceview.WorkspaceViewModel

// workspaceSelectedMsg is emitted by the embedded list's OnSelect hook. The
// hook returns a message rather than mutating wrapper state directly:
// workspacelist invokes OnSelect from inside its own (value-receiver) Update,
// so a closure writing to the wrapper would mutate the wrong copy.
type workspaceSelectedMsg struct{ name string }

type loadedMsg struct {
	workspaces []workspaceview.WorkspaceViewModel
}

// Model is a thin wrapper over workspacelist.Model. p.Run() returns it by
// value; callers read Result after Quitting.
type Model struct {
	loader Loader
	list   workspacelist.Model
	styles styles.Styles
	width  int
	height int

	Result   Result
	Quitting bool
}

// NewModel constructs the picker. The loader is called from Init.
func NewModel(loader Loader, sty styles.Styles) Model {
	list := workspacelist.New(workspacelist.Config{
		// Parity with the old standalone picker: show every workspace,
		// including ones with no live sessions (the component hides them by
		// default). Pseudo-workspaces are filtered by the caller's loader.
		ShowEmpty: true,
		Styles:    sty,
		OnSelect: func(ws workspaceview.WorkspaceViewModel) tea.Cmd {
			name := ws.Name
			return func() tea.Msg { return workspaceSelectedMsg{name: name} }
		},
		// OnCreate/OnRename/OnDelete intentionally nil — the switcher is
		// select-only, so those keys + footer entries auto-hide.
	})
	return Model{loader: loader, list: list, styles: sty}
}

func (m Model) Init() tea.Cmd {
	loader := m.loader
	return tea.Batch(
		func() tea.Msg { return loadedMsg{workspaces: loader()} },
		m.list.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case loadedMsg:
		m.list, _ = m.list.Update(workspacelist.ReloadMsg{Workspaces: msg.workspaces})
		return m, nil

	case workspaceSelectedMsg:
		m.Result = Result{Action: "switch", Workspace: msg.name}
		m.Quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		// The embedded list treats esc/ctrl+c as "clear a non-empty filter,
		// else bubble out" (returns nil). An empty-filter esc/ctrl+c is the
		// wrapper's cue to close the switcher.
		if s := msg.String(); s == "esc" || s == "ctrl+c" {
			if m.list.FilterText() == "" {
				m.Quitting = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m Model) view() string {
	if m.Quitting {
		return ""
	}
	// Title chrome on top of the component's search input + list + footer.
	return "  " + m.styles.Title.Bold(true).Render("workspaces") + "\n\n" + m.list.View()
}
