package tabs

import (
	"strconv"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// StampBirth records immutable birth identity on a pane: origin, scope, and the
// born timestamp. Born is the idempotency key — like the tab id, it is set ONCE.
// Re-stamping an already-born pane (a reused `run -n` tab) is a no-op, so a
// human-born tab later driven by an agent keeps origin=human. now is injected
// for testability.
func StampBirth(r tmux.Runner, paneID, origin, scope string, now time.Time) error {
	born, err := r.ShowPaneOption(paneID, OptBorn)
	if err != nil {
		return err
	}
	if born != "" {
		return nil // already born — identity is immutable
	}
	return r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptBorn, Value: strconv.FormatInt(now.Unix(), 10)},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptOrigin, Value: origin},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptScope, Value: scope},
	})
}

// MarkAgentShell tags a pane as an agent's home shell: origin=agent,
// scope=agent-shell. This is the ROOT signal the reaper's origin inheritance
// needs — a `run` fired from an agent-shell pane stamps its new tab origin=agent
// (short TTL), and the shell itself is a keep-scope (never auto-reaped). The
// zmux skill's session-start hook calls this so agent-driven tabs are tagged
// automatically, with no per-run env flag.
//
// Unlike StampBirth this OVERRIDES scope/origin rather than being set-once: the
// hook is definitive evidence the pane is agent-driven, so a tab adopted earlier
// as a plain shell is upgraded. born is stamped only when unset, preserving the
// original age clock for an already-born tab.
func MarkAgentShell(r tmux.Runner, paneID string, now time.Time) error {
	born, err := r.ShowPaneOption(paneID, OptBorn)
	if err != nil {
		return err
	}
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptOrigin, Value: OriginAgent},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptScope, Value: ScopeAgentShell},
	}
	if born == "" {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: OptBorn, Value: strconv.FormatInt(now.Unix(), 10)})
	}
	return r.ApplyOptions(writes)
}

// SetTTL writes a per-tab ttl override (stored as whole seconds). A non-positive
// ttl unsets the option, reverting the tab to the scope default.
func SetTTL(r tmux.Runner, paneID string, ttl time.Duration) error {
	if ttl <= 0 {
		return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptTTL, Unset: true}})
	}
	return r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptTTL, Value: strconv.FormatInt(int64(ttl/time.Second), 10)},
	})
}

// SetKeep marks (or clears) a tab as never-auto-reap.
func SetKeep(r tmux.Runner, paneID string, keep bool) error {
	if !keep {
		return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptKeep, Unset: true}})
	}
	return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptKeep, Value: "1"}})
}

// PeerMetadata is sanitized, tmux-visible metadata for a prompt-scoped peer.
// Do not put full prompts or sensitive task text here.
type PeerMetadata struct {
	Role     string
	HostTab  string
	HostPane string
	Topic    string
}

// StampPeer marks a pane as an agent-owned prompt peer and writes the minimal
// metadata consumed by diagnostics/reaper policy. It deliberately does NOT set
// @zmux_keep=1; prompt-scoped peer retention should be timestamped via
// SetPeerKeepUntil/SetPeerParkUntil.
func StampPeer(r tmux.Runner, paneID string, meta PeerMetadata, now time.Time) error {
	if err := StampBirth(r, paneID, OriginAgent, ScopePeer, now); err != nil {
		return err
	}
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptOrigin, Value: OriginAgent},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptScope, Value: ScopePeer},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptKeep, Unset: true},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptKeepUntil, Unset: true},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptParkUntil, Unset: true},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptStaleAt, Unset: true},
	}
	writes = appendPeerMetadataWrite(writes, paneID, OptPeerRole, strings.TrimSpace(meta.Role))
	writes = appendPeerMetadataWrite(writes, paneID, OptPeerHostTab, strings.TrimSpace(meta.HostTab))
	writes = appendPeerMetadataWrite(writes, paneID, OptPeerHostPane, strings.TrimSpace(meta.HostPane))
	if topic := sanitizePeerTopic(meta.Topic); topic != "" {
		writes = appendPeerMetadataWrite(writes, paneID, OptPeerTopic, topic)
		writes = append(writes,
			tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: OptPeerTurns, Unset: true},
			tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: OptPeerLastTurn, Unset: true},
		)
	}
	return r.ApplyOptions(writes)
}

func appendPeerMetadataWrite(writes []tmux.OptionWrite, paneID, key, value string) []tmux.OptionWrite {
	if value == "" {
		return writes
	}
	return append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: key, Value: value})
}

