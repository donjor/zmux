// Package tabs is the logical-tab core: a zmux-managed tab is a PANE with a
// stable opaque identity (@zmux_tab_id), not a window. Windows are
// presentation containers; placement (full window, pane-of another tab,
// parked in the hidden dock) is computed live from tmux state on every scan
// — tmux is physical truth, options are identity and advisory metadata only.
package tabs

import "time"

// Pane-scoped user options owned by this package. Label/state option names
// live in tablabel/tabstate; these complete the logical-tab contract.
const (
	// OptTabID is the opaque stable identity. It is ONLY ever set at pane
	// scope: tmux format reads merge scopes (pane → window → session), so a
	// window-scoped value would make every pane in that window read as
	// managed.
	OptTabID = "@zmux_tab_id"
	// OptAnchor advisorily names the host tab id a pane-of tab was joined
	// to. Display/repair hint only — live placement never trusts it over
	// physical window membership.
	OptAnchor = "@zmux_tab_anchor"
	// OptHidden holds the ORIGIN SESSION name while a tab is docked: one
	// option doubles as the hidden flag and the default `tab show` target.
	OptHidden = "@zmux_hidden"
)

// Lifecycle user options (pane-canonical, like OptTabID) — how a tab was born
// and whether it's safe for the reaper to flag/kill. Plan 038.
const (
	OptOrigin      = "@zmux_origin"        // agent | human | preexisting
	OptBorn        = "@zmux_born"          // unix seconds, set ONCE at birth
	OptScope       = "@zmux_scope"         // agent-shell | task | daemon | peer | worker | shell
	OptTTL         = "@zmux_ttl"           // seconds; optional per-tab override
	OptKeep        = "@zmux_keep"          // "1" = never auto-reap
	OptStaleAt     = "@zmux_stale_at"      // unix seconds; recorded by an EARLIER reap sweep
	OptLastInputAt = "@zmux_last_input_at" // unix seconds; zmux-mediated input (run/send/type)
)

// Origin values — who created the tab. Default conservative (human/preexisting);
// only an explicit signal marks a tab agent-created.
const (
	OriginAgent       = "agent"
	OriginHuman       = "human"
	OriginPreexisting = "preexisting"
)

// Scope values — what the tab is for. Drives reaper eligibility.
const (
	ScopeAgentShell = "agent-shell" // a long-lived agent CLI shell; never auto-killed
	ScopeTask       = "task"        // an ad-hoc run; reapable when stale
	ScopeDaemon     = "daemon"      // a long-running server; never auto-killed
	ScopePeer       = "peer"        // a review peer; peer-skill teardown owns it
	ScopeWorker     = "worker"      // an orchestrate worker session; orchestrate owns it
	ScopeShell      = "shell"       // a plain interactive shell
)

// Placement is where a logical tab physically lives right now.
type Placement string

const (
	// PlacementFull: the tab owns its window (raw human splits beside it
	// don't demote it — only other managed tabs do).
	PlacementFull Placement = "full"
	// PlacementPaneOf: the tab is a pane inside another managed tab's
	// window.
	PlacementPaneOf Placement = "pane-of"
	// PlacementDock: the tab is parked in the hidden dock session.
	PlacementDock Placement = "dock"
)

// LogicalTab is one zmux-managed tab with its live-computed placement.
type LogicalTab struct {
	ID     string // @zmux_tab_id (ztab_…)
	Label  string // user-facing, mutable; merged-scope read (pane wins)
	PaneID string // %N — the canonical address for send/capture/state

	Session string // session the pane is physically in (dock when hidden)
	// OriginSession is where the tab logically belongs: @zmux_hidden while
	// docked, otherwise Session.
	OriginSession string

	WindowID    string
	WindowIndex int
	WindowName  string
	WindowPanes int

	Placement Placement
	// AnchorID is the host tab's id while pane-of (live-computed; the
	// advisory option only breaks owner ties). Empty otherwise.
	AnchorID string

	State string // @zmux_state (pane-canonical)

	Visible bool // window is its session's current window
	Active  bool // Visible AND the pane is its window's active pane

	Command  string
	Dir      string
	Title    string
	Activity time.Time // window_activity — MRU/recency input
}
