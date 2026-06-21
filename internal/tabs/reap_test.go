package tabs

import (
	"strconv"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

func TestClassifyReap(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	ago := func(d time.Duration) string { return strconv.FormatInt(now.Add(-d).Unix(), 10) }
	ctx := ReapContext{Now: now, CallerPaneID: "%caller"}.withDefaults()

	// base: a single-pane, unattached, bash-at-prompt tab in a multi-window
	// session — clears every never-touch guard so the policy body is exercised.
	base := func() tmux.LogicalPaneRow {
		return tmux.LogicalPaneRow{PaneID: "%1", Session: "work", WindowID: "@1", WindowPanes: 1, Command: "bash"}
	}

	cases := []struct {
		name           string
		mut            func(*tmux.LogicalPaneRow)
		sessionWindows int
		want           ReapAction
	}{
		{"keep flag", func(r *tmux.LogicalPaneRow) { r.Keep = "1" }, 2, ReapKeep},
		{"daemon", func(r *tmux.LogicalPaneRow) { r.Scope = ScopeDaemon }, 2, ReapKeep},
		{"agent-shell", func(r *tmux.LogicalPaneRow) { r.Scope = ScopeAgentShell }, 2, ReapKeep},
		{"worker", func(r *tmux.LogicalPaneRow) { r.Scope = ScopeWorker }, 2, ReapKeep},
		{"peer", func(r *tmux.LogicalPaneRow) { r.Scope = ScopePeer }, 2, ReapKeep},
		{"calling pane", func(r *tmux.LogicalPaneRow) { r.PaneID = "%caller" }, 2, ReapKeep},
		{"hidden", func(r *tmux.LogicalPaneRow) { r.Hidden = "work" }, 2, ReapKeep},
		{"last window", func(r *tmux.LogicalPaneRow) { r.Origin = OriginHuman; r.Born = ago(48 * time.Hour) }, 1, ReapKeep},
		{"multi-pane", func(r *tmux.LogicalPaneRow) { r.WindowPanes = 2 }, 2, ReapKeep},
		{"visible attached", func(r *tmux.LogicalPaneRow) {
			r.SessionAttached = 1
			r.WindowActive = true
			r.PaneActive = true
		}, 2, ReapKeep},

		{"unborn adopt", func(r *tmux.LogicalPaneRow) { r.Origin = OriginHuman; r.Born = "" }, 2, ReapAdopt},

		{"agent task idle past ttl", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginAgent
			r.Scope = ScopeTask
			r.Born = ago(2 * time.Hour)
			r.WindowActivity = now.Add(-2 * time.Hour)
		}, 2, ReapKill},
		{"agent task within ttl", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginAgent
			r.Scope = ScopeTask
			r.Born = ago(30 * time.Minute)
			r.WindowActivity = now.Add(-30 * time.Minute)
		}, 2, ReapKeep},
		{"agent task live", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginAgent
			r.Scope = ScopeTask
			r.Born = ago(2 * time.Hour)
			r.Command = "node"
		}, 2, ReapKeep},
		{"agent task custom ttl kill", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginAgent
			r.Scope = ScopeTask
			r.TTL = "300" // 5m
			r.Born = ago(10 * time.Minute)
			r.WindowActivity = now.Add(-10 * time.Minute)
		}, 2, ReapKill},

		{"human within flag age", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(time.Hour)
			r.WindowActivity = now.Add(-time.Hour)
		}, 2, ReapKeep},
		{"human old but live -> flag", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(5 * time.Hour)
			r.Command = "vim"
		}, 2, ReapFlag},
		{"human old idle unflagged -> flag", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(5 * time.Hour)
			r.WindowActivity = now.Add(-5 * time.Hour)
		}, 2, ReapFlag},
		{"human old idle flagged past kill -> kill", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(25 * time.Hour)
			r.StaleAt = ago(20 * time.Hour)
			r.WindowActivity = now.Add(-25 * time.Hour)
		}, 2, ReapKill},
		{"human flagged but not past kill age -> flag", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(5 * time.Hour)
			r.StaleAt = ago(time.Hour)
			r.WindowActivity = now.Add(-5 * time.Hour)
		}, 2, ReapFlag},
		{"preexisting old idle -> flag", func(r *tmux.LogicalPaneRow) {
			r.Origin = "" // unstamped origin reads as human path
			r.Born = ago(6 * time.Hour)
			r.WindowActivity = now.Add(-6 * time.Hour)
		}, 2, ReapFlag},

		// No activity signal at all (zero WindowActivity, no LastInputAt) — can't
		// prove idle, so never kill on the guess. (codex review)
		{"agent task no activity signal -> keep", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginAgent
			r.Scope = ScopeTask
			r.Born = ago(2 * time.Hour) // WindowActivity left zero
		}, 2, ReapKeep},
		{"human flagged past kill but no signal -> flag", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(25 * time.Hour)
			r.StaleAt = ago(20 * time.Hour) // WindowActivity left zero → no signal
		}, 2, ReapFlag},
		// Concurrency belt: flagged just now (< minStalePersist) can't be killed this
		// sweep even when otherwise past kill age — defeats a racing flag-then-kill.
		{"human flagged moments ago -> flag not kill", func(r *tmux.LogicalPaneRow) {
			r.Origin = OriginHuman
			r.Born = ago(25 * time.Hour)
			r.StaleAt = ago(2 * time.Minute)
			r.WindowActivity = now.Add(-25 * time.Hour)
		}, 2, ReapFlag},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base()
			tc.mut(&r)
			got := classifyReap(r, ctx, tc.sessionWindows)
			if got.Action != tc.want {
				t.Fatalf("action = %q (%s), want %q", got.Action, got.Reason, tc.want)
			}
		})
	}
}

