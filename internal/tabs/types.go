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

// Peer/agent-turn lifecycle options (pane-canonical). These are policy metadata,
// not the human-facing glyph mirror in tabstate. Keep values sanitized: no full
// prompts or sensitive task text in tmux-visible options.
const (
	OptTurnState    = "@zmux_turn_state"     // running | ready(waiting legacy) | attention | failed | consumed | parked
	OptTurnAt       = "@zmux_turn_at"        // unix seconds for the latest turn-state transition
	OptTurnSeq      = "@zmux_turn_seq"       // generation incremented when a new peer/agent turn starts
	OptPeerRole     = "@zmux_peer_role"      // claude | codex | pi | agy | unknown
	OptPeerHostTab  = "@zmux_peer_host_tab"  // stable host logical tab id, when known
	OptPeerHostPane = "@zmux_peer_host_pane" // host pane id, when known
	OptPeerTopic    = "@zmux_peer_topic"     // sanitized display topic/title
	OptPeerTurns    = "@zmux_peer_turns"     // diagnostic turn count for the current topic
	OptPeerLastTurn = "@zmux_peer_last_turn" // unix seconds for latest peer turn transition
	OptKeepUntil    = "@zmux_keep_until"     // unix seconds; timestamped retention
	OptParkUntil    = "@zmux_park_until"     // unix seconds; parked-peer inspection TTL
)

// Command lifecycle options (pane-canonical). These are written by shell
// integration for the pane's root interactive shell. They are the structured
// completion channel that normal shell use, zmux run, and Pi tools consume;
// stdout sentinels should not be part of the lifecycle contract.
const (
	OptCmdSeq        = "@zmux_cmd_seq"         // monotonically increasing root-shell command sequence
	OptCmdState      = "@zmux_cmd_state"       // running | done | failed
	OptCmdStartedAt  = "@zmux_cmd_started_at"  // unix seconds for command start
	OptCmdFinishedAt = "@zmux_cmd_finished_at" // unix seconds for command end
	OptCmdLastExit   = "@zmux_last_exit"       // last root-shell command exit code
	OptCmdRunID      = "@zmux_cmd_run_id"      // run nonce consumed by the running command
	OptNextRunID     = "@zmux_next_run_id"     // run nonce staged by zmux run before input
	OptRunResult     = "@zmux_run_result"      // <run-id>:<exit>, written at command end
	OptCmdText       = "@zmux_cmd_text"        // sanitized short command label for diagnostics
)

const (
	TurnRunning   = "running"
	TurnReady     = "ready"
	TurnWaiting   = "waiting" // legacy alias for ready
	TurnAttention = "attention"
	TurnFailed    = "failed"
	TurnConsumed  = "consumed"
	TurnParked    = "parked"
)

const (
	CmdRunning = "running"
	CmdDone    = "done"
	CmdFailed  = "failed"
)

var validScopes = map[string]bool{
	ScopeAgentShell: true,
	ScopeTask:       true,
	ScopeDaemon:     true,
	ScopePeer:       true,
	ScopeWorker:     true,
	ScopeShell:      true,
}

// ValidScope reports whether scope is a known lifecycle scope. Empty is valid
// for callers that want the command-specific default.
func ValidScope(scope string) bool {
	return scope == "" || validScopes[scope]
}

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
	ScopePeer       = "peer"        // a prompt-scoped review peer; reaped after park/ttl when safe
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
