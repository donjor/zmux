// Package tabs implements the individual dashboard tab models.
package tabs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/workspace"
)

// currentMode tracks the current interaction mode.
type currentMode int

const (
	currentModeList                 currentMode = iota // browsing the tree
	currentModeRename                                  // inline rename (ws / session / window)
	currentModeCreate                                  // inline "new session in workspace" input
	currentModeConfirmKill                             // y/N confirmation
	currentModeConfirmKillAttached                     // second-step confirm for killing attached ws
	currentModeMoveWindow                              // session picker for moving a window
)

// windowDetail combines a window with its pane details and process stats.
type windowDetail struct {
	tmux.Window
	Panes  []tmux.Pane
	Stats  tmux.ProcessStats
	Uptime string
}

// confirmState / renameState / moveState live in mode_state.go and are
// shared with the Workspaces tab.

// currentMoveTarget is a simplified session entry for the move picker.
type currentMoveTarget struct {
	Name    string
	Windows int
}

// ── Messages ──

// currentDataMsg carries fetched session + workspace data for the tab.
type currentDataMsg struct {
	reqID       int64
	wsName      string
	sessionName string
	sessionDir  string
	attached    int
	windows     []windowDetail
	siblings    []session.SessionInfo
	wsModel     *tui.WorkspaceViewModel // enriched workspace view for header actions
	err         error
}