// killWorthyOpts mirrors a kill-worthy agent-task tab into pane-exact reads, so
// confirmKillPaneExact re-validates to Kill instead of vetoing.
func killWorthyOpts(m *tmux.MockRunner, paneID string, bornUnix string) {
	if m.PaneOptions == nil {
		m.PaneOptions = map[string]string{}
	}
	m.PaneOptions[paneID+"\x00"+OptOrigin] = OriginAgent
	m.PaneOptions[paneID+"\x00"+OptScope] = ScopeTask
	m.PaneOptions[paneID+"\x00"+OptBorn] = bornUnix
}

func countCalls(m *tmux.MockRunner, method string) int {
	n := 0
	for _, c := range m.Calls {
		if c.Method == method {
			n++
		}
	}
	return n
}

func TestApplyReapKillBudgetReservesLastWindow(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	row := func(pane, win string) tmux.LogicalPaneRow {
		return tmux.LogicalPaneRow{
			PaneID: pane, WindowID: win, Session: "work", WindowPanes: 1,
			Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
			WindowActivity: now.Add(-2 * time.Hour),
		}
	}
	m := &tmux.MockRunner{LogicalRows: []tmux.LogicalPaneRow{row("%1", "@1"), row("%2", "@2")}}
	killWorthyOpts(m, "%1", born)
	killWorthyOpts(m, "%2", born)

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	// Both windows are killable, but the session must keep its last one.
	if stats.Killed != 1 {
		t.Fatalf("killed = %d, want 1 (last window reserved)", stats.Killed)
	}
	if got := countCalls(m, "KillWindowByID"); got != 1 {
		t.Fatalf("KillWindowByID calls = %d, want 1", got)
	}
}

