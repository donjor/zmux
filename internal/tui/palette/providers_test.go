package palette

import (
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// ── SessionsProvider ──

// TestSessionsProviderTitlesUseLabelNotRawName guards the display fix: titles
// show the workspace-local label, never the generated zws_… tmux name, while the
// switch/kill payloads keep the raw name as the target.
func TestSessionsProviderTitlesUseLabelNotRawName(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{
		{Name: "zws_proj__main", Managed: true, Workspace: "proj", SessionLabel: "main"},
	}

	got, err := (&SessionsProvider{Runner: mock}).Actions()
	if err != nil {
		t.Fatalf("Actions() error: %v", err)
	}

	var sawSwitch, sawKill bool
	for _, a := range got {
		switch p := a.Payload.(type) {
		case SessionSwitchPayload:
			sawSwitch = true
			if strings.Contains(a.Title, "zws_") {
				t.Errorf("switch title leaks raw name: %q", a.Title)
			}
			if a.Title != "Switch to proj/main" {
				t.Errorf("switch title = %q, want %q", a.Title, "Switch to proj/main")
			}
			if p.Name != "zws_proj__main" {
				t.Errorf("switch payload target = %q, want raw name", p.Name)
			}
		case SessionKillPayload:
			sawKill = true
			if strings.Contains(a.Title, "zws_") {
				t.Errorf("kill title leaks raw name: %q", a.Title)
			}
			if p.Name != "zws_proj__main" {
				t.Errorf("kill payload target = %q, want raw name", p.Name)
			}
		}
	}
	if !sawSwitch || !sawKill {
		t.Fatalf("expected switch+kill actions; switch=%v kill=%v", sawSwitch, sawKill)
	}
}

func TestSessionsProviderEmits1NewPlusNSwitchPlusNKill(t *testing.T) {
	mock := tmux.NewMockRunner()
	now := time.Now()
	mock.Sessions = []tmux.Session{
		{Name: "dev", Activity: now},
		{Name: "api", Activity: now},
	}

	p := &SessionsProvider{Runner: mock}
	got, err := p.Actions()
	if err != nil {
		t.Fatalf("Actions() error: %v", err)
	}

	// 1 new + 2 switch + 2 kill = 5.
	if len(got) != 5 {
		t.Errorf("want 5 actions (1 new + 2 switch + 2 kill), got %d", len(got))
	}

	var newCount, switchCount, killCount int
	for _, a := range got {
		switch {
		case a.ID == "session:new":
			newCount++
		case strings.HasPrefix(a.ID, "session:switch:"):
			switchCount++
		case strings.HasPrefix(a.ID, "session:kill:"):
			killCount++
		}
	}
	if newCount != 1 || switchCount != 2 || killCount != 2 {
		t.Errorf("counts: new=%d switch=%d kill=%d", newCount, switchCount, killCount)
	}
}

func TestSessionsProviderAttachedSubtitle(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{
		{Name: "dev", Attached: true, Activity: time.Now()},
	}

	p := &SessionsProvider{Runner: mock}
	got, _ := p.Actions()

	found := false
	for _, a := range got {
		if strings.HasPrefix(a.ID, "session:switch:") && a.Subtitle == "attached" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected attached session to get 'attached' subtitle, got: %+v", got)
	}
}

func TestSessionsProviderRunnerError(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Err = &testErr{"can't list"}
	p := &SessionsProvider{Runner: mock}

	_, err := p.Actions()
	if err == nil {
		t.Fatal("expected error from failing runner, got nil")
	}
}

// ── ThemesProvider ──

func TestThemesProviderEmitsOnePerBundledTheme(t *testing.T) {
	// Resolver pointing at nothing — returns just the bundled themes.
	resolver := theme.NewResolver(&fakeFS{home: "/home/test"}, "", "")
	p := &ThemesProvider{Resolver: resolver}

	got, err := p.Actions()
	if err != nil {
		t.Fatalf("Actions() error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected bundled themes, got none")
	}
	for _, a := range got {
		if !strings.HasPrefix(a.ID, "theme:set:") {
			t.Errorf("theme action has wrong ID prefix: %s", a.ID)
		}
		if a.Group != "Themes" {
			t.Errorf("group = %q, want Themes", a.Group)
		}
		if a.Subtitle != "dark" && a.Subtitle != "light" {
			t.Errorf("subtitle = %q, want dark or light", a.Subtitle)
		}
	}
}

func TestThemesProviderNilResolverReturnsEmpty(t *testing.T) {
	p := &ThemesProvider{Resolver: nil}
	got, err := p.Actions()
	if err != nil {
		t.Errorf("nil resolver error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("nil resolver actions = %d, want 0", len(got))
	}
}

// ── BarProvider ──

func TestBarProviderEmitsOnePerPreset(t *testing.T) {
	p := &BarProvider{}
	got, err := p.Actions()
	if err != nil {
		t.Fatalf("Actions() error: %v", err)
	}
	if len(got) < 5 {
		t.Errorf("expected many bar presets, got %d", len(got))
	}
	for _, a := range got {
		if !strings.HasPrefix(a.ID, "bar:set:") {
			t.Errorf("bar action wrong ID prefix: %s", a.ID)
		}
		if a.Group != "Bar" {
			t.Errorf("group = %q, want Bar", a.Group)
		}
		if _, ok := a.Payload.(BarSetPayload); !ok {
			t.Errorf("payload type = %T, want BarSetPayload", a.Payload)
		}
	}
}

// ── DashboardProvider ──

func TestDashboardProviderStatic(t *testing.T) {
	p := &DashboardProvider{}
	got, _ := p.Actions()

	// Expect four static actions: current/sessions/settings/help.
	if len(got) != 4 {
		t.Errorf("DashboardProvider actions = %d, want 4", len(got))
	}
	for _, a := range got {
		if a.Kind != ActionOpenDashboard {
			t.Errorf("action %s: kind = %v, want ActionOpenDashboard", a.ID, a.Kind)
		}
		if _, ok := a.Payload.(DashboardTabPayload); !ok {
			t.Errorf("action %s: payload = %T, want DashboardTabPayload", a.ID, a.Payload)
		}
	}
}

// ── HelpProvider ──

func TestHelpProviderHasOneAction(t *testing.T) {
	p := &HelpProvider{}
	got, _ := p.Actions()
	if len(got) != 1 {
		t.Fatalf("HelpProvider actions = %d, want 1", len(got))
	}
	if got[0].Kind != ActionOpenDashboard {
		t.Errorf("help kind = %v, want ActionOpenDashboard", got[0].Kind)
	}
}

// ── OvermindProvider ──
//
// Overmind actions depend on `source.Discover()` touching the live
// system, so we just sanity-check that it doesn't panic and returns
// a slice (possibly nil) without an error when no overmind processes
// exist.

func TestOvermindProviderNoPanicOnEmptyCatalog(t *testing.T) {
	p := &OvermindProvider{}
	got, err := p.Actions()
	// Discover is best-effort; the expected empty-env result is either
	// (nil, nil) or ([]Action{}, nil).
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_ = got
}

// ── NewDefaultRegistry smoke ──

func TestNewDefaultRegistryWiresAllProviders(t *testing.T) {
	mock := tmux.NewMockRunner()
	resolver := theme.NewResolver(&fakeFS{home: "/home/test"}, "", "")
	fs := &fakeFS{home: "/home/test"}

	r := NewDefaultRegistry(mock, resolver, fs)
	if r == nil {
		t.Fatal("NewDefaultRegistry returned nil")
	}
	actions := r.All()
	if len(actions) == 0 {
		t.Error("default registry returned no actions")
	}

	// Must include at least one from each of the stable providers
	// (sessions/themes/bar/dashboard/help). Overmind may be empty.
	haveGroup := func(g string) bool {
		for _, a := range actions {
			if a.Group == g {
				return true
			}
		}
		return false
	}
	for _, g := range []string{"Sessions", "Themes", "Bar", "Dashboard", "Help"} {
		if !haveGroup(g) {
			t.Errorf("default registry missing group %q", g)
		}
	}
}

// ── shared test types ──

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
