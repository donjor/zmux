package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
)

// pickerMode tracks the current mode of the picker.
type pickerMode int

const (
	modeNormal         pickerMode = iota // unified: type to filter/name, enter to attach/create
	modeConfirmDelete                    // y/N to confirm delete
	modeTemplateSelect                   // picking a template
	modeTemplateName                     // text input for template session name
)

// PickerResult holds the outcome of the picker interaction.
type PickerResult struct {
	Action    string // "attach", "hijack", "new", "template", "overmind-connect", "external-attach", "workspace-focus", ""
	Session   string // session name to attach
	Name      string // name for new session (may be "" for auto tmp-N)
	Template  string // template name if action is "template"
	Workspace string // workspace name for workspace-level actions

	// External source fields (overmind-connect, external-attach).
	ExternalSource *source.Source // source owning the session/process
}

// pickerItem represents a single row in the grouped picker list.
// Workspace headers are selectable (Enter → focus workspace).
// Section headers ("sessions", "temporary") are non-selectable.
type pickerItem struct {
	IsHeader  bool
	Header    string               // section label
	Workspace string               // non-empty on workspace headers (selectable)
	Session   *session.SessionInfo // nil for headers
}

// windowsMsg carries fetched window data for all sessions.
type windowsMsg struct {
	windows map[string][]tmux.Window
}

// catalogMsg carries the result of async external source discovery.
type catalogMsg struct {
	catalog *source.Catalog
}

// workspaceStateMsg carries workspace state loaded asynchronously.
type workspaceStateMsg struct {
	state map[string]string // session → workspace
}

// PickerModel is the bubbletea model for the outside-tmux session picker.
type PickerModel struct {
	runner   tmux.Runner
	sessions []session.SessionInfo
	filtered []session.SessionInfo
	items    []pickerItem // grouped view built from filtered + workspaceState
	cursor   int
	lastQuery string // previous filter value — for detecting clears
	width    int
	height   int
	styles   Styles
	mode     pickerMode
	input    textinput.Model // unified input: filter + new session name
	err      error

	// Window names per session (cached).
	windows map[string][]tmux.Window

	// Workspace state: root session name → workspace name.
	workspaceState map[string]string

	// Templates available for selection.
	templates      []session.Template
	templateCursor int

	// Text input for template name entry.
	nameInput textinput.Model

	// Selected template (when in modeTemplateName).
	selectedTemplate string

	// Workspace loader (set via SetWorkspaceLoader).
	wsLoader WorkspaceLoader

	// External sources (catalog).
	catalog       *source.Catalog
	groupExpanded map[int]bool // expanded state per source group index

	// Result state (read after quit).
	Result   PickerResult
	Quitting bool
}

// WorkspaceLoader is a function that returns the current workspace state
// (session→workspace map). Used to decouple picker from workspace package.
type WorkspaceLoader func() map[string]string

// NewPickerModel creates a new session picker model.
func NewPickerModel(runner tmux.Runner, styles Styles) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "search or create..."
	ti.CharLimit = 64
	ti.Focus()

	ni := textinput.New()
	ni.Placeholder = "session name"
	ni.CharLimit = 64

	return PickerModel{
		runner:         runner,
		styles:         styles,
		input:          ti,
		nameInput:      ni,
		windows:        make(map[string][]tmux.Window),
		groupExpanded:  make(map[int]bool),
		workspaceState: make(map[string]string),
	}
}

// SetWorkspaceLoader sets the function used to load workspace state.
// Must be called before the picker's Init runs.
func (m *PickerModel) SetWorkspaceLoader(loader WorkspaceLoader) {
	m.wsLoader = loader
}

// SetTemplates sets the available templates for the picker.
func (m *PickerModel) SetTemplates(templates []session.Template) {
	m.templates = templates
}

// ── Messages ──

type refreshSessionsMsg struct {
	sessions []session.SessionInfo
	err      error
}

func refreshSessions(runner tmux.Runner) tea.Cmd {
	return func() tea.Msg {
		sessions, err := session.ListSessions(runner)
		return refreshSessionsMsg{sessions: sessions, err: err}
	}
}

func fetchWindows(runner tmux.Runner, sessions []session.SessionInfo) tea.Cmd {
	return func() tea.Msg {
		wins := make(map[string][]tmux.Window)
		for _, s := range sessions {
			w, err := runner.ListWindows(s.Name)
			if err == nil {
				wins[s.Name] = w
			}
		}
		return windowsMsg{windows: wins}
	}
}

