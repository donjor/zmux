package tabs

import (
	"errors"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// ReapAction is the reaper's verdict for one pane/tab.
type ReapAction string

const (
	ReapKeep  ReapAction = "keep"  // not eligible
	ReapFlag  ReapAction = "flag"  // mark stale (set @zmux_stale_at); never kill this pass
	ReapKill  ReapAction = "kill"  // eligible to kill
	ReapAdopt ReapAction = "adopt" // unborn pre-existing tab: stamp born=now, then keep
)

// Reaper timing defaults (plan 038). Agent task tabs are litter on a short
// clock; human/pre-existing tabs get a long, visible ramp.
const (
	DefaultAgentTTL     = time.Hour
	DefaultHumanFlagAge = 4 * time.Hour
	DefaultHumanKillAge = 24 * time.Hour
)

// minStalePersist is how long a tab's @zmux_stale_at flag must have stood before
// a kill. It closes the concurrency hole where two racing sweeps both pass the
// throttle, then one flags a tab and the other kills it in the same instant
// (no human-visible warning window). Far below any real human kill gap (flag at
// 4h, kill at 24h → stale_at ~20h old), so it never delays a legitimate kill.
const minStalePersist = 5 * time.Minute

// ReapDecision is the verdict for one tab plus the human-readable why.
type ReapDecision struct {
	PaneID   string
	WindowID string // @N — kill target (a reapable tab is always its window's sole pane)
	Session  string
	Group    string // session group key (group name, else session) — kill-budget unit
	Label    string
	Origin   string
	Scope    string
	Action   ReapAction
	Reason   string
}

// groupKey is the kill-budget / last-window unit. Session-group clones (dev,
// dev-b) share linked windows, so a kill removes the window from every clone —
// they must be counted and reserved as one. Falls back to the session name when
// ungrouped.
func groupKey(r tmux.LogicalPaneRow) string {
	if r.SessionGroup != "" {
		return r.SessionGroup
	}
	return r.Session
}

// collapseClones reduces session-group duplicate rows to one per pane. tmux
// `list-panes -a` repeats a grouped window's pane once per clone session; a kill
// removes the shared window from all of them, so the reaper must treat it as a
// single tab. Visibility is OR-ed across clones — a pane visible/attached in ANY
// clone is kept. First-seen order is preserved for deterministic kill ordering.
func collapseClones(rows []tmux.LogicalPaneRow) []tmux.LogicalPaneRow {
	idx := map[string]int{}
	out := make([]tmux.LogicalPaneRow, 0, len(rows))
	for _, r := range rows {
		if r.PaneID == "" {
			out = append(out, r) // unidentifiable — can't dedup safely
			continue
		}
		if i, ok := idx[r.PaneID]; ok {
			if r.SessionAttached > 0 && r.WindowActive && r.PaneActive {
				out[i].SessionAttached = r.SessionAttached
				out[i].WindowActive = true
				out[i].PaneActive = true
			}
			continue
		}
		idx[r.PaneID] = len(out)
		out = append(out, r)
	}
	return out
}

// ReapContext is the (mostly injectable) policy environment. Zero-value
// durations and a nil IsLive fall back to the package defaults.
type ReapContext struct {
	Now          time.Time
	CallerPaneID string // the invoking pane — never killed
	AgentTTL     time.Duration
	HumanFlagAge time.Duration
	HumanKillAge time.Duration
	IsLive       func(tmux.LogicalPaneRow) bool
}

func (c ReapContext) withDefaults() ReapContext {
	if c.AgentTTL == 0 {
		c.AgentTTL = DefaultAgentTTL
	}
	if c.HumanFlagAge == 0 {
		c.HumanFlagAge = DefaultHumanFlagAge
	}
	if c.HumanKillAge == 0 {
		c.HumanKillAge = DefaultHumanKillAge
	}
	if c.IsLive == nil {
		c.IsLive = PaneIsLive
	}
	return c
}

// idleShells are the shells whose presence as a pane's foreground command means
// "sitting at a prompt" — eligible to be considered idle. Anything else
// (editors, REPLs, agent CLIs, servers, TUIs) is a live foreground process.
var idleShells = map[string]bool{"bash": true, "zsh": true, "fish": true, "sh": true, "dash": true, "ksh": true}

// PaneIsLive reports whether a pane has a live FOREGROUND process. tmux's
// pane_current_command is the pane's foreground process, so a known shell means
// the prompt is idle. Unknown/empty command → live (false negatives over false
// kills). Background jobs under an idle shell are invisible here — ApplyReap's
// final confirm covers them with a pane_pid child-process check before any kill.
func PaneIsLive(row tmux.LogicalPaneRow) bool {
	if row.Command == "" {
		return true
	}
	return !idleShells[row.Command]
}

// PlanReap classifies every scanned row. Pure: no tmux writes — the apply step
// (separate) acts on Flag/Kill/Adopt. Operates on raw rows, so unmanaged
// pre-existing tabs (no @zmux_tab_id) are covered too.
func PlanReap(rows []tmux.LogicalPaneRow, ctx ReapContext) []ReapDecision {
	ctx = ctx.withDefaults()
	rows = collapseClones(rows)
	// Distinct windows per session group — killing the last leaves none.
	windows := sessionWindowSets(rows)
	out := make([]ReapDecision, 0, len(rows))
	for _, r := range rows {
		out = append(out, classifyReap(r, ctx, len(windows[groupKey(r)])))
	}
	return out
}

func classifyReap(r tmux.LogicalPaneRow, ctx ReapContext, sessionWindows int) ReapDecision {
	d := ReapDecision{PaneID: r.PaneID, WindowID: r.WindowID, Session: r.Session, Group: groupKey(r), Label: labelOf(r), Origin: r.Origin, Scope: r.Scope}
	keep := func(reason string) ReapDecision { d.Action = ReapKeep; d.Reason = reason; return d }

	// Hard never-touch guards (order matters: cheapest/safest first).
	if r.Keep == "1" {
		return keep("--keep")
	}
	switch r.Scope {
	case ScopeDaemon:
		return keep("daemon")
	case ScopeAgentShell:
		return keep("agent-shell")
	case ScopeWorker:
		return keep("worker (orchestrate-owned)")
	case ScopePeer:
		return keep("peer (peer-skill owned)")
	}
	if r.PaneID != "" && r.PaneID == ctx.CallerPaneID {
		return keep("calling pane")
	}
	if r.Hidden != "" {
		return keep("hidden/docked")
	}
	if sessionWindows <= 1 {
		return keep("last window in session")
	}
	if r.WindowPanes > 1 {
		return keep("multi-pane window")
	}
	if r.SessionAttached > 0 && r.WindowActive && r.PaneActive {
		return keep("visible in attached client")
	}

	// First-seen: an unstamped pre-existing tab. Stamp born=now so its age clock
	// starts at first sight — never kill old human state right after install.
	born, ok := ParseUnix(r.Born)
	if !ok {
		d.Action = ReapAdopt
		d.Reason = "first seen — stamping born"
		return d
	}
	age := ctx.Now.Sub(born)
	live := ctx.IsLive(r)
	act := lastActivity(r)
	// No activity signal at all (tmux gave no window_activity, no zmux input) —
	// we can't prove idle, so never kill on the guess. idleFor would otherwise be
	// "now - epoch", a false eternity. (codex review, plan 038)
	noSignal := act.IsZero()
	idleFor := ctx.Now.Sub(act)

	// Agent-created task tabs: short clock, killed directly when idle past ttl.
	if r.Origin == OriginAgent && r.Scope == ScopeTask {
		ttl := ctx.AgentTTL
		if t, ok := ParseTTL(r.TTL); ok {
			ttl = t
		}
		if live || noSignal {
			return keep("agent task busy (live process)")
		}
		if idleFor >= ttl && age >= ttl {
			d.Action = ReapKill
			d.Reason = "agent task idle past ttl"
			return d
		}
		return keep("agent task within ttl")
	}

	// Human / pre-existing / plain shell: long, visible ramp.
	if age < ctx.HumanFlagAge {
		return keep("within flag age")
	}
	if live {
		d.Action = ReapFlag
		d.Reason = "human tab old but live — flag only"
		return d
	}
	// Timing invariant: kill only if an EARLIER sweep already recorded stale_at
	// (flag-then-kill across separate passes), the flag has stood at least
	// minStalePersist (defeats a concurrent flag-then-kill in one instant), past
	// the kill age, still idle, with a real activity signal to prove the idle.
	staleAt, flagged := ParseUnix(r.StaleAt)
	staleEnough := flagged && ctx.Now.Sub(staleAt) >= minStalePersist
	if staleEnough && !noSignal && age >= ctx.HumanKillAge && idleFor >= ctx.HumanFlagAge {
		d.Action = ReapKill
		d.Reason = "human tab idle, flagged earlier, past kill age"
		return d
	}
	d.Action = ReapFlag
	d.Reason = "human tab stale"
	return d
}

// lastActivity is the most recent observable activity: tmux window_activity or
// the last zmux-mediated input, whichever is later.
func lastActivity(r tmux.LogicalPaneRow) time.Time {
	last := r.WindowActivity
	if in, ok := ParseUnix(r.LastInputAt); ok && in.After(last) {
		last = in
	}
	return last
}

func labelOf(r tmux.LogicalPaneRow) string {
	if r.Label != "" {
		return r.Label
	}
	return r.WindowName
}

// ReapStats reports what an apply pass actually did.
type ReapStats struct {
	Killed  int
	Flagged int
	Adopted int
}

// ApplyReap scans, plans, and executes in one pass. Flag/adopt are idempotent
// (set-once) and applied directly. Kills are treated as ADVISORY: before any
// pane dies it is re-confirmed against a SECOND, fresh scan and pane-exact
// option reads, so nothing is killed on stale or forged state (codex review #2,
// plan 038). Specifically a kill must clear:
//
//   - group kill budget: never kill a session GROUP's last window. list-panes -a
//     repeats a grouped window once per clone and a kill removes it from all of
//     them, so counting + reservation are by group, not session name.
//   - fresh-state confirm: re-scan by pane id; the pane must still exist, still
//     live in the same window, and re-run the full policy to Kill on its CURRENT
//     volatile state (not the plan's snapshot — it may have moved, gained a pane,
//     become visible, or started a process since).
//   - pane-exact lifecycle: re-read origin/scope/born/ttl/keep/stale_at pane-
//     exactly (defeats merged window/session inheritance); any read error vetoes.
//   - no background work: veto if the pane's shell has live child processes a
//     foreground-only liveness check can't see (a human `job &`).
func ApplyReap(r tmux.Runner, ctx ReapContext) (ReapStats, error) {
	ctx = ctx.withDefaults()
	rows, err := r.ListLogicalPaneRows()
	if err != nil {
		return ReapStats{}, err
	}
	decisions := PlanReap(rows, ctx)

	var stats ReapStats
	var errs []error
	// fresh scan + budget lazily built only if a kill is actually pending.
	var fresh map[string]tmux.LogicalPaneRow
	var winLeft map[string]int
	for _, d := range decisions {
		switch d.Action {
		case ReapAdopt:
			if e := StampBirth(r, d.PaneID, OriginPreexisting, ScopeShell, ctx.Now); e != nil {
				errs = append(errs, e)
				continue
			}
			stats.Adopted++
		case ReapFlag:
			if e := SetStaleAt(r, d.PaneID, ctx.Now); e != nil {
				errs = append(errs, e)
				continue
			}
			stats.Flagged++
		case ReapKill:
			if fresh == nil {
				fresh, winLeft = freshKillState(r) // post-flag, maximally current
			}
			cur, ok := fresh[d.PaneID]
			if !ok || cur.WindowID != d.WindowID || groupKey(cur) != d.Group {
				continue // pane vanished, or moved window/group since the plan
			}
			if winLeft[d.Group] <= 1 {
				continue // reserve the group's last window (fresh-scan count)
			}
			if !confirmKill(r, cur, ctx) {
				continue // current/pane-exact policy disagrees, or a read error
			}
			if r.PaneHasLiveChildren(cur.PanePID) {
				continue // backgrounded job under an idle prompt
			}
			if e := r.KillWindowByID(d.WindowID); e != nil {
				errs = append(errs, e)
				continue
			}
			winLeft[d.Group]--
			stats.Killed++
		}
	}
	return stats, errors.Join(errs...)
}

// freshKillState re-scans right before the kill phase and returns the current
// pane rows (by pane id) plus the live per-group window budget. A best-effort
// failure yields empty maps — every kill then no-ops (pane "vanished"), which is
// the safe direction.
func freshKillState(r tmux.Runner) (map[string]tmux.LogicalPaneRow, map[string]int) {
	rows, err := r.ListLogicalPaneRows()
	if err != nil {
		return map[string]tmux.LogicalPaneRow{}, map[string]int{}
	}
	rows = collapseClones(rows)
	byPane := make(map[string]tmux.LogicalPaneRow, len(rows))
	for _, row := range rows {
		byPane[row.PaneID] = row
	}
	winLeft := map[string]int{}
	for g, wins := range sessionWindowSets(rows) {
		winLeft[g] = len(wins)
	}
	return byPane, winLeft
}

// confirmKill re-validates a kill against the fresh row's CURRENT volatile state
// plus pane-exact lifecycle options. Any pane-option read error vetoes (we won't
// kill on an unreadable @zmux_keep). sessionWindows is forced to 2 so the
// last-window guard doesn't mask a real policy disagreement — the budget owns
// last-window reservation.
func confirmKill(r tmux.Runner, row tmux.LogicalPaneRow, ctx ReapContext) bool {
	for _, f := range []struct {
		key string
		set func(v string)
	}{
		{OptOrigin, func(v string) { row.Origin = v }},
		{OptScope, func(v string) { row.Scope = v }},
		{OptBorn, func(v string) { row.Born = v }},
		{OptTTL, func(v string) { row.TTL = v }},
		{OptKeep, func(v string) { row.Keep = v }},
		{OptStaleAt, func(v string) { row.StaleAt = v }},
	} {
		v, err := r.ShowPaneOption(row.PaneID, f.key)
		if err != nil {
			return false // veto on any exact-read error
		}
		f.set(v)
	}
	return classifyReap(row, ctx, 2).Action == ReapKill
}

// sessionWindowSets counts distinct windows per session GROUP (group clones share
// linked windows, so they reserve/budget as one unit).
func sessionWindowSets(rows []tmux.LogicalPaneRow) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, r := range rows {
		key := groupKey(r)
		if out[key] == nil {
			out[key] = map[string]bool{}
		}
		out[key][r.WindowID] = true
	}
	return out
}
