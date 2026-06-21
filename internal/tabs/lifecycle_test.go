package tabs

import (
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// writeFor returns the value written for key by an ApplyOptions call, and
// whether any write touched it (Args: scope, target, key, value, "unset=..").
func writeFor(m *tmux.MockRunner, key string) (value string, unset bool, found bool) {
	for _, c := range m.Calls {
		if c.Method == "ApplyOptions" && len(c.Args) >= 5 && c.Args[2] == key {
			return c.Args[3], c.Args[4] == "unset=true", true
		}
	}
	return "", false, false
}

func TestStampBirthFreshWritesIdentity(t *testing.T) {
	m := tmux.NewMockRunner()
	now := time.Unix(1000, 0)
	if err := StampBirth(m, "%1", OriginAgent, ScopeTask, now); err != nil {
		t.Fatalf("StampBirth: %v", err)
	}
	if v, _, ok := writeFor(m, OptBorn); !ok || v != "1000" {
		t.Fatalf("born = %q (found=%v), want 1000", v, ok)
	}
	if v, _, ok := writeFor(m, OptOrigin); !ok || v != OriginAgent {
		t.Fatalf("origin = %q (found=%v), want agent", v, ok)
	}
	if v, _, ok := writeFor(m, OptScope); !ok || v != ScopeTask {
		t.Fatalf("scope = %q (found=%v), want task", v, ok)
	}
}

func TestStampBirthIdempotentOnReuse(t *testing.T) {
	m := tmux.NewMockRunner()
	m.PaneOptions = map[string]string{"%1\x00" + OptBorn: "500"} // already born
	if err := StampBirth(m, "%1", OriginAgent, ScopeTask, time.Unix(9999, 0)); err != nil {
		t.Fatalf("StampBirth: %v", err)
	}
	// No write should have happened — identity is immutable once born.
	if _, _, ok := writeFor(m, OptBorn); ok {
		t.Fatal("re-stamped born on an already-born pane")
	}
	if _, _, ok := writeFor(m, OptOrigin); ok {
		t.Fatal("re-stamped origin on an already-born pane")
	}
}

func TestMarkAgentShellFreshStampsIdentity(t *testing.T) {
	m := tmux.NewMockRunner()
	if err := MarkAgentShell(m, "%1", time.Unix(1000, 0)); err != nil {
		t.Fatalf("MarkAgentShell: %v", err)
	}
	if v, _, ok := writeFor(m, OptOrigin); !ok || v != OriginAgent {
		t.Fatalf("origin = %q (found=%v), want agent", v, ok)
	}
	if v, _, ok := writeFor(m, OptScope); !ok || v != ScopeAgentShell {
		t.Fatalf("scope = %q (found=%v), want agent-shell", v, ok)
	}
	if v, _, ok := writeFor(m, OptBorn); !ok || v != "1000" {
		t.Fatalf("born = %q (found=%v), want 1000 stamped when unborn", v, ok)
	}
}

func TestMarkAgentShellUpgradesAdoptedShell(t *testing.T) {
	m := tmux.NewMockRunner()
	// Adopted earlier as a plain preexisting shell, born at 500.
	m.PaneOptions = map[string]string{
		"%1\x00" + OptBorn:   "500",
		"%1\x00" + OptOrigin: OriginPreexisting,
		"%1\x00" + OptScope:  ScopeShell,
	}
	if err := MarkAgentShell(m, "%1", time.Unix(9999, 0)); err != nil {
		t.Fatalf("MarkAgentShell: %v", err)
	}
	// Scope/origin are UPGRADED (override, unlike set-once StampBirth)...
	if v, _, ok := writeFor(m, OptScope); !ok || v != ScopeAgentShell {
		t.Fatalf("scope = %q (found=%v), want upgraded to agent-shell", v, ok)
	}
	if v, _, ok := writeFor(m, OptOrigin); !ok || v != OriginAgent {
		t.Fatalf("origin = %q (found=%v), want upgraded to agent", v, ok)
	}
	// ...but born is preserved (no re-stamp), keeping the original age clock.
	if _, _, ok := writeFor(m, OptBorn); ok {
		t.Fatal("re-stamped born on an already-born pane; age clock must be preserved")
	}
}

func TestResolveOrigin(t *testing.T) {
	cases := []struct {
		name                                 string
		flag, callerOrigin, callerScope, env string
		want                                 string
	}{
		{"explicit flag wins", OriginHuman, OriginAgent, ScopeAgentShell, OriginAgent, OriginHuman},
		{"caller origin agent", "", OriginAgent, "", "", OriginAgent},
		{"caller scope agent-shell", "", "", ScopeAgentShell, "", OriginAgent},
		{"env actor agent", "", OriginHuman, ScopeShell, OriginAgent, OriginAgent},
		{"default human", "", "", "", "", OriginHuman},
		{"bogus flag ignored, falls through", "garbage", "", "", "", OriginHuman},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveOrigin(tc.flag, tc.callerOrigin, tc.callerScope, tc.env); got != tc.want {
				t.Fatalf("ResolveOrigin = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSetTTL(t *testing.T) {
	m := tmux.NewMockRunner()
	if err := SetTTL(m, "%1", 90*time.Second); err != nil {
		t.Fatal(err)
	}
	if v, _, ok := writeFor(m, OptTTL); !ok || v != "90" {
		t.Fatalf("ttl = %q (found=%v), want 90", v, ok)
	}

	m2 := tmux.NewMockRunner()
	if err := SetTTL(m2, "%1", 0); err != nil {
		t.Fatal(err)
	}
	if _, unset, ok := writeFor(m2, OptTTL); !ok || !unset {
		t.Fatalf("ttl<=0 should unset, got found=%v unset=%v", ok, unset)
	}
}

func TestSetKeep(t *testing.T) {
	m := tmux.NewMockRunner()
	if err := SetKeep(m, "%1", true); err != nil {
		t.Fatal(err)
	}
	if v, _, ok := writeFor(m, OptKeep); !ok || v != "1" {
		t.Fatalf("keep = %q (found=%v), want 1", v, ok)
	}
	m2 := tmux.NewMockRunner()
	if err := SetKeep(m2, "%1", false); err != nil {
		t.Fatal(err)
	}
	if _, unset, ok := writeFor(m2, OptKeep); !ok || !unset {
		t.Fatalf("keep=false should unset, got found=%v unset=%v", ok, unset)
	}
}

func TestParseUnixAndTTL(t *testing.T) {
	if ts, ok := ParseUnix("1000"); !ok || !ts.Equal(time.Unix(1000, 0)) {
		t.Fatalf("ParseUnix(1000) = %v,%v", ts, ok)
	}
	if _, ok := ParseUnix(""); ok {
		t.Fatal("ParseUnix(empty) should be !ok")
	}
	if _, ok := ParseUnix("nope"); ok {
		t.Fatal("ParseUnix(malformed) should be !ok")
	}
	if d, ok := ParseTTL("3600"); !ok || d != time.Hour {
		t.Fatalf("ParseTTL(3600) = %v,%v", d, ok)
	}
	if _, ok := ParseTTL("0"); ok {
		t.Fatal("ParseTTL(0) should be !ok")
	}
	if _, ok := ParseTTL("-5"); ok {
		t.Fatal("ParseTTL(negative) should be !ok")
	}
}