func TestApplyReapPaneExactDefeatsMergedScopeLeak(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-6*time.Hour).Unix(), 10)
	// Scan shows scope=task/origin=agent (a leak from session/window scope), so
	// PlanReap verdicts Kill — but pane-exact reads carry no scope/origin, so the
	// pane is really a human tab and must NOT die.
	leak := tmux.LogicalPaneRow{
		PaneID: "%1", WindowID: "@1", Session: "work", WindowPanes: 1,
		Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
		WindowActivity: now.Add(-6 * time.Hour),
	}
	keeper := tmux.LogicalPaneRow{
		PaneID: "%2", WindowID: "@2", Session: "work", WindowPanes: 1,
		Command: "bash", Origin: OriginHuman, Born: strconv.FormatInt(now.Unix(), 10),
		WindowActivity: now,
	}
	m := &tmux.MockRunner{LogicalRows: []tmux.LogicalPaneRow{leak, keeper}}
	// pane-exact: only born is real (human-adopted); scope/origin absent.
	m.PaneOptions = map[string]string{"%1\x00" + OptBorn: born}

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Killed != 0 {
		t.Fatalf("killed = %d, want 0 (merged-scope leak must not kill)", stats.Killed)
	}
	if got := countCalls(m, "KillWindowByID"); got != 0 {
		t.Fatalf("KillWindowByID calls = %d, want 0", got)
	}
}

func TestApplyReapAdoptAndFlag(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	unborn := tmux.LogicalPaneRow{
		PaneID: "%1", WindowID: "@1", Session: "work", WindowPanes: 1,
		Command: "bash", Origin: OriginHuman, // Born "" → adopt
	}
	stale := tmux.LogicalPaneRow{
		PaneID: "%2", WindowID: "@2", Session: "work", WindowPanes: 1,
		Command: "bash", Origin: OriginHuman,
		Born:           strconv.FormatInt(now.Add(-6*time.Hour).Unix(), 10),
		WindowActivity: now.Add(-6 * time.Hour), // old, idle, unflagged → flag
	}
	m := &tmux.MockRunner{LogicalRows: []tmux.LogicalPaneRow{unborn, stale}}

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Adopted != 1 || stats.Flagged != 1 || stats.Killed != 0 {
		t.Fatalf("stats = %+v, want 1 adopt / 1 flag / 0 kill", stats)
	}
	if got := countCalls(m, "KillWindowByID"); got != 0 {
		t.Fatalf("KillWindowByID calls = %d, want 0", got)
	}
}

func TestPlanReapCollapsesGroupClones(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	stale := func(pane, win, sess string) tmux.LogicalPaneRow {
		return tmux.LogicalPaneRow{
			PaneID: pane, WindowID: win, Session: sess, SessionGroup: "dev", WindowPanes: 1,
			Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
			WindowActivity: now.Add(-2 * time.Hour),
		}
	}

	// Two windows (@1,@2) shared by clones dev + dev-b → list-panes -a yields 4
	// rows. Must collapse to 2 decisions, both killable (group has 2 windows).
	twoWin := []tmux.LogicalPaneRow{
		stale("%1", "@1", "dev"), stale("%2", "@2", "dev"),
		stale("%1", "@1", "dev-b"), stale("%2", "@2", "dev-b"),
	}
	ds := PlanReap(twoWin, ReapContext{Now: now})
	if len(ds) != 2 {
		t.Fatalf("decisions = %d, want 2 (clones collapsed)", len(ds))
	}
	for _, d := range ds {
		if d.Action != ReapKill {
			t.Fatalf("%s: action %q, want kill", d.PaneID, d.Action)
		}
	}

	// One window (@1) shared by both clones → the group's last window: keep.
	oneWin := []tmux.LogicalPaneRow{stale("%1", "@1", "dev"), stale("%1", "@1", "dev-b")}
	ds = PlanReap(oneWin, ReapContext{Now: now})
	if len(ds) != 1 || ds[0].Action != ReapKeep {
		t.Fatalf("solo group window: got %+v, want one keep (group last window)", ds)
	}
}