// ── Init ──

func (m PickerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{refreshSessions(m.runner), textinput.Blink, discoverCatalog()}
	if m.wsLoader != nil {
		loader := m.wsLoader
		cmds = append(cmds, func() tea.Msg {
			return workspaceStateMsg{state: loader()}
		})
	}
	return tea.Batch(cmds...)
}

// discoverCatalog runs external source discovery asynchronously.
func discoverCatalog() tea.Cmd {
	return func() tea.Msg {
		cat, _ := source.Discover()
		return catalogMsg{catalog: cat}
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
		if msg.err != nil {
			m.sessions = nil
		} else {
			m.sessions = msg.sessions
		}
		m.err = nil
		m.applyFilter()
		return m, fetchWindows(m.runner, m.sessions)

	case windowsMsg:
		m.windows = msg.windows
		return m, nil

	case catalogMsg:
		m.catalog = msg.catalog
		// Default all groups to expanded.
		if m.catalog != nil {
			for i := range m.catalog.External {
				if _, ok := m.groupExpanded[i]; !ok {
					m.groupExpanded[i] = true
				}
			}
		}
		return m, nil

	case workspaceStateMsg:
		if msg.state != nil {
			m.workspaceState = msg.state
		}
		m.buildItems()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to active text input.
	if m.mode == modeNormal {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd
	}
	if m.mode == modeTemplateName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m PickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Confirm delete mode ──
	if m.mode == modeConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			itemIdx := m.cursor - 1
			if itemIdx >= 0 && itemIdx < len(m.items) && m.items[itemIdx].Session != nil {
				_ = session.Kill(m.runner, m.items[itemIdx].Session.Name)
			}
			m.mode = modeNormal
			return m, refreshSessions(m.runner)
		default:
			m.mode = modeNormal
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
			m.Result = PickerResult{Action: "template", Name: name, Template: m.selectedTemplate}
			m.Quitting = true
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
	}

	// ── Normal mode (unified input) ──
	query := m.input.Value()

	switch msg.String() {
	case "ctrl+c":
		m.Quitting = true
		return m, tea.Quit

	case "up":
		if m.cursor > 0 {
			m.cursor--
			m.skipHeaders(-1)
		}
		return m, nil

	case "down":
		total := m.totalItems()
		if m.cursor < total-1 {
			m.cursor++
			m.skipHeaders(1)
		}
		return m, nil

	case "enter":
		return m.handleEnter()

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick-select: digit N attaches to the Nth named (non-tmp) session.
		// Only when input is empty (otherwise it's a character).
		if m.input.Value() == "" {
			idx, _ := strconv.Atoi(msg.String())
			// Count only named (non-tmp, non-header) items.
			n := 0
			for _, item := range m.items {
				if item.IsHeader || item.Session == nil || item.Session.IsTmp {
					continue
				}
				n++
				if n == idx {
					m.Result = PickerResult{Action: "attach", Session: item.Session.Name}
					m.Quitting = true
					return m, tea.Quit
				}
			}
			return m, nil
		}

	case "ctrl+x":
		// Kill selected session (not the "+ new" entry, not headers).
		itemIdx := m.cursor - 1
		if itemIdx >= 0 && itemIdx < len(m.items) && !m.items[itemIdx].IsHeader && m.items[itemIdx].Session != nil {
			m.mode = modeConfirmDelete
		}
		return m, nil

	case "ctrl+h":
		// Hijack — forcefully attach, detaching other clients.
		itemIdx := m.cursor - 1
		if itemIdx >= 0 && itemIdx < len(m.items) && !m.items[itemIdx].IsHeader && m.items[itemIdx].Session != nil {
			s := m.items[itemIdx].Session
			if s.Attached {
				m.Result = PickerResult{Action: "hijack", Session: s.Name}
				m.Quitting = true
				return m, tea.Quit
			}
		}
		return m, nil

	case "ctrl+t":
		// Template mode.
		if len(m.templates) == 0 {
			m.Result = PickerResult{Action: "template"}
			m.Quitting = true
			return m, tea.Quit
		}
		m.mode = modeTemplateSelect
		m.templateCursor = 0
		m.input.Blur()
		return m, nil

	case "esc":
		// If there's text, clear it. If empty, quit.
		if query != "" {
			m.input.SetValue("")
			m.applyFilter()
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// For 'q' — only quit if input is empty (otherwise it's a character).
	if msg.String() == "q" && query == "" && len(m.sessions) == 0 {
		m.Quitting = true
		return m, tea.Quit
	}

	// All other keys go to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m PickerModel) handleEnter() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.input.Value())

	// Cursor 0 = "+ new session" entry.
	if m.cursor == 0 {
		// Create with typed name, or blank for tmp-N.
		m.Result = PickerResult{Action: "new", Name: query}
		m.Quitting = true
		return m, tea.Quit
	}

	// Cursor > 0 = item at items[cursor-1].
	itemIdx := m.cursor - 1
	if itemIdx < len(m.items) {
		item := m.items[itemIdx]
		if item.IsHeader && item.Workspace != "" {
			// Workspace header — focus workspace. Caller resolves last-active.
			m.Result = PickerResult{Action: "workspace-focus", Workspace: item.Workspace}
			m.Quitting = true
			return m, tea.Quit
		}
		if item.IsHeader {
			// Non-workspace headers (e.g., "other", "temporary") — non-selectable.
			return m, nil
		}
		if item.Session != nil {
			m.Result = PickerResult{Action: "attach", Session: item.Session.Name}
			m.Quitting = true
			return m, tea.Quit
		}
	}

	// Check if cursor is in external source area.
	if ext := m.externalItemAt(m.cursor); ext != nil {
		if ext.isHeader {
			// Toggle collapse/expand.
			m.groupExpanded[ext.groupIdx] = !m.groupExpanded[ext.groupIdx]
			return m, nil
		}
		// Entry row — connect or attach.
		srcCopy := ext.group.Source
		if ext.group.Source.Kind == source.SourceOvermind {
			m.Result = PickerResult{
				Action:         "overmind-connect",
				Session:        ext.entry.Session,
				ExternalSource: &srcCopy,
			}
		} else {
			m.Result = PickerResult{
				Action:         "external-attach",
				Session:        ext.entry.Session,
				ExternalSource: &srcCopy,
			}
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// Fallback.
	m.Result = PickerResult{Action: "new", Name: query}
	m.Quitting = true
	return m, tea.Quit
}

func (m *PickerModel) applyFilter() {
	query := m.input.Value()
	changed := query != m.lastQuery
	m.lastQuery = query

	if query == "" {
		m.filtered = m.sessions
	} else {
		names := make([]string, len(m.sessions))
		for i, s := range m.sessions {
			names[i] = s.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]session.SessionInfo, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.sessions[match.Index]
		}
	}

	m.buildItems()

	total := m.totalItems()
	if m.cursor >= total {
		m.cursor = max(0, total-1)
	}

	if !changed {
		return
	}

	// Query cleared — reset cursor to "+ new session".
	if query == "" {
		m.cursor = 0
		return
	}

	// Query matches existing sessions — point cursor at the first match
	// rather than staying on "+ Create".
	if len(m.filtered) > 0 && m.cursor == 0 {
		m.cursor = 1
		m.skipHeaders(1)
	}
}

// buildItems constructs the grouped item list from filtered sessions and
// workspace state. When no workspaces exist, items is a flat list matching
// filtered (no headers). When workspaces exist, items include section headers.
func (m *PickerModel) buildItems() {
	if len(m.workspaceState) == 0 {
		// No workspace state — flat list, no headers.
		m.items = make([]pickerItem, len(m.filtered))
		for i := range m.filtered {
			s := m.filtered[i]
			m.items[i] = pickerItem{Session: &s}
		}
		return
	}

	// Group sessions by workspace.
	type wsGroup struct {
		name     string
		sessions []session.SessionInfo
	}

	groups := make(map[string]*wsGroup)
	var order []string
	var untagged []session.SessionInfo
	var tmp []session.SessionInfo

	for i := range m.filtered {
		s := m.filtered[i]
		if s.IsTmp {
			tmp = append(tmp, s)
			continue
		}
		root := session.RootName(s.Name)
		ws, ok := m.workspaceState[root]
		if ok {
			g, exists := groups[ws]
			if !exists {
				g = &wsGroup{name: ws}
				groups[ws] = g
				order = append(order, ws)
			}
			g.sessions = append(g.sessions, s)
		} else {
			untagged = append(untagged, s)
		}
	}

	// Sort workspace names for stable display.
	sort.Strings(order)

	var items []pickerItem

	// Workspace groups (headers are selectable).
	for _, ws := range order {
		g := groups[ws]
		items = append(items, pickerItem{IsHeader: true, Header: ws, Workspace: ws})
		for i := range g.sessions {
			s := g.sessions[i]
			items = append(items, pickerItem{Session: &s})
		}
	}

	// Untagged sessions (header is NOT selectable — no Workspace field).
	if len(untagged) > 0 {
		items = append(items, pickerItem{IsHeader: true, Header: "other"})
		for i := range untagged {
			s := untagged[i]
			items = append(items, pickerItem{Session: &s})
		}
	}

	// Temporary sessions.
	if len(tmp) > 0 {
		items = append(items, pickerItem{IsHeader: true, Header: "temporary"})
		for i := range tmp {
			s := tmp[i]
			items = append(items, pickerItem{Session: &s})
		}
	}

	m.items = items
}

// itemSessionAt returns the session at the given item index (0-based within
// the items slice), or nil if it's a header.
func (m PickerModel) itemSessionAt(itemIdx int) *session.SessionInfo {
	if itemIdx < 0 || itemIdx >= len(m.items) {
		return nil
	}
	return m.items[itemIdx].Session
}

// isSkippableHeaderAt returns true if the item at cursor position is a
// non-selectable header (section headers like "other", "temporary").
// Workspace headers are selectable and NOT skipped.
// cursor is absolute (0=new, 1..=items, after=external).
func (m PickerModel) isHeaderAt(cursor int) bool {
	itemIdx := cursor - 1 // offset for "+ new session"
	if itemIdx < 0 || itemIdx >= len(m.items) {
		return false
	}
	item := m.items[itemIdx]
	// Workspace headers are selectable — don't skip.
	if item.IsHeader && item.Workspace != "" {
		return false
	}
	return item.IsHeader
}

// skipHeaders adjusts the cursor to skip header items in the given direction.
// dir is +1 (down) or -1 (up).
func (m *PickerModel) skipHeaders(dir int) {
	total := m.totalItems()
	for m.cursor >= 1 && m.cursor < 1+len(m.items) && m.isHeaderAt(m.cursor) {
		m.cursor += dir
		if m.cursor < 0 || m.cursor >= total {
			// Went past bounds — reverse.
			m.cursor -= dir
			break
		}
	}
}

// ── External source helpers ──

// externalItem describes a single item in the external source area.
type externalItem struct {
	isHeader bool               // true = group header, false = entry
	groupIdx int                // index into catalog.External
	group    *source.SourceGroup
	entry    source.CatalogEntry // valid only when !isHeader
}

// externalItemCount returns how many cursor slots the external area uses.
func (m PickerModel) externalItemCount() int {
	if m.catalog == nil || len(m.catalog.External) == 0 {
		return 0
	}
	count := 0
	for i, g := range m.catalog.External {
		count++ // header
		if m.groupExpanded[i] {
			count += len(g.Entries)
		}
	}
	return count
}

// totalItems returns the total number of cursor positions (including headers).
func (m PickerModel) totalItems() int {
	return 1 + len(m.items) + m.externalItemCount()
}

// externalItemAt returns the external item at the given absolute cursor
// position, or nil if the cursor is not in the external area.
func (m PickerModel) externalItemAt(cursor int) *externalItem {
	base := 1 + len(m.items) // start of external area
	if cursor < base {
		return nil
	}
	if m.catalog == nil {
		return nil
	}

	pos := base
	for i := range m.catalog.External {
		g := &m.catalog.External[i]
		if cursor == pos {
			return &externalItem{isHeader: true, groupIdx: i, group: g}
		}
		pos++
		if m.groupExpanded[i] {
			for j := range g.Entries {
				if cursor == pos {
					return &externalItem{groupIdx: i, group: g, entry: g.Entries[j]}
				}
				pos++
			}
		}
	}
	return nil
}

// isOnExternalItem returns true if the cursor is in the external area.
func (m PickerModel) isOnExternalItem() bool {
	return m.cursor >= 1+len(m.items) && m.externalItemAt(m.cursor) != nil
}

// ── View ──

// logo renders the zmux block-art banner (matches v0).
var logo = "" +
	"░█████████ ░█████████████  ░██    ░██ ░██    ░██\n" +
	"     ░███  ░██   ░██   ░██ ░██    ░██  ░██  ░██\n" +
	"   ░███    ░██   ░██   ░██ ░██    ░██   ░█████\n" +
	" ░███      ░██   ░██   ░██ ░██   ░███  ░██  ░██\n" +
	"░█████████ ░██   ░██   ░██  ░█████░██ ░██    ░██"

func (m PickerModel) View() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	hasSessions := len(m.sessions) > 0

	// ── Logo ──
	if !hasSessions {
		b.WriteString("\n")
		for _, line := range strings.Split(logo, "\n") {
			b.WriteString("  " + m.styles.Accent.Render(line) + "\n")
		}
		b.WriteString("  " + m.styles.Dim.Render(strings.Repeat("━", 56)) + "\n")
		b.WriteString("\n")
	} else {
		b.WriteString("\n  " + m.styles.Title.Bold(true).Render("zmux") + "\n")
		// Status line: session count + quick tip.
		statusParts := []string{
			fmt.Sprintf("%d sessions", len(m.sessions)),
		}
		statusParts = append(statusParts, "prefix: ctrl+space")
		b.WriteString("  " + m.styles.Dim.Render(strings.Join(statusParts, " \u2022 ")) + "\n\n")
	}

	// ── Mode-specific content ──
	switch m.mode {
	case modeTemplateSelect:
		b.WriteString(m.viewTemplateSelect())
	case modeTemplateName:
		b.WriteString(m.viewTemplateNameInput())
	default:
		b.WriteString(m.viewNormal())
	}

	// ── Delete confirmation ──
	itemIdx := m.cursor - 1
	if m.mode == modeConfirmDelete && itemIdx >= 0 && itemIdx < len(m.items) && m.items[itemIdx].Session != nil {
		b.WriteString("\n")
		name := m.items[itemIdx].Session.Name
		prompt := m.styles.Error.Render("  Delete ") +
			m.styles.Error.Bold(true).Render(name) +
			m.styles.Error.Render("? ") +
			m.styles.Dim.Render("(y/N)")
		b.WriteString(prompt + "\n")
	}

	// ── Help bar ──
	b.WriteString("\n")
	b.WriteString(m.viewHelp())

	// ── Live prompt — shows the equivalent CLI command ──
	sep := m.styles.Dim.Render("  " + strings.Repeat("━", 56))
	b.WriteString("\n\n" + sep + "\n\n")

	dir := "~"
	if cwd, err := os.Getwd(); err == nil {
		dir = shortenPath(cwd)
	}
	cmd := m.ghostCmd()

	dirStyle := m.styles.Muted
	chevron := m.styles.Accent.Render("❯")
	cmdStyle := m.styles.Normal
	b.WriteString("  " + dirStyle.Render(dir) + "  " + chevron + " " + cmdStyle.Render(cmd) + "\n")

	return b.String()
}

func (m PickerModel) viewNormal() string {
	var b strings.Builder
	query := m.input.Value()

	// ── Input ──
	prompt := m.styles.Accent.Render("  ▸ ")
	b.WriteString(prompt + m.input.View() + "\n")
	b.WriteString("\n")

	// ── "+ new session" entry (always first, cursor 0) ──
	newLabel := "+ new session"
	if query != "" {
		newLabel = "+ create \"" + query + "\""
	}
	if m.cursor == 0 {
		b.WriteString("  " + m.styles.Accent.Render("▸ ") + m.styles.Accent.Bold(true).Render(newLabel) + "\n")
	} else {
		b.WriteString("    " + m.styles.Muted.Render(newLabel) + "\n")
	}

	// ── Session list (grouped) ──
	if len(m.items) > 0 {
		b.WriteString("\n")
		b.WriteString(m.viewSessionList())
	} else if query != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Dim.Render("  no matching sessions") + "\n")
	}

	// ── External source groups ──
	if m.catalog != nil && len(m.catalog.External) > 0 {
		b.WriteString(m.viewExternalSources())
	}

	return b.String()
}

