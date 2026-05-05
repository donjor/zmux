package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
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
	workspaces []WorkspaceViewModel
}

// ── Model ──

// PickerModel is the bubbletea model for the outside-tmux workspace picker.
type PickerModel struct {
	runner tmux.Runner
	styles Styles

	// State (drives all rendering).
	state pickerState

	// Data.
	workspaces         []WorkspaceViewModel // all workspaces (enriched, MRU sorted)
	filteredWorkspaces []WorkspaceViewModel // after visibility + fuzzy filter

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

	// Window names per session (cached).
	windows map[string][]tmux.Window

	// Templates.
	templates        []session.Template
	templateCursor   int
	nameInput        textinput.Model
	selectedTemplate string

	// External sources.
	catalog *source.Catalog

	// Workspace data loader.
	wsLoader WorkspaceDataLoader

	// Workspace mutator for delete operations.
	wsStore WorkspaceMutator

	// Result state (read after quit).
	Result   PickerResult
	Quitting bool
}

// NewPickerModel creates a new workspace picker model.
func NewPickerModel(runner tmux.Runner, styles Styles) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "search or create..."
	ti.CharLimit = 64
	ti.ShowSuggestions = true
	// Customize prompt/completion styles to match our theme.
	ti.Prompt = ""
	ti.CompletionStyle = styles.Dim
	ti.Focus()

	ni := textinput.New()
	ni.Placeholder = "session name"
	ni.CharLimit = 64

	return PickerModel{
		runner:    runner,
		styles:    styles,
		input:     ti,
		nameInput: ni,
		tree:      outline.NewTree(),
		windows:   make(map[string][]tmux.Window),
	}
}

// SetWorkspaceDataLoader sets the function used to load workspace view models.
func (m *PickerModel) SetWorkspaceDataLoader(loader WorkspaceDataLoader) {
	m.wsLoader = loader
}

// SetWorkspaceStore sets the store used for workspace mutations (delete).
func (m *PickerModel) SetWorkspaceStore(store WorkspaceMutator) {
	m.wsStore = store
}

// SetTemplates sets the available templates for the picker.
func (m *PickerModel) SetTemplates(templates []session.Template) {
	m.templates = templates
}

// ── Init ──