func sanitizePeerTopic(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		s = strings.TrimSpace(s[:80])
	}
	return s
}

func SetTurnState(r tmux.Runner, paneID, state string, now time.Time) error {
	state = NormalizeTurnState(state)
	switch state {
	case TurnRunning, TurnReady, TurnAttention, TurnFailed, TurnConsumed, TurnParked:
	default:
		return nil
	}
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptTurnState, Value: state},
		{Scope: tmux.ScopePane, Target: paneID, Key: OptTurnAt, Value: strconv.FormatInt(now.Unix(), 10)},
	}
	if state == TurnRunning {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: OptPeerTurns, Value: strconv.Itoa(nextPeerTurn(r, paneID))})
	}
	if state == TurnRunning || state == TurnReady || state == TurnAttention || state == TurnFailed {
		writes = append(writes, tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: OptPeerLastTurn, Value: strconv.FormatInt(now.Unix(), 10)})
	}
	return r.ApplyOptions(writes)
}

// NormalizeTurnState maps legacy spellings to the v2 turn-state vocabulary.
func NormalizeTurnState(state string) string {
	if state == TurnWaiting {
		return TurnReady
	}
	return state
}

func nextPeerTurn(r tmux.Runner, paneID string) int {
	cur, err := r.ShowPaneOption(paneID, OptPeerTurns)
	if err != nil {
		return 1
	}
	n, err := strconv.Atoi(cur)
	if err != nil || n < 0 {
		return 1
	}
	return n + 1
}

func SetPeerKeepUntil(r tmux.Runner, paneID string, until time.Time) error {
	return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptKeepUntil, Value: strconv.FormatInt(until.Unix(), 10)}})
}

func ClearPeerKeepUntil(r tmux.Runner, paneID string) error {
	return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptKeepUntil, Unset: true}})
}

func SetPeerParkUntil(r tmux.Runner, paneID string, until time.Time) error {
	return r.ApplyOptions([]tmux.OptionWrite{{Scope: tmux.ScopePane, Target: paneID, Key: OptParkUntil, Value: strconv.FormatInt(until.Unix(), 10)}})
}

// SetStaleAt records the first-flag time for a tab — the reaper's "warned in an
// earlier sweep" marker. Set-once like born: a tab flagged at T keeps stale_at=T
// across later sweeps, so the kill clock measures from the original warning, not
// the most recent pass. now is injected for testability.
func SetStaleAt(r tmux.Runner, paneID string, now time.Time) error {
	cur, err := r.ShowPaneOption(paneID, OptStaleAt)
	if err != nil {
		return err
	}
	if cur != "" {
		return nil // already flagged — keep the original warning time
	}
	return r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptStaleAt, Value: strconv.FormatInt(now.Unix(), 10)},
	})
}

// TouchInput records the last zmux-mediated input time (run/send/type). The
// reaper's idle test trusts this for "no input" — raw human keypresses in a tab
// aren't observable, so this is a conservative lower bound on activity.
func TouchInput(r tmux.Runner, paneID string, now time.Time) error {
	return r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptLastInputAt, Value: strconv.FormatInt(now.Unix(), 10)},
	})
}

// ResolveOrigin picks the origin for a NEW tab, conservative by construction.
// Priority: explicit flag → caller-pane inheritance (a run fired from inside an
// agent shell is agent-originated) → env ZMUX_ACTOR=agent → fallback human.
// callerOrigin/callerScope are the lifecycle options of the pane the command was
// invoked from (empty if unknown); envActor is os.Getenv("ZMUX_ACTOR").
func ResolveOrigin(flagOrigin, callerOrigin, callerScope, envActor string) string {
	switch flagOrigin {
	case OriginAgent, OriginHuman, OriginPreexisting:
		return flagOrigin
	}
	if callerOrigin == OriginAgent || callerScope == ScopeAgentShell {
		return OriginAgent
	}
	if envActor == OriginAgent {
		return OriginAgent
	}
	return OriginHuman
}

// ParseUnix reads a stored unix-seconds option into a time.Time. Returns the
// zero time (and false) when unset or malformed — callers treat that as "no
// signal", never as epoch 0.
func ParseUnix(opt string) (time.Time, bool) {
	if opt == "" {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(opt, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

// ParseTTL reads a stored ttl (whole seconds) into a Duration; ok is false when
// unset or malformed.
func ParseTTL(opt string) (time.Duration, bool) {
	if opt == "" {
		return 0, false
	}
	sec, err := strconv.ParseInt(opt, 10, 64)
	if err != nil || sec <= 0 {
		return 0, false
	}
	return time.Duration(sec) * time.Second, true
}