func (m PickerModel) viewSessionList() string {
	var b strings.Builder

	// Max name width for alignment.
	maxName := 0
	for _, item := range m.items {
		if item.Session != nil && len(item.Session.Name) > maxName {
			maxName = len(item.Session.Name)
		}
	}
	if maxName < 8 {
		maxName = 8
	}
	if maxName > 20 {
		maxName = 20
	}

	// Scroll window. listCursor is the cursor within items (0-based).
	listCursor := m.cursor - 1 // offset for "+ new session" entry
	listHeight := m.height - 14
	if listHeight < 5 {
		listHeight = 20
	}
	start := 0
	if listCursor >= listHeight {
		start = listCursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(m.items) {
		end = len(m.items)
	}

	// Track whether we need a named/tmp divider (flat mode, no workspace headers).
	hasHeaders := false
	for _, item := range m.items {
		if item.IsHeader {
			hasHeaders = true
			break
		}
	}
	shownDivider := false

	for i := start; i < end; i++ {
		item := m.items[i]
		if item.IsHeader {
			// Workspace / section header.
			b.WriteString("\n")
			b.WriteString("  " + m.styles.Dim.Render(item.Header) + "\n")
			continue
		}
		if item.Session != nil {
			// In flat mode (no workspace headers), show a divider before first tmp session.
			if !hasHeaders && item.Session.IsTmp && !shownDivider {
				// Check if there are named sessions for divider logic.
				for _, it := range m.items {
					if it.Session != nil && !it.Session.IsTmp {
						shownDivider = true
						b.WriteString("  " + m.styles.Dim.Render(strings.Repeat("\u2500", maxName+30)) + "\n")
						break
					}
				}
				if !shownDivider {
					shownDivider = true // no named sessions, skip divider
				}
			}
			b.WriteString(m.viewSessionEntry(i, *item.Session, maxName, listCursor))
		}
	}

	if start > 0 {
		b.WriteString(m.styles.Dim.Render("  \u2191 more") + "\n")
	}
	if end < len(m.items) {
		b.WriteString(m.styles.Dim.Render("  \u2193 more") + "\n")
	}

	return b.String()
}

func (m PickerModel) viewExternalSources() string {
	var b strings.Builder

	// Heavy divider before external section.
	b.WriteString("\n")
	b.WriteString("  " + m.styles.Dim.Render(strings.Repeat("\u2501", 40)) + "\n")

	pos := 1 + len(m.items) // first external cursor position
	for i, g := range m.catalog.External {
		selected := m.cursor == pos

		// Group header.
		arrow := "\u25bc" // expanded
		if !m.groupExpanded[i] {
			arrow = "\u25b6" // collapsed
		}

		kindLabel := string(g.Source.Kind)
		label := g.Source.Label
		countStr := fmt.Sprintf("(%d procs)", len(g.Entries))

		headerStyle := m.styles.Dim
		if selected {
			headerStyle = m.styles.Accent
		}

		cursor := "  "
		if selected {
			cursor = m.styles.Accent.Render("\u25b8 ")
		}

		healthBadge := ""
		if g.Source.Health == source.HealthDegraded {
			healthBadge = " " + m.styles.Error.Render("[degraded]")
		}

		b.WriteString("  " + cursor + headerStyle.Render(arrow+" "+kindLabel+": "+label) +
			"  " + m.styles.Dim.Render(countStr) + healthBadge + "\n")
		pos++

		// Entries (if expanded).
		if m.groupExpanded[i] {
			for _, entry := range g.Entries {
				entrySelected := m.cursor == pos

				icon := "\u25cb" // stopped
				iconStyle := m.styles.Dim
				if entry.Attached {
					icon = "\u25cf" // running
					iconStyle = m.styles.Info
				}
				if entrySelected {
					iconStyle = m.styles.Accent
				}

				nameStyle := m.styles.Normal
				if entrySelected {
					nameStyle = m.styles.Accent.Bold(true)
				}

				entryCursor := "    "
				if entrySelected {
					entryCursor = "  " + m.styles.Accent.Render("\u25b8 ")
				}

				statusLabel := m.styles.Dim.Render("stopped")
				if entry.Attached {
					statusLabel = m.styles.Info.Render("running")
				}

				b.WriteString("  " + entryCursor + iconStyle.Render(icon) + " " +
					nameStyle.Render(fmt.Sprintf("%-16s", entry.Session)) + " " +
					statusLabel + "\n")
				pos++
			}
		}
	}

	return b.String()
}

func (m PickerModel) viewSessionEntry(idx int, s session.SessionInfo, maxName int, listCursor int) string {
	selected := idx == listCursor

	// Icon.
	var icon string
	var iconStyle lipgloss.Style
	if s.Attached {
		icon = "●"
		iconStyle = m.styles.Info
	} else {
		icon = "○"
		iconStyle = m.styles.Dim
	}

	// Index number — named sessions get sequential 1-9 indices for quick-select.
	var indexStr string
	if !s.IsTmp {
		// Count how many named (non-tmp, non-header) sessions appear before this one.
		namedCount := 0
		for i := 0; i < idx; i++ {
			item := m.items[i]
			if !item.IsHeader && item.Session != nil && !item.Session.IsTmp {
				namedCount++
			}
		}
		num := namedCount + 1
		if num <= 9 {
			indexStr = m.styles.Dim.Render(fmt.Sprintf("%d", num))
		} else {
			indexStr = m.styles.Dim.Render(" ")
		}
	} else {
		indexStr = m.styles.Dim.Render(" ")
	}

	// Cursor.
	if selected {
		iconStyle = m.styles.Accent
	}
	cursor := " " + indexStr + " "
	if selected {
		cursor = m.styles.Accent.Render("▸") + indexStr + " "
	}
	iconStr := iconStyle.Render(icon)

	// Name + created age (together).
	nameStyle := m.styles.Normal.Bold(true)
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	if s.IsTmp && !selected {
		nameStyle = m.styles.Dim
	}
	paddedName := s.Name
	if len(paddedName) > maxName {
		paddedName = paddedName[:maxName]
	}
	paddedName = fmt.Sprintf("%-*s", maxName, paddedName)
	nameStr := nameStyle.Render(paddedName)

	// Created age right after name.
	createdStr := ""
	if !s.Created.IsZero() {
		createdStr = m.styles.Dim.Render(" " + session.HumanAge(s.Created))
	}

	// Window names — fixed width column.
	windowStr := ""
	if wins, ok := m.windows[s.Name]; ok && len(wins) > 0 {
		names := make([]string, 0, len(wins))
		for _, w := range wins {
			names = append(names, w.Name)
		}
		windowStr = "[" + strings.Join(names, ", ") + "]"
	} else {
		windowStr = fmt.Sprintf("%dw", s.Windows)
	}
	// Pad window string for alignment.
	if len(windowStr) < 20 {
		windowStr = fmt.Sprintf("%-20s", windowStr)
	}
	windowStr = m.styles.Dim.Render(windowStr)

	// Dir — fixed width column.
	dirStr := ""
	if s.Dir != "" {
		d := shortenPath(s.Dir)
		if len(d) > 20 {
			d = d[:20]
		}
		dirStr = fmt.Sprintf("%-20s", d)
	}
	dirStr = m.styles.Dim.Render(dirStr)

	// Last attached (end of line).
	lastActiveStr := ""
	if !s.LastAttached.IsZero() && !s.LastAttached.Equal(s.Created) {
		lastActiveStr = m.styles.Dim.Render(session.HumanAge(s.LastAttached) + " ago")
	}

	// Attached tag — shows client count when multiple viewports.
	attachedTag := ""
	if s.Attached {
		if s.AttachedClients > 1 {
			attachedTag = "  " + m.styles.Info.Render(fmt.Sprintf("attached ×%d", s.AttachedClients))
		} else {
			attachedTag = "  " + m.styles.Info.Render("attached")
		}
	}

	line := "  " + cursor + iconStr + " " + nameStr + createdStr + "  " + windowStr + "  " + dirStr
	if lastActiveStr != "" {
		line += "  " + lastActiveStr
	}
	line += attachedTag
	return line + "\n"
}

func (m PickerModel) viewTemplateSelect() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  Select Template")
	b.WriteString(label + "\n\n")

	if len(m.templates) == 0 {
		b.WriteString(m.styles.Muted.Render("  No templates available") + "\n")
		return b.String()
	}

	for i, tmpl := range m.templates {
		selected := i == m.templateCursor

		cursor := "  "
		if selected {
			cursor = m.styles.Accent.Render("▸ ")
		}

		nameStyle := m.styles.Normal.Bold(true)
		if selected {
			nameStyle = m.styles.Accent.Bold(true)
		}

		line := "  " + cursor + nameStyle.Render(tmpl.Name)
		if tmpl.Description != "" {
			line += "  " + m.styles.Dim.Render(tmpl.Description)
		}
		if len(tmpl.Windows) > 0 {
			winNames := make([]string, 0, len(tmpl.Windows))
			for _, w := range tmpl.Windows {
				winNames = append(winNames, w.Name)
			}
			line += "  " + m.styles.Dim.Render("["+strings.Join(winNames, ", ")+"]")
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + m.styles.Dim.Render("  enter:select  esc:cancel") + "\n")
	return b.String()
}

func (m PickerModel) viewTemplateNameInput() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  New from Template")
	tmplName := m.styles.Info.Render(m.selectedTemplate)
	b.WriteString(label + "  " + tmplName + "\n\n")

	prompt := m.styles.Accent.Render("  name ▸ ")
	b.WriteString(prompt + m.nameInput.View() + "\n")
	b.WriteString("\n" + m.styles.Dim.Render("  enter:create  esc:back") + "\n")

	return b.String()
}

