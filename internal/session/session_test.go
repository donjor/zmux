package session

import (
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

func TestIsTemp(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"tmp-1", true},
		{"tmp-42", true},
		{"tmp-0", true},
		{"tmp-", false},
		{"tmp", false},
		{"dev", false},
		{"my-tmp-1", false},
		{"tmp-1-extra", false},
		{"TMP-1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTemp(tt.name)
			if got != tt.want {
				t.Errorf("IsTemp(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNextTmpName(t *testing.T) {
	tests := []struct {
		desc     string
		sessions []tmux.Session
		want     string
	}{
		{
			desc: "no sessions",
			want: "tmp-1",
		},
		{
			desc: "no tmp sessions",
			sessions: []tmux.Session{
				{Name: "dev"},
				{Name: "work"},
			},
			want: "tmp-1",
		},
		{
			desc: "existing tmp sessions",
			sessions: []tmux.Session{
				{Name: "tmp-1"},
				{Name: "tmp-3"},
				{Name: "dev"},
			},
			want: "tmp-4",
		},
		{
			desc: "sequential tmp sessions",
			sessions: []tmux.Session{
				{Name: "tmp-1"},
				{Name: "tmp-2"},
			},
			want: "tmp-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			m := tmux.NewMockRunner()
			m.Sessions = tt.sessions
			got := NextTmpName(m)
			if got != tt.want {
				t.Errorf("NextTmpName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanAge(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		desc string
		t    time.Time
		want string
	}{
		{"seconds ago", now.Add(-30 * time.Second), "30s"},
		{"minutes ago", now.Add(-5 * time.Minute), "5m"},
		{"hours ago", now.Add(-2 * time.Hour), "2h"},
		{"day ago", now.Add(-36 * time.Hour), "1d"},
		{"days ago", now.Add(-3 * 24 * time.Hour), "3d"},
		{"week ago", now.Add(-10 * 24 * time.Hour), "1w"},
		{"weeks ago", now.Add(-21 * 24 * time.Hour), "3w"},
		{"zero duration", now, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := HumanAgeSince(tt.t, now)
			if got != tt.want {
				t.Errorf("HumanAgeSince(%v, %v) = %q, want %q", tt.t, now, got, tt.want)
			}
		})
	}
}

func TestListSessions(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "tmp-2", Windows: 1, Attached: false, Activity: now, Dir: "/tmp"},
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Dir: "/home/user/work"},
		{Name: "tmp-1", Windows: 1, Attached: true, Activity: now, Dir: "/tmp"},
		{Name: "alpha", Windows: 2, Attached: false, Activity: now, Dir: "/home/user"},
	}

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	if len(sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sessions))
	}

	// Named sessions should come first, alphabetically.
	if sessions[0].Name != "alpha" {
		t.Errorf("expected first session to be 'alpha', got %q", sessions[0].Name)
	}
	if sessions[1].Name != "dev" {
		t.Errorf("expected second session to be 'dev', got %q", sessions[1].Name)
	}

	// Tmp sessions come after, alphabetically.
	if sessions[2].Name != "tmp-1" {
		t.Errorf("expected third session to be 'tmp-1', got %q", sessions[2].Name)
	}
	if sessions[3].Name != "tmp-2" {
		t.Errorf("expected fourth session to be 'tmp-2', got %q", sessions[3].Name)
	}

	// Verify IsTmp flag.
	if sessions[0].IsTmp {
		t.Error("'alpha' should not be marked as tmp")
	}
	if !sessions[2].IsTmp {
		t.Error("'tmp-1' should be marked as tmp")
	}

	// Verify attached state.
	if !sessions[1].Attached {
		t.Error("'dev' should be attached")
	}
	if sessions[3].Attached {
		t.Error("'tmp-2' should not be attached")
	}
}

// Reserved zmux-internal sessions (the hidden-tab dock) never surface in
// the enriched list — this is the collapse point ls/pickers/bar flow through.
func TestListSessionsFiltersReserved(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now},
		{Name: "__zmux_dock", Windows: 2, Attached: false, Activity: now},
	}

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "dev" {
		t.Fatalf("expected only 'dev', got %+v", sessions)
	}
}

