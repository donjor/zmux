// Package tui — workspace picker model.
//
// The picker is split across several files for readability:
//
//   - picker.go         — Messages, [PickerModel] struct, constructor, setters,
//     Init, top-level Update (message dispatcher).
//   - picker_update.go  — Key handling: handleKey + handleNormalKey.
//   - picker_actions.go — What happens when the user selects something:
//     handleEnter and its handle*Enter delegates, plus
//     the workspace/session delete mutation.
//   - picker_outline.go — Filter, outline-row construction, label helpers.
//   - picker_view.go    — View(), top-level view() + header rendering.
//   - picker_view_list.go — List/row rendering, including external source rows.
//   - picker_view_help.go — Help footer and ghost-prompt rendering.
//   - picker_search.go  — Fuzzy-match helpers (separately maintained).
//   - picker_types.go   — Modes, confirm-targets, view-models, key bindings.
package picker

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// ── Messages ──

type refreshSessionsMsg struct {
	sessions []session.SessionInfo
	err      error
}

type windowsMsg struct {
	windows map[string][]tmux.Window
}

type catalogMsg struct {
	catalog *source.Catalog
}

type workspacesLoadedMsg struct {
	workspaces []workspaceview.WorkspaceViewModel
}

// ── Model ──

// PickerModel is the bubbletea model for the outside-tmux workspace picker.
type PickerModel struct {
	runner tmux.Runner
	styles styles.Styles

	// State (drives all rendering).
	state pickerState

	// Data.
	workspaces         []workspaceview.WorkspaceViewModel // all workspaces (enriched, MRU sorted)
	filteredWorkspaces []workspaceview.WorkspaceViewModel // after visibility + fuzzy filter

	// Outline tree — owns the cursor and expansion state. Rebuilt via
	// buildOutline() whenever filter/data/expansion changes.
	tree *outline.Tree

	// Search input.
	input textinput.Model

	// Dimensions.
	width  int
	height int

	// Mode overlays (modal states on top of normal).
	mode pickerMode

	// Snapshot of the row targeted when confirm-delete was entered. Used
	// so the second-step "this will detach clients" prompt and the actual
	// mutation both act on what the user was looking at, not the current
	// row (which could change if a background refresh lands).
	confirm *pickerConfirmTarget

	// Stable row ID to land the cursor on after a delete reload. Captured at
	// delete-commit via tree.NeighborID() (before the row is removed) and
	// consumed by the next buildOutline so the cursor lands on the next
	// cleanup row instead of snapping to the workspace header.
	postDeleteJump string

	// Window names per session (cached).
	windows map[string][]tmux.Window

	// External sources.
	catalog *source.Catalog

	// Workspace data loader.
	wsLoader workspaceview.WorkspaceDataLoader

	// Workspace mutator for delete operations.
	wsStore WorkspaceMutator

	// Result state (read after quit).
	Result   PickerResult
	Quitting bool
}

// NewPickerModel creates a new workspace picker model.
func NewPickerModel(runner tmux.Runner, styles styles.Styles) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "search or create..."
	ti.CharLimit = 64
	ti.ShowSuggestions = true
	// Customize prompt/completion styles to match our theme.
	ti.Prompt = ""
	// Non-zero default width; refined on WindowSizeMsg. Guards the first
	// render (before any size msg) against the bubbles v2 width-0 placeholder
	// bug that renders only the first rune.
	ti.SetWidth(40)
	tiStyles := ti.Styles()
	tiStyles.Focused.Suggestion = styles.Dim
	tiStyles.Blurred.Suggestion = styles.Dim
	ti.SetStyles(tiStyles)
	ti.Focus()

	return PickerModel{
		runner:  runner,
		styles:  styles,
		input:   ti,
		tree:    outline.NewTree(),
		windows: make(map[string][]tmux.Window),
	}
}

// SetWorkspaceDataLoader sets the function used to load workspace view models.
func (m *PickerModel) SetWorkspaceDataLoader(loader workspaceview.WorkspaceDataLoader) {
	m.wsLoader = loader
}

// SetWorkspaceStore sets the store used for workspace mutations (delete).
func (m *PickerModel) SetWorkspaceStore(store WorkspaceMutator) {
	m.wsStore = store
}

// ── Init ──

func (m PickerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.loadSessions(),
		textinput.Blink,
		tea.Cmd(func() tea.Msg {
			cat, _ := source.Discover(m.runner.Endpoint())
			return catalogMsg{catalog: cat}
		}),
	}
	if m.wsLoader != nil {
		loader := m.wsLoader
		cmds = append(cmds, func() tea.Msg {
			return workspacesLoadedMsg{workspaces: loader()}
		})
	}
	return tea.Batch(cmds...)
}

func (m PickerModel) loadSessions() tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		sessions, err := session.ListSessions(runner)
		return refreshSessionsMsg{sessions: sessions, err: err}
	}
}

// ── Update ──

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate width to the text inputs. bubbles v2 renders only the
		// first placeholder rune when Width()==0 (the "stray s" bug), so the
		// inputs must always carry a non-zero width.
		inputWidth := m.width - 6
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.input.SetWidth(inputWidth)
		return m, nil

	case refreshSessionsMsg:
		// Sessions loaded — fetch windows.
		if msg.err == nil && len(msg.sessions) > 0 {
			runner := m.runner
			sessions := msg.sessions
			return m, tea.Cmd(func() tea.Msg {
				wins := make(map[string][]tmux.Window)
				for _, s := range sessions {
					w, err := runner.ListWindows(s.Name)
					if err == nil {
						wins[s.Name] = w
					}
				}
				return windowsMsg{windows: wins}
			})
		}
		return m, nil

	case windowsMsg:
		m.windows = msg.windows
		return m, nil

	case catalogMsg:
		m.catalog = msg.catalog
		m.applyFilter()
		return m, nil

	case workspacesLoadedMsg:
		m.workspaces = msg.workspaces

		// Push workspace names into the textinput's suggestion list for
		// ghost completion.
		names := make([]string, 0, len(m.workspaces))
		for _, ws := range m.workspaces {
			if !ws.IsPseudo {
				names = append(names, ws.Name)
			}
		}
		m.input.SetSuggestions(names)

		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to active text input.
	if m.mode == modeNormal {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.onInputChanged()
		return m, cmd
	}
	return m, nil
}