func (m PickerModel) viewHelp() string {
	query := m.input.Value()

	switch m.mode {
	case modeConfirmDelete:
		return m.styles.Help.Render("  y:confirm  any:cancel")
	case modeTemplateSelect:
		return m.styles.Help.Render("  enter:select  esc:cancel")
	case modeTemplateName:
		return m.styles.Help.Render("  enter:create  esc:back")
	}

	// Normal mode — context-sensitive.
	parts := []string{}

	// Check if cursor is on external item.
	if ext := m.externalItemAt(m.cursor); ext != nil {
		if ext.isHeader {
			parts = append(parts, "enter:toggle")
		} else if ext.group.Source.Kind == source.SourceOvermind {
			parts = append(parts, "enter:connect")
		} else {
			parts = append(parts, "enter:attach")
		}
		if query != "" {
			parts = append(parts, "esc:clear")
		} else {
			parts = append(parts, "esc:quit")
		}
		return m.styles.Help.Render("  " + strings.Join(parts, "  "))
	}

	// Check if selected session is attached.
	selectedAttached := false
	if m.cursor > 0 {
		itemIdx := m.cursor - 1
		if itemIdx < len(m.items) && m.items[itemIdx].Session != nil {
			selectedAttached = m.items[itemIdx].Session.Attached
		}
	}

	if m.cursor == 0 {
		parts = append(parts, "enter:new")
	} else if selectedAttached {
		parts = append(parts, "enter:group")
		parts = append(parts, "ctrl+h:hijack")
	} else {
		parts = append(parts, "enter:attach")
	}

	parts = append(parts, "ctrl+t:template")

	if m.cursor > 0 {
		itemIdx := m.cursor - 1
		if itemIdx < len(m.items) && !m.items[itemIdx].IsHeader && m.items[itemIdx].Session != nil {
			parts = append(parts, "ctrl+x:kill")
		}
	}

	if query != "" {
		parts = append(parts, "esc:clear")
	} else {
		parts = append(parts, "esc:quit")
	}

	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}