func (m currentDataMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// currentMutationDoneMsg signals that a mutation completed — the tab refetches.
type currentMutationDoneMsg struct {
	reqID int64
}

func (m currentMutationDoneMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// currentMoveDestMsg carries the list of move destinations.
type currentMoveDestMsg struct {
	reqID    int64
	sessions []currentMoveTarget
}

func (m currentMoveDestMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// CurrentTab implements the Tab interface for the unified
// "Session & Workspace" view.
type CurrentTab struct {
	runner   tmux.Runner
	styles   tui.Styles
	wsLoader tui.WorkspaceDataLoader
	wsStore  *workspace.Store

	// Tree owns row data + cursor.
	tree *outline.Tree

	// Snapshot data.
	wsName      string
	sessionName string
	sessionDir  string
	attached    int
	windows     []windowDetail
	siblings    []session.SessionInfo
	wsModel     *tui.WorkspaceViewModel

	// Viewport.
	width  int
	height int

	// Interaction mode.
	mode currentMode

	// Inputs / overlay state.
	renameInput textinput.Model
	createInput textinput.Model
	rename      *renameState
	confirm     *confirmState
	moveSt      *moveState

	// Move-window overlay state.
	moveTargets []currentMoveTarget
	moveCursor  int

	// Async-correctness primitives.
	reqID         int64
	pending       *currentDataMsg // staged refetch arrived during a modal
	pendingJumpTo string          // one-shot row ID to land on after next apply
}

// NewCurrentTab creates a new "Session & Workspace" tab.
// wsLoader returns enriched workspace view models; wsStore is required for
// workspace CRUD mutations from inside the tab.
func NewCurrentTab(runner tmux.Runner, styles tui.Styles, wsLoader tui.WorkspaceDataLoader, wsStore *workspace.Store) *CurrentTab {
	ri := textinput.New()
	ri.Placeholder = "new name..."
	ri.CharLimit = 64

	ci := textinput.New()
	ci.Placeholder = "session name..."
	ci.CharLimit = 64

	return &CurrentTab{
		runner:      runner,
		styles:      styles,
		wsLoader:    wsLoader,
		wsStore:     wsStore,
		tree:        outline.NewTree(),
		renameInput: ri,
		createInput: ci,
	}
}

// ── Tab interface ──

func (t *CurrentTab) ID() dashboard.TabID { return dashboard.TabSession }
func (t *CurrentTab) Title() string       { return "Session & Workspace" }
func (t *CurrentTab) Init() tea.Cmd       { return nil }

func (t *CurrentTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = dashboard.NextReqID()
	return t.fetchData(t.reqID)
}

func (t *CurrentTab) Deactivate() {
	// Bump reqID so any in-flight responses become stale.
	t.reqID = dashboard.NextReqID()
	t.exitMode()
	t.pending = nil
	t.pendingJumpTo = ""
	t.renameInput.Blur()
	t.createInput.Blur()
}

func (t *CurrentTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Update processes messages for the session tab.
func (t *CurrentTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil

	case currentDataMsg:
		t.onDataMsg(msg)
		return t, nil

	case currentMutationDoneMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.reqID = dashboard.NextReqID()
		return t, t.fetchData(t.reqID)

	case currentMoveDestMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.moveTargets = msg.sessions
		t.moveCursor = 0
		t.mode = currentModeMoveWindow
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward stray messages to the active text input.
	switch t.mode {
	case currentModeRename:
		var cmd tea.Cmd
		t.renameInput, cmd = t.renameInput.Update(msg)
		return t, cmd
	case currentModeCreate:
		var cmd tea.Cmd
		t.createInput, cmd = t.createInput.Update(msg)
		return t, cmd
	}
	return t, nil
}

// onDataMsg handles an incoming data message: dropped if stale, staged if a
// modal is open, or applied immediately otherwise.
func (t *CurrentTab) onDataMsg(msg currentDataMsg) {
	if msg.reqID != t.reqID {
		return
	}
	if msg.err != nil {
		// Surface via the next status flash; don't crash the tab.
		return
	}
	if t.mode != currentModeList {
		t.pending = &msg
		return
	}
	t.applyData(msg)
}

// applyData replaces the snapshot data and rebuilds the tree.
func (t *CurrentTab) applyData(msg currentDataMsg) {
	t.wsName = msg.wsName
	t.sessionName = msg.sessionName
	t.sessionDir = msg.sessionDir
	t.attached = msg.attached
	t.windows = msg.windows
	t.siblings = msg.siblings
	t.wsModel = msg.wsModel
	rows := t.buildRows()
	if t.pendingJumpTo != "" {
		t.tree.SetRowsAndJumpTo(rows, t.pendingJumpTo)
		t.pendingJumpTo = ""
	} else {
		t.tree.SetRows(rows)
	}
}

// exitMode resets to list mode and applies any staged data refetch.
func (t *CurrentTab) exitMode() {
	t.mode = currentModeList
	t.rename = nil
	t.confirm = nil
	t.moveSt = nil
	t.renameInput.Blur()
	t.createInput.Blur()
	if t.pending != nil {
		t.applyData(*t.pending)
		t.pending = nil
	}
}

// ── ShortHelp ──

func (t *CurrentTab) ShortHelp() string {
	switch t.mode {
	case currentModeRename:
		return "enter:confirm  esc:cancel"
	case currentModeCreate:
		return "enter:create  esc:cancel"
	case currentModeConfirmKill, currentModeConfirmKillAttached:
		return "y:confirm  any:cancel"
	case currentModeMoveWindow:
		return "enter:move  esc:cancel"
	}

	row := t.tree.Current()
	if row == nil {
		return "r:rename  x:kill  n:new"
	}

	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return strings.Join([]string{"r:rename", "x:kill", "n:new session"}, "  ")
	case outline.RowSession:
		return strings.Join([]string{"enter:switch", "r:rename", "x:kill", "n:new window"}, "  ")
	case outline.RowWindow:
		return strings.Join([]string{"enter:focus", "r:rename", "x:kill", "m:move", "</>:reorder", "n:new"}, "  ")
	}
	return "r:rename  x:kill  n:new"
}

// ── View ──

// View renders the session tab content.
func (t *CurrentTab) View() string {
	if t.mode == currentModeMoveWindow {
		return t.viewMove()
	}

	var b strings.Builder
	b.WriteString(t.renderHeader())

	// Overlays render above the list.
	switch t.mode {
	case currentModeRename:
		b.WriteString(t.renderRenameOverlay())
	case currentModeCreate:
		b.WriteString(t.renderCreateOverlay())
	case currentModeConfirmKill:
		b.WriteString(t.renderConfirmOverlay(1))
	case currentModeConfirmKillAttached:
		b.WriteString(t.renderConfirmOverlay(2))
	}

	if t.sessionName == "" {
		b.WriteString("  " + t.styles.Dim.Render("No active session.") + "\n")
		b.WriteString("  " + t.styles.Dim.Render("Open this dashboard from within a tmux session.") + "\n")
		return b.String()
	}

	rows := t.tree.Rows
	listHeight := t.height - 8
	if listHeight < 5 {
		listHeight = 12
	}
	start, end := outline.ComputeWindow(t.tree.Cursor, len(rows), listHeight)

	if start > 0 {
		b.WriteString(t.styles.Dim.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		b.WriteString(t.renderRow(&rows[i], i == t.tree.Cursor))
	}
	if end < len(rows) {
		b.WriteString(t.styles.Dim.Render(fmt.Sprintf("  ↓ %d more", len(rows)-end)) + "\n")
	}
	if len(rows) == 0 {
		b.WriteString(t.styles.Dim.Render("  (no rows)") + "\n")
	}
	return b.String()
}

// renderHeader renders the compact session metadata line above the tree.
func (t *CurrentTab) renderHeader() string {
	var b strings.Builder
	b.WriteString("\n")

	if t.sessionName == "" {
		return b.String()
	}

	var metaParts []string
	if t.sessionDir != "" {
		metaParts = append(metaParts, shortenDir(t.sessionDir))
	}
	metaParts = append(metaParts, fmt.Sprintf("%d tabs", len(t.windows)))
	if t.attached > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d client", t.attached))
	}

	b.WriteString("  " + t.styles.Dim.Render(strings.Join(metaParts, "  ")) + "\n\n")
	return b.String()
}

// viewMove renders the window-move destination picker (full-screen overlay).
func (t *CurrentTab) viewMove() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + t.styles.Accent.Bold(true).Render("Move Window") + "\n")
	b.WriteString("  " + t.styles.Dim.Render("Move window to another session") + "\n\n")

	if len(t.moveTargets) == 0 {
		b.WriteString("  " + t.styles.Dim.Render("No other sessions available.") + "\n")
	} else {
		for i, s := range t.moveTargets {
			cursor := "  "
			nameStyle := t.styles.Normal
			if i == t.moveCursor {
				cursor = t.styles.Accent.Render("▸ ")
				nameStyle = t.styles.Accent.Bold(true)
			}
			meta := t.styles.Dim.Render(fmt.Sprintf("  %dw", s.Windows))
			b.WriteString("  " + cursor + nameStyle.Render(s.Name) + meta + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("  " + t.styles.Dim.Render("enter:move  esc:cancel") + "\n")
	return b.String()
}

// ── Helpers ──

// shortenDir replaces the home directory with ~ and truncates long paths.
func shortenDir(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 4 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}

// isIdleWindow returns true if the window's command is a shell with no CPU usage.
func isIdleWindow(w windowDetail) bool {
	if w.Stats.CPU > 0.5 {
		return false
	}

	cmd := ""
	for _, p := range w.Panes {
		if p.Active {
			cmd = p.Command
			break
		}
	}
	if cmd == "" && len(w.Panes) > 0 {
		cmd = w.Panes[0].Command
	}

	switch cmd {
	case "bash", "zsh", "fish", "sh", "dash", "ksh":
		return true
	}
	return false
}
