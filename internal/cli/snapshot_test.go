package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/snapshot"
	"github.com/donjor/zmux/internal/tmux"
)

func TestResolvePaneTargetsExplicitDedup(t *testing.T) {
	runner := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"": {{ID: "%1", Command: "vim"}, {ID: "%2", Command: "vim"}},
	}}
	targets, explicit, err := resolvePaneTargets(runner, []string{"%1", "%2", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if !explicit {
		t.Error("explicit should be true when --pane given")
	}
	if len(targets) != 2 {
		t.Fatalf("got %d targets, want 2 (empty id skipped)", len(targets))
	}
	if targets[0].Name == targets[1].Name {
		t.Errorf("names must be de-duplicated: %q == %q", targets[0].Name, targets[1].Name)
	}
	if targets[0].PaneID != "%1" || targets[1].PaneID != "%2" {
		t.Errorf("pane ids = %q,%q", targets[0].PaneID, targets[1].PaneID)
	}
}

func TestResolvePaneTargetsDefaultCurrentWindow(t *testing.T) {
	runner := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"": {{ID: "%5", Command: "server"}, {ID: "%6", Command: ""}},
	}}
	targets, explicit, err := resolvePaneTargets(runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if explicit {
		t.Error("explicit should be false with no --pane")
	}
	if len(targets) != 2 {
		t.Fatalf("got %d targets, want 2", len(targets))
	}
	if targets[0].Name != "server" {
		t.Errorf("name from command = %q, want server", targets[0].Name)
	}
	if targets[1].Name != "%6" {
		t.Errorf("command-less pane name = %q, want fallback to id %%6", targets[1].Name)
	}
}

func TestAllPanesInCurrentWindow(t *testing.T) {
	runner := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"": {{ID: "%1"}, {ID: "%2"}},
	}}
	if ok, _ := allPanesInCurrentWindow(runner, []snapshot.PaneTarget{{PaneID: "%1"}, {PaneID: "%2"}}); !ok {
		t.Error("all panes in window should be ok")
	}
	ok, reason := allPanesInCurrentWindow(runner, []snapshot.PaneTarget{{PaneID: "%1"}, {PaneID: "%9"}})
	if ok {
		t.Error("off-window pane %9 should fail the guard")
	}
	if !strings.Contains(reason, "%9") {
		t.Errorf("reason should name the offending pane, got %q", reason)
	}
}

func TestResolveOutDir(t *testing.T) {
	got := resolveOutDir("/home/u/.zmux/snapshots", "")
	if !strings.HasPrefix(got, "/home/u/.zmux/snapshots/") {
		t.Errorf("default out dir = %q, want under ~/.zmux/snapshots/", got)
	}
	if strings.ContainsAny(strings.TrimPrefix(got, "/home/u/.zmux/snapshots/"), ":.") {
		t.Errorf("stamp segment not filesystem-safe: %q", got)
	}

	got = resolveOutDir("/home/u/.zmux/snapshots", "/tmp/custom")
	if got != "/tmp/custom" {
		t.Errorf("explicit out = %q, want /tmp/custom", got)
	}
}