// ghostCmd returns the CLI command that would produce the same result as
// pressing enter right now. Updates live as the user types/navigates.
func (m PickerModel) ghostCmd() string {
	query := strings.TrimSpace(m.input.Value())

	switch m.mode {
	case modeTemplateName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name != "" {
			return "zmux new -t " + m.selectedTemplate + " " + name
		}
		return "zmux new -t " + m.selectedTemplate
	case modeTemplateSelect:
		if m.templateCursor < len(m.templates) {
			return "zmux new -t " + m.templates[m.templateCursor].Name
		}
		return "zmux new -t ..."
	case modeConfirmDelete:
		itemIdx := m.cursor - 1
		if itemIdx >= 0 && itemIdx < len(m.items) && m.items[itemIdx].Session != nil {
			return "zmux kill " + m.items[itemIdx].Session.Name
		}
		return "zmux kill ..."
	}

	// Normal mode.
	if m.cursor == 0 {
		// On "+ new session" entry.
		if query != "" {
			return "zmux new " + query
		}
		return "zmux new"
	}

	itemIdx := m.cursor - 1
	if itemIdx < len(m.items) {
		item := m.items[itemIdx]
		if item.IsHeader {
			return "# " + item.Header
		}
		if item.Session != nil {
			s := item.Session
			if s.Attached {
				return "zmux " + s.Name + "  \u2192  " + s.Name + "-b"
			}
			return "zmux " + s.Name
		}
	}

	// External source ghost commands.
	if ext := m.externalItemAt(m.cursor); ext != nil {
		if ext.isHeader {
			return "# toggle " + ext.group.Source.Label
		}
		if ext.group.Source.Kind == source.SourceOvermind && ext.group.Source.Overmind != nil {
			return "overmind connect " + ext.entry.Session + " -s " + ext.group.Source.Overmind.ControlSocket
		}
		ep := ext.group.Source.Endpoint
		return "tmux " + strings.Join(ep.Args(), " ") + " attach -t " + ext.entry.Session
	}

	return "zmux new"
}

// shortenPath replaces the home directory with ~ and truncates long paths.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}
