package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/snapshot"
	"github.com/donjor/zmux/internal/tmux"
)

func TestResolvePaneTargetsExplicit(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		panes         map[string][]tmux.Pane
		logicalRows   []tmux.LogicalPaneRow
		want          []snapshot.PaneTarget
		noLogicalScan bool
	}{
		{
			name: "label resolves through tab target",
			args: []string{"sim"},
			panes: map[string][]tmux.Pane{
				"": {{ID: "%8", Command: "node"}},
			},
			logicalRows: []tmux.LogicalPaneRow{
				logicalRow("%8", "test-session", "@4", 2, "ztab_sim01", "sim"),
			},
			want: []snapshot.PaneTarget{{Name: "sim", PaneID: "%8"}},
		},
		{
			name: "raw pane id bypasses tab resolver",
			args: []string{"%5"},
			panes: map[string][]tmux.Pane{
				"": {{ID: "%5", Command: "ssh"}},
			},
			logicalRows: []tmux.LogicalPaneRow{
				logicalRow("%5", "test-session", "@5", 1, "ztab_raw01", "not-used"),
			},
			want:          []snapshot.PaneTarget{{Name: "ssh", PaneID: "%5"}},
			noLogicalScan: true,
		},
		{
			name: "unresolved label falls through to scoped tmux target",
			args: []string{"missing"},
			panes: map[string][]tmux.Pane{
				"": {{ID: "%1", Command: "shell"}},
			},
			want: []snapshot.PaneTarget{{Name: "missing", PaneID: "test-session:missing"}},
		},
		{
			name: "dedupe still de-collides raw command names",
			args: []string{"%1", "%2", "  "},
			panes: map[string][]tmux.Pane{
				"": {{ID: "%1", Command: "vim"}, {ID: "%2", Command: "vim"}},
			},
			want:          []snapshot.PaneTarget{{Name: "vim", PaneID: "%1"}, {Name: "vim-2", PaneID: "%2"}},
			noLogicalScan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, mock := newTestApp(t)
			mock.Panes = tt.panes
			mock.LogicalRows = tt.logicalRows

			targets, explicit, err := resolvePaneTargets(app, "test-session", tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if !explicit {
				t.Error("explicit should be true when --pane given")
			}
			if !reflect.DeepEqual(targets, tt.want) {
				t.Fatalf("targets = %#v, want %#v", targets, tt.want)
			}
			if tt.noLogicalScan && mockCalled(mock, "ListLogicalPaneRows") {
				t.Fatal("raw pane ids must bypass resolveTabTarget")
			}
		})
	}
}

func TestResolvePaneTargetsDefaultCurrentWindow(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Panes = map[string][]tmux.Pane{
		"": {{ID: "%5", Command: "server"}, {ID: "%6", Command: ""}},
	}
	targets, explicit, err := resolvePaneTargets(app, "", nil)
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

func mockCalled(mock *tmux.MockRunner, method string) bool {
	for _, call := range mock.Calls {
		if call.Method == method {
			return true
		}
	}
	return false
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
