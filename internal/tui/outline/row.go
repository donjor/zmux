// Package outline provides a flat, cursor-friendly tree model used by
// the picker and dashboard tabs. It is intentionally a pure data package
// with no bubbletea or internal/tui dependency, so it is unit-testable
// in isolation and cannot participate in an import cycle.
package outline

import "fmt"

// FormatSessionCount returns "N sessions" with grammar-aware pluralisation.
// Used by callers that compose workspace labels.
func FormatSessionCount(n int) string {
	if n == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

// RowKind distinguishes what sits on a given line. The same key press can
// mean different things depending on RowKind — this is the pivot point for
// the context-sensitive key handling in Session / Workspaces tabs.
type RowKind int

const (
	// RowTopAction is the create/new action row shown at the top of a
	// view (e.g. "+ new tmp session" or "+ create \"name\"" in the picker).
	RowTopAction RowKind = iota

	// RowWorkspaceHeader is a workspace row that may have sessions beneath
	// it when expanded.
	RowWorkspaceHeader

	// RowSession is a session (child of a workspace).
	RowSession

	// RowWindow is a tmux window (child of a session), used by the
	// Session tab's depth-3 tree.
	RowWindow

	// RowPane is a tmux pane (child of a window), used by the Session tab
	// when a window has one or more panes.
	RowPane

	// RowExternalGroup is the header row for an external source group
	// (e.g. an overmind instance, a tmux socket).
	RowExternalGroup

	// RowExternalEntry is a single entry within an external source group.
	RowExternalEntry

	// RowDivider is a thin visual separator (e.g. "── external ──"). Not
	// selectable.
	RowDivider

	// RowPlaceholder is explanatory text when a section is empty
	// (e.g. "(no live sessions)"). Not selectable.
	RowPlaceholder
)

// Row is one line in the outline. Data is type-erased because different
// callers need different payloads; typed accessors live in the consuming
// package. The pointer is stable for the lifetime of one build.
type Row struct {
	ID       string // stable across rebuilds (see ID constructors below)
	Kind     RowKind
	Depth    int    // 0 = top-level, 1 = child, 2 = grandchild
	ParentID string // empty if top-level
	Label    string // primary display label (renderers may enrich)
	Data     any    // caller-owned payload (cast in per-package renderers)

	// Visual flags decoupled from Kind so a renderer can style without
	// re-switching on the kind.
	Selectable bool   // false for dividers / placeholders
	Current    bool   // "this is the currently active thing" marker
	Attached   bool   // has an attached tmux client
	Expanded   bool   // only meaningful for rows with children
	Badge      string // optional right-aligned badge text (move mode etc.)
}

// ── Stable ID constructors ──
//
// Every consumer of the outline package must use these to produce row IDs.
// Cursor restoration and expansion state are keyed by these strings, so any
// drift in shape would silently break both.
//
// External entry / group IDs deliberately avoid index-based keys. Sources
// re-order their catalogs across refetches, which would invalidate every
// cursor and expansion state entry on every refresh.

// TopActionID is the stable ID of the "+ new" / "+ create" top row.
func TopActionID() string { return "top" }

// WorkspaceID is the stable ID of a workspace row.
func WorkspaceID(name string) string { return "ws:" + name }

// SessionID is the stable ID of a session row.
func SessionID(name string) string { return "session:" + name }

// WindowID is the stable ID of a window row under a session.
func WindowID(sessionName string, index int) string {
	return fmt.Sprintf("window:%s:%d", sessionName, index)
}

// PaneID is the stable ID of a pane row under a window.
func PaneID(sessionName, paneID string) string {
	return fmt.Sprintf("pane:%s:%s", sessionName, paneID)
}

// ExternalGroupID is the stable ID of an external source group header.
// kind identifies the source type ("overmind", "tmux-socket", etc.);
// key is the source's own instance identifier (socket path, workspace
// path, etc.).
func ExternalGroupID(kind, key string) string {
	return fmt.Sprintf("extgroup:%s:%s", kind, key)
}

// ExternalEntryID is the stable ID of a single external catalog entry.
// kind identifies the source type; key is the source's own stable key
// for the entry (catalog session name, overmind process name, etc.).
func ExternalEntryID(kind, key string) string {
	return fmt.Sprintf("extentry:%s:%s", kind, key)
}

// RowData unpacks a row's typed payload. The type parameter is the
// element type, not a pointer — the result is always a pointer to that
// element (the same shape Rows are built with). Safe on nil rows:
// returns (nil, false) without panicking.
//
// Use this wherever callers would otherwise write:
//
//	ws, ok := row.Data.(*WorkspaceViewModel)
//
// …so that cursor lookup + nil check + type assertion collapse into
// one line at the call site.
func RowData[T any](r *Row) (*T, bool) {
	if r == nil {
		return nil, false
	}
	v, ok := r.Data.(*T)
	return v, ok
}