func (m PickerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.loadSessions(),
		textinput.Blink,
		tea.Cmd(func() tea.Msg {
			cat, _ := source.Discover()
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
	if m.mode == modeTemplateName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── Key handling ──

func (m PickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Confirm delete (first step) ──
	if m.mode == modeConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			// If we're killing a workspace with a live attached client,
			// route through the second-step confirm before running the
			// mutation — deleting an attached workspace from outside
			// tmux silently kills the user's only client, so we require
			// a second explicit y/N.
			if m.confirm != nil && m.confirm.kind == "workspace" && m.confirm.attached {
				m.mode = modeConfirmDeleteAttached
				return m, nil
			}
			m.applyConfirmedDelete()
			m.mode = modeNormal
			m.confirm = nil
			return m, m.reloadWorkspaces()
		default:
			m.mode = modeNormal
			m.confirm = nil
			return m, nil
		}
	}

	// ── Confirm delete (second step — attached workspace) ──
	if m.mode == modeConfirmDeleteAttached {
		switch msg.String() {
		case "y", "Y":
			m.applyConfirmedDelete()
			m.mode = modeNormal
			m.confirm = nil
			return m, m.reloadWorkspaces()
		default:
			m.mode = modeNormal
			m.confirm = nil
			return m, nil
		}
	}

	// ── Template select mode ──
	if m.mode == modeTemplateSelect {
		switch {
		case key.Matches(msg, Keys.Back), key.Matches(msg, Keys.Quit):
			m.mode = modeNormal
			m.input.Focus()
			return m, textinput.Blink
		case key.Matches(msg, Keys.Up):
			if m.templateCursor > 0 {
				m.templateCursor--
			}
			return m, nil
		case key.Matches(msg, Keys.Down):
			if m.templateCursor < len(m.templates)-1 {
				m.templateCursor++
			}
			return m, nil
		case key.Matches(msg, Keys.Enter):
			if m.templateCursor < len(m.templates) {
				tmpl := m.templates[m.templateCursor]
				m.selectedTemplate = tmpl.Name
				m.mode = modeTemplateName
				m.nameInput.SetValue(tmpl.Name)
				m.nameInput.Placeholder = "blank for " + tmpl.Name
				m.nameInput.Focus()
				return m, textinput.Blink
			}
			return m, nil
		}
		return m, nil
	}

	// ── Template name input mode ──
	if m.mode == modeTemplateName {
		switch {
		case key.Matches(msg, Keys.Back):
			m.mode = modeTemplateSelect
			m.nameInput.Blur()
			m.nameInput.SetValue("")
			return m, nil
		case key.Matches(msg, Keys.Enter):
			name := strings.TrimSpace(m.nameInput.Value())
			// Scope the template to the workspace the cursor is currently on.
			wsName := ""
			if row := m.tree.CurrentSelectable(); row != nil && row.Kind == outline.RowWorkspaceHeader {
				if ws, ok := outline.RowData[WorkspaceViewModel](row); ok && ws != nil && !ws.IsPseudo {
					wsName = ws.Name
				}
			}
			m.Result = PickerResult{
				Action:    "template",
				Name:      name,
				Template:  m.selectedTemplate,
				Workspace: wsName,
			}
			m.Quitting = true
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
	}

	// ── Normal mode ──
	return m.handleNormalKey(msg)
}

// handleNormalKey handles keys in the normal flat-list mode.
func (m PickerModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Quitting = true
		return m, tea.Quit

	case "up":
		m.tree.MoveUp()
		m.buildOutline()
		return m, nil

	case "down":
		m.tree.MoveDown()
		m.buildOutline()
		return m, nil

	case "enter":
		return m.handleEnter()

	case "tab":
		// Let bubbles textinput accept its suggestion. We forward the tab
		// key so AcceptSuggestion fires.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.onInputChanged()
		return m, cmd

	case "ctrl+t":
		if len(m.templates) == 0 {
			return m, nil
		}
		m.mode = modeTemplateSelect
		m.templateCursor = 0
		m.input.Blur()
		return m, nil

	case "ctrl+h":
		// Toggle visibility of empty workspaces.
		m.state.showEmpty = !m.state.showEmpty
		m.applyFilter()
		return m, nil

	case "ctrl+x":
		// Delete the current row (workspace or session). Snapshot the
		// target so the two-step confirm flow (for attached workspaces)
		// operates on a stable reference; the actual kill runs in
		// handleKey after y/N.
		row := m.tree.CurrentSelectable()
		if row == nil {
			return m, nil
		}
		switch row.Kind {
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[WorkspaceViewModel](row); ok && ws != nil && !ws.IsPseudo {
				m.confirm = &pickerConfirmTarget{
					kind:      "workspace",
					name:      ws.Name,
					attached:  ws.HasAttached,
					liveCount: len(ws.LiveSessions),
				}
				m.mode = modeConfirmDelete
			}
		case outline.RowSession:
			if s, ok := outline.RowData[session.SessionInfo](row); ok && s != nil {
				m.confirm = &pickerConfirmTarget{
					kind: "session",
					name: s.Name,
				}
				m.mode = modeConfirmDelete
			}
		}
		return m, nil

	case "esc":
		if m.state.workspaceQuery != "" || m.state.sessionQuery != "" {
			m.input.SetValue("")
			m.onInputChanged()
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick-select named session by 1-based index across the whole list.
		if m.state.workspaceQuery == "" && m.state.sessionQuery == "" {
			idx := int(msg.String()[0] - '0')
			count := 0
			for i := range m.tree.Rows {
				r := &m.tree.Rows[i]
				if r.Kind != outline.RowSession {
					continue
				}
				s, ok := outline.RowData[session.SessionInfo](r)
				if !ok || s == nil || s.IsTmp {
					continue
				}
				count++
				if count == idx {
					m.Result = PickerResult{Action: "attach", Session: s.Name}
					m.Quitting = true
					return m, tea.Quit
				}
			}
			return m, nil
		}
	}

	// All other keys go to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.onInputChanged()
	return m, cmd
}

// handleEnter dispatches based on the row under the cursor.
func (m PickerModel) handleEnter() (tea.Model, tea.Cmd) {
	row := m.tree.CurrentSelectable()
	if row == nil {
		return m, nil
	}

	switch row.Kind {
	case outline.RowTopAction:
		return m.handleTopActionEnter()
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[WorkspaceViewModel](row)
		return m.handleWorkspaceEnter(ws)
	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		return m.handleSessionEnter(s)
	case outline.RowExternalGroup:
		// Toggle expansion and rebuild.
		m.tree.ToggleExpand(row.ID)
		m.buildOutline()
		return m, nil
	case outline.RowExternalEntry:
		return m.handleExternalEntryEnter(row)
	}
	return m, nil
}

// handleExternalEntryEnter converts the row into an attach/connect result.
func (m PickerModel) handleExternalEntryEnter(row *outline.Row) (tea.Model, tea.Cmd) {
	entry, _ := outline.RowData[source.CatalogEntry](row)
	if entry == nil {
		return m, nil
	}
	src := externalEntrySource(m.catalog, row)
	if src == nil {
		return m, nil
	}
	srcCopy := *src
	if src.Kind == source.SourceOvermind {
		m.Result = PickerResult{
			Action:         "overmind-connect",
			Session:        entry.Session,
			ExternalSource: &srcCopy,
		}
	} else {
		m.Result = PickerResult{
			Action:         "external-attach",
			Session:        entry.Session,
			ExternalSource: &srcCopy,
		}
	}
	m.Quitting = true
	return m, tea.Quit
}

func (m PickerModel) handleTopActionEnter() (tea.Model, tea.Cmd) {
	wsName := strings.TrimSpace(m.state.workspaceQuery)
	if wsName == "" {
		// Empty input → create tmp session (no workspace).
		m.Result = PickerResult{Action: "new"}
		m.Quitting = true
		return m, tea.Quit
	}
	// Typed workspace name → create workspace. If a session name was
	// also typed (e.g. "myapp dev"), pass it through so root.go creates
	// that session instead of the default "main".
	sessName := strings.TrimSpace(m.state.sessionQuery)
	m.Result = PickerResult{
		Action:    "workspace-create",
		Workspace: wsName,
		Name:      sessName, // "" → root.go defaults to "main"
	}
	m.Quitting = true
	return m, tea.Quit
}

func (m PickerModel) handleWorkspaceEnter(ws *WorkspaceViewModel) (tea.Model, tea.Cmd) {
	if ws == nil {
		return m, nil
	}

	// Session query present → user typed "workspace session" in the
	// search bar. This means "create a new session named <session> in
	// this workspace", equivalent to `zmux new <ws> <session>`.
	if m.state.sessionQuery != "" {
		m.Result = PickerResult{
			Action:    "new",
			Name:      m.state.sessionQuery,
			Workspace: ws.Name,
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// No live sessions → create default session (named after workspace).
	if len(ws.LiveSessions) == 0 {
		m.Result = PickerResult{
			Action:    "new",
			Name:      ws.Name,
			Workspace: ws.Name,
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// Has sessions, no session query → drill into the workspace and require the
	// user to pick an explicit session row. This avoids surprising auto-attach to
	// last-active when a workspace contains multiple sessions.
	return m.drillIntoWorkspaceSessions(ws.Name), nil
}

func (m PickerModel) drillIntoWorkspaceSessions(workspaceName string) PickerModel {
	wsID := outline.WorkspaceID(workspaceName)
	m.buildOutlineWithFocus(wsID)
	for i := range m.tree.Rows {
		row := &m.tree.Rows[i]
		if row.Kind == outline.RowSession && row.ParentID == wsID {
			m.tree.Cursor = i
			return m
		}
	}
	_ = m.tree.JumpToID(wsID)
	return m
}

func (m PickerModel) handleSessionEnter(s *session.SessionInfo) (tea.Model, tea.Cmd) {
	if s == nil {
		return m, nil
	}
	m.Result = PickerResult{Action: "attach", Session: s.Name}
	m.Quitting = true
	return m, tea.Quit
}

// ── Input change handling ──

func (m *PickerModel) onInputChanged() {
	raw := m.input.Value()
	wsQuery, sessQuery := parseQuery(raw)

	queryChanged := wsQuery != m.state.workspaceQuery || sessQuery != m.state.sessionQuery
	m.state.workspaceQuery = wsQuery
	m.state.sessionQuery = sessQuery

	m.filteredWorkspaces = m.visibleWorkspaces(wsQuery)

	// Exact-workspace-match biases cursor to that workspace so when we
	// build rows below the workspace is automatically expanded. We remember
	// the target ID and jump to it after build.
	var pinTarget string
	if wsQuery != "" {
		for _, ws := range m.filteredWorkspaces {
			if ws.Name == wsQuery {
				pinTarget = outline.WorkspaceID(ws.Name)
				break
			}
		}
	}

	// Reset cursor on query change if we don't have a pin target.
	if queryChanged && pinTarget == "" {
		m.tree.Cursor = 0
	}

	if pinTarget != "" {
		// Build once, then jump to the target so expansion logic sees the
		// target as focused on the next build.
		m.buildOutlineWithFocus(pinTarget)
		m.tree.JumpToID(pinTarget)
		m.buildOutlineWithFocus(pinTarget)
	} else {
		m.buildOutline()
	}
}

// visibleWorkspaces returns workspaces respecting hide-empty + fuzzy filter.
// Searches always show all matches (including empty).
func (m *PickerModel) visibleWorkspaces(query string) []WorkspaceViewModel {
	if query != "" {
		return matchWorkspaces(query, m.workspaces)
	}
	if m.state.showEmpty {
		return m.workspaces
	}
	var visible []WorkspaceViewModel
	for _, ws := range m.workspaces {
		if ws.LiveSessionCount > 0 {
			visible = append(visible, ws)
		}
	}
	return visible
}

// applyFilter recomputes filteredWorkspaces and rebuilds the outline.
func (m *PickerModel) applyFilter() {
	m.filteredWorkspaces = m.visibleWorkspaces(m.state.workspaceQuery)
	m.buildOutline()
}

// buildOutline rebuilds the outline rows from current state and pushes
// them into the tree (which preserves cursor by ID).
func (m *PickerModel) buildOutline() {
	m.buildOutlineWithFocus("")
}

// buildOutlineWithFocus is like buildOutline but accepts an explicit
// workspace ID to treat as "focused" (expanded) during the build. Used by
// the exact-match flow in onInputChanged where the cursor hasn't moved yet
// but we want the matched workspace expanded.
func (m *PickerModel) buildOutlineWithFocus(forceFocusWS string) {
	rows := []outline.Row{
		{
			ID:         outline.TopActionID(),
			Kind:       outline.RowTopAction,
			Label:      topActionLabel(m.state.workspaceQuery),
			Selectable: true,
		},
	}

	hasSearch := m.state.workspaceQuery != "" || m.state.sessionQuery != ""

	// Which workspace is "focused" for expansion purposes?
	focusedWS := forceFocusWS
	if focusedWS == "" && !hasSearch {
		if row := m.tree.Current(); row != nil {
			switch row.Kind {
			case outline.RowWorkspaceHeader:
				focusedWS = row.ID
			case outline.RowSession:
				focusedWS = row.ParentID
			}
		}
	}

	for i := range m.filteredWorkspaces {
		ws := &m.filteredWorkspaces[i]
		wsID := outline.WorkspaceID(ws.Name)

		// Filter sessions by session query.
		sessions := ws.LiveSessions
		if m.state.sessionQuery != "" {
			sessions = matchSessions(m.state.sessionQuery, sessions)
		}

		// Expand when searching, or when this is the focused workspace.
		// Note: Expanded isn't set on the row because picker_view doesn't
		// render expansion chevrons — whether children follow is the
		// only signal it needs.
		expanded := hasSearch || wsID == focusedWS

		rows = append(rows, outline.Row{
			ID:         wsID,
			Kind:       outline.RowWorkspaceHeader,
			Label:      formatWorkspaceLabel(ws),
			Selectable: true,
			Attached:   ws.HasAttached,
			Data:       ws,
		})

		if expanded {
			for j := range sessions {
				s := sessions[j]
				rows = append(rows, outline.Row{
					ID:         outline.SessionID(s.Name),
					Kind:       outline.RowSession,
					Depth:      1,
					ParentID:   wsID,
					Label:      s.Name,
					Selectable: true,
					Attached:   s.Attached,
					Data:       &s,
				})
			}
		}
	}

	// External sources below the workspaces.
	rows = append(rows, buildExternalRows(m.catalog, m.tree)...)

	m.tree.SetRows(rows)
}

// topActionLabel returns the display label for the top action row based
// on the current search query.
func topActionLabel(query string) string {
	if query == "" {
		return "+ new tmp session"
	}
	return "+ create \"" + query + "\""
}

// formatWorkspaceLabel returns the display label for a workspace header row.
// Kept as a helper so the outline builder and views can stay in sync.
func formatWorkspaceLabel(ws *WorkspaceViewModel) string {
	if ws == nil {
		return ""
	}
	return ws.Name
}

func (m PickerModel) reloadWorkspaces() tea.Cmd {
	if m.wsLoader == nil {
		return nil
	}
	loader := m.wsLoader
	return func() tea.Msg {
		return workspacesLoadedMsg{workspaces: loader()}
	}
}

// applyConfirmedDelete runs the mutation described by m.confirm. Safe to
// call unconditionally — it no-ops on nil. For a workspace target it kills
// every live session it can find (by name — the snapshot taken at ctrl+x
// time may be stale by a few hundred ms, but we re-resolve from the live
// workspace set rather than trusting the snapshot's session list).
func (m PickerModel) applyConfirmedDelete() {
	if m.confirm == nil {
		return
	}
	switch m.confirm.kind {
	case "session":
		_ = session.Kill(m.runner, m.confirm.name)

	case "workspace":
		// Re-resolve the workspace from the live snapshot so we act on
		// whatever sessions are live *now*, not whatever was live when
		// ctrl+x was pressed. Fall back to the stored sessions field on
		// the workspace.Workspace itself if we can't find the live row.
		var killed bool
		for i := range m.workspaces {
			if m.workspaces[i].Name != m.confirm.name {
				continue
			}
			for _, s := range m.workspaces[i].LiveSessions {
				_ = session.Kill(m.runner, s.Name)
			}
			killed = true
			break
		}
		if !killed {
			// Workspace disappeared from view-model between ctrl+x and
			// the confirm — nothing live to kill, just drop the store
			// entry below.
		}
		if m.wsStore != nil {
			_ = m.wsStore.DeleteWorkspace(m.confirm.name)
		}
	}
}