func TestApplyReapVetoesKillWithLiveChildren(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	// %1 is a stale agent task (would kill) whose shell has a backgrounded job;
	// %2 is a daemon (kept) so the session keeps >1 window and the children veto
	// — not the last-window guard — is what spares %1.
	stale := tmux.LogicalPaneRow{
		PaneID: "%1", WindowID: "@1", Session: "work", WindowPanes: 1, PanePID: 4242,
		Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
		WindowActivity: now.Add(-2 * time.Hour),
	}
	daemon := tmux.LogicalPaneRow{
		PaneID: "%2", WindowID: "@2", Session: "work", WindowPanes: 1, Scope: ScopeDaemon,
	}
	m := &tmux.MockRunner{
		LogicalRows:  []tmux.LogicalPaneRow{stale, daemon},
		PaneChildren: map[int]bool{4242: true},
	}
	killWorthyOpts(m, "%1", born)

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Killed != 0 {
		t.Fatalf("killed = %d, want 0 (backgrounded job vetoes the kill)", stats.Killed)
	}
	if got := countCalls(m, "KillWindowByID"); got != 0 {
		t.Fatalf("KillWindowByID calls = %d, want 0", got)
	}
}

func TestApplyReapSkipsWindowMovedGroupsSincePlan(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	stale := func(sess string) tmux.LogicalPaneRow {
		return tmux.LogicalPaneRow{
			PaneID: "%1", WindowID: "@1", Session: sess, WindowPanes: 1,
			Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
			WindowActivity: now.Add(-2 * time.Hour),
		}
	}
	daemon := func(pane, win string) tmux.LogicalPaneRow {
		return tmux.LogicalPaneRow{PaneID: pane, WindowID: win, Session: "work", WindowPanes: 1, Scope: ScopeDaemon}
	}
	// Plan sees %1 in group "work" (3 windows → killable). Fresh re-scan sees it
	// moved to a different session "other" where @1 is now alone — killing it on
	// the old "work" budget would empty "other". Must skip.
	plan := []tmux.LogicalPaneRow{stale("work"), daemon("%2", "@2"), daemon("%3", "@3")}
	freshAfterMove := []tmux.LogicalPaneRow{stale("other"), daemon("%2", "@2"), daemon("%3", "@3")}
	m := &tmux.MockRunner{LogicalRowsByCall: [][]tmux.LogicalPaneRow{plan, freshAfterMove}}
	killWorthyOpts(m, "%1", born)

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Killed != 0 {
		t.Fatalf("killed = %d, want 0 (window moved groups since plan)", stats.Killed)
	}
}

func TestApplyReapSkipsVanishedPaneSincePlan(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	born := strconv.FormatInt(now.Add(-2*time.Hour).Unix(), 10)
	stale := tmux.LogicalPaneRow{
		PaneID: "%1", WindowID: "@1", Session: "work", WindowPanes: 1,
		Command: "bash", Origin: OriginAgent, Scope: ScopeTask, Born: born,
		WindowActivity: now.Add(-2 * time.Hour),
	}
	keeper := tmux.LogicalPaneRow{PaneID: "%2", WindowID: "@2", Session: "work", WindowPanes: 1, Scope: ScopeDaemon}
	// Plan flags %1 for kill; by the fresh re-scan it's gone (user closed it).
	plan := []tmux.LogicalPaneRow{stale, keeper}
	gone := []tmux.LogicalPaneRow{keeper}
	m := &tmux.MockRunner{LogicalRowsByCall: [][]tmux.LogicalPaneRow{plan, gone}}
	killWorthyOpts(m, "%1", born)

	stats, err := ApplyReap(m, ReapContext{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Killed != 0 || countCalls(m, "KillWindowByID") != 0 {
		t.Fatalf("killed = %d (kill calls %d), want 0 (pane vanished since plan)", stats.Killed, countCalls(m, "KillWindowByID"))
	}
}

func TestPlanReapAggregatesSessionWindows(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	old := strconv.FormatInt(now.Add(-48*time.Hour).Unix(), 10)
	// One human tab alone in its session — last-window guard must keep it even
	// though it is ancient and idle.
	rows := []tmux.LogicalPaneRow{
		{PaneID: "%1", Session: "solo", WindowID: "@1", WindowPanes: 1, Command: "bash", Origin: OriginHuman, Born: old},
	}
	got := PlanReap(rows, ReapContext{Now: now})
	if len(got) != 1 || got[0].Action != ReapKeep {
		t.Fatalf("solo session tab: got %+v, want keep (last window)", got)
	}
}