func TestListSessionsError(t *testing.T) {
	m := tmux.NewMockRunner()
	m.Err = errTestError

	_, err := ListSessions(m)
	if err == nil {
		t.Fatal("expected error from ListSessions")
	}
}

func TestListSessionsCollapsesGrouped(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "zmux", Windows: 2, Attached: true, Activity: now, Dir: "/home/user/zmux", Group: "zmux"},
		{Name: "zmux-b", Windows: 2, Attached: true, Activity: now, Dir: "/home/user/zmux", Group: "zmux"},
		{Name: "dev", Windows: 3, Attached: false, Activity: now, Dir: "/home/user/work"},
	}

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	// Should collapse zmux + zmux-b into one entry.
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (zmux collapsed, dev), got %d", len(sessions))
	}

	// Find zmux entry.
	var zmuxSession *SessionInfo
	for i := range sessions {
		if sessions[i].Name == "zmux" {
			zmuxSession = &sessions[i]
			break
		}
	}
	if zmuxSession == nil {
		t.Fatal("expected 'zmux' session in results")
	}
	if zmuxSession.AttachedClients != 2 {
		t.Errorf("expected AttachedClients=2, got %d", zmuxSession.AttachedClients)
	}

	// zmux-b should NOT appear.
	for _, s := range sessions {
		if s.Name == "zmux-b" {
			t.Error("grouped session 'zmux-b' should not appear in list")
		}
	}
}

// Regression: the roots map used to hold pointers into the infos slice; an
// append reallocation between root registration and clone merge stranded the
// pointer, silently dropping AttachedClients increments. Enough sessions
// between root and clone forces the realloc.
func TestListSessionsGroupedCountSurvivesRealloc(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 1, Attached: true, Activity: now, Group: "dev"},
	}
	for _, n := range []string{"m1", "m2", "m3", "m4", "m5", "m6", "m7"} {
		m.Sessions = append(m.Sessions,
			tmux.Session{Name: n, Windows: 1, Activity: now})
	}
	m.Sessions = append(m.Sessions,
		tmux.Session{Name: "dev-b", Windows: 1, Attached: true, Activity: now, Group: "dev"})

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	for _, s := range sessions {
		if s.Name == "dev" && s.AttachedClients != 2 {
			t.Errorf("dev AttachedClients = %d, want 2 (clone increment lost to realloc)", s.AttachedClients)
		}
	}
}

func TestListSessionsIncludesPinnedGroupedViews(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "zmux", Windows: 2, Attached: true, Activity: now, Dir: "/home/user/zmux", Group: "zmux", SessionLabel: "main"},
		{Name: "zmux-b", Windows: 2, Attached: false, Activity: now, Dir: "/home/user/zmux", Group: "zmux", Clone: true, PinnedView: true, ViewRoot: "zmux", SessionLabel: "main"},
	}

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected root plus pinned view, got %d: %#v", len(sessions), sessions)
	}
	var pinned *SessionInfo
	for i := range sessions {
		if sessions[i].Name == "zmux-b" {
			pinned = &sessions[i]
		}
	}
	if pinned == nil {
		t.Fatal("expected pinned grouped view in session list")
	}
	if !pinned.PinnedView || pinned.ViewRoot != "zmux" {
		t.Fatalf("pinned metadata = pinned:%v root:%q", pinned.PinnedView, pinned.ViewRoot)
	}
	if got := LocalDisplayName(*pinned); got != "main · view b" {
		t.Fatalf("display label = %q; want main · view b", got)
	}
	pinned.Workspace = "dev"
	if got := QualifiedDisplayName(*pinned); got != "dev/main · view b" {
		t.Fatalf("qualified display label = %q; want dev/main · view b", got)
	}
}

func TestListSessionsUngroupedSuffixNotCollapsed(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	// "work-b" exists but "work" does NOT — should not be collapsed.
	m.Sessions = []tmux.Session{
		{Name: "work-b", Windows: 2, Attached: false, Activity: now, Dir: "/home/user"},
		{Name: "dev", Windows: 1, Attached: false, Activity: now, Dir: "/home/user"},
	}

	sessions, err := ListSessions(m)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

var errTestError = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }
