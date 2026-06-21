//go:build integration

package tests

import (
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// TestReapAgainstRealTmux drives ApplyReap against a live tmux server on a
// private socket. Unit tests cover the policy with an injected clock; this locks
// the real round-trips the mocks can't: list-panes parsing the 8 lifecycle
// fields, pane-exact ShowPaneOption re-validation, and KillWindowByID removing
// the right window — plus the apply-level kill budget against real windows.
//
// window_activity is set by tmux to ~now and can't be backdated, so the test
// injects a future ReapContext.Now: every born/activity stamp made at real-now
// then reads as aged. Deterministic, no sleeps.
func TestReapAgainstRealTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	const sock = "zmux-it-reap"
	defer func() { _ = exec.Command("tmux", "-L", sock, "kill-server").Run() }()

	realNow := time.Now()
	born := strconv.FormatInt(realNow.Unix(), 10)
	stamp := func(pane, key, val string) {
		tmuxAt(t, sock, "set-option", "-p", "-t", pane, key, val)
	}
	pane := func(target string) string {
		return tmuxAt(t, sock, "list-panes", "-t", target, "-F", "#{pane_id}")
	}

	// Session "mix": a daemon (keep), a stale agent task (kill), an unborn tab
	// (adopt). 3 windows, so a kill leaves the session alive.
	tmuxAt(t, sock, "new-session", "-d", "-s", "mix", "-n", "w-keep", "-x", "100", "-y", "30")
	tmuxAt(t, sock, "new-window", "-t", "mix", "-n", "w-kill")
	tmuxAt(t, sock, "new-window", "-t", "mix", "-n", "w-adopt")

	stamp(pane("mix:w-keep"), tabs.OptScope, tabs.ScopeDaemon)

	killPane := pane("mix:w-kill")
	stamp(killPane, tabs.OptOrigin, tabs.OriginAgent)
	stamp(killPane, tabs.OptScope, tabs.ScopeTask)
	stamp(killPane, tabs.OptBorn, born)
	// w-adopt: deliberately unstamped → unborn → adopt.

	// Session "allstale": two stale agent tasks; the budget must spare one.
	tmuxAt(t, sock, "new-session", "-d", "-s", "allstale", "-n", "w-a", "-x", "100", "-y", "30")
	tmuxAt(t, sock, "new-window", "-t", "allstale", "-n", "w-b")
	for _, w := range []string{"allstale:w-a", "allstale:w-b"} {
		p := pane(w)
		stamp(p, tabs.OptOrigin, tabs.OriginAgent)
		stamp(p, tabs.OptScope, tabs.ScopeTask)
		stamp(p, tabs.OptBorn, born)
	}

	client := tmux.NewClientFor(tmux.NamedEndpoint(sock))

	// Throttle primitives round-trip (display-message reads the global stamp).
	if got, _ := client.DisplayMessage("", "#{@zmux_last_reap}"); got != "" {
		t.Fatalf("unset throttle stamp = %q, want empty", got)
	}
	if err := client.SetOption("-g", "@zmux_last_reap", "12345"); err != nil {
		t.Fatal(err)
	}
	if got, _ := client.DisplayMessage("", "#{@zmux_last_reap}"); got != "12345" {
		t.Fatalf("throttle stamp = %q, want 12345", got)
	}

	// Reap 48h in the future: every real-now stamp reads as long aged/idle.
	stats, err := tabs.ApplyReap(client, tabs.ReapContext{Now: realNow.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Killed != 2 {
		t.Fatalf("killed = %d, want 2 (mix:w-kill + one of allstale)", stats.Killed)
	}
	if stats.Adopted != 1 {
		t.Fatalf("adopted = %d, want 1 (mix:w-adopt)", stats.Adopted)
	}

	windows := func(sess string) []string {
		out := tmuxAt(t, sock, "list-windows", "-t", sess, "-F", "#{window_name}")
		if out == "" {
			return nil
		}
		return splitLines(out)
	}

	mix := windows("mix")
	if has(mix, "w-kill") {
		t.Fatalf("mix:w-kill survived: %v", mix)
	}
	if !has(mix, "w-keep") {
		t.Fatalf("mix:w-keep (daemon) was reaped: %v", mix)
	}
	if !has(mix, "w-adopt") {
		t.Fatalf("mix:w-adopt was killed instead of adopted: %v", mix)
	}
	// Adopt stamped born pane-exactly.
	if got := tmuxAt(t, sock, "display-message", "-p", "-t", "mix:w-adopt", "#{@zmux_born}"); got == "" {
		t.Fatal("adopt did not stamp @zmux_born")
	}

	if got := len(windows("allstale")); got != 1 {
		t.Fatalf("allstale window count = %d, want 1 (budget spares last)", got)
	}
}

// TestReapGroupedSessionAgainstRealTmux proves the session-GROUP kill budget
// against real linked windows (codex review #3). A grouped session's windows are
// shared across clones; list-panes -a repeats each pane once per clone, and a
// kill removes the shared window from all of them. The reaper must count + reserve
// by group, killing one window and sparing the group's last.
func TestReapGroupedSessionAgainstRealTmux(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	const sock = "zmux-it-reap-grp"
	defer func() { _ = exec.Command("tmux", "-L", sock, "kill-server").Run() }()

	realNow := time.Now()
	born := strconv.FormatInt(realNow.Unix(), 10)

	// Group "g" with two windows, cloned into "g-b" (shares the window list).
	tmuxAt(t, sock, "new-session", "-d", "-s", "g", "-n", "w1", "-x", "100", "-y", "30")
	tmuxAt(t, sock, "new-window", "-t", "g", "-n", "w2")
	tmuxAt(t, sock, "new-session", "-d", "-t", "g", "-s", "g-b")

	for _, w := range []string{"g:w1", "g:w2"} {
		p := tmuxAt(t, sock, "list-panes", "-t", w, "-F", "#{pane_id}")
		tmuxAt(t, sock, "set-option", "-p", "-t", p, tabs.OptOrigin, tabs.OriginAgent)
		tmuxAt(t, sock, "set-option", "-p", "-t", p, tabs.OptScope, tabs.ScopeTask)
		tmuxAt(t, sock, "set-option", "-p", "-t", p, tabs.OptBorn, born)
	}

	client := tmux.NewClientFor(tmux.NamedEndpoint(sock))
	stats, err := tabs.ApplyReap(client, tabs.ReapContext{Now: realNow.Add(48 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	// Two shared windows, both stale → exactly one killed (group budget), not two
	// (which would have happened with per-session-name double counting).
	if stats.Killed != 1 {
		t.Fatalf("killed = %d, want 1 (group keeps its last window)", stats.Killed)
	}
	// The kill is reflected in BOTH clones (linked windows).
	for _, sess := range []string{"g", "g-b"} {
		if got := len(splitLines(tmuxAt(t, sock, "list-windows", "-t", sess, "-F", "#{window_id}"))); got != 1 {
			t.Fatalf("%s window count = %d, want 1", sess, got)
		}
	}
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func has(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
