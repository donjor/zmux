package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func TestSetTabLabelSetsManualLabel(t *testing.T) {
	a, mock := newTestApp(t)
	cmd, out := outputCommand()
	if err := setTabLabel(a, cmd, "@11", "pi", false); err != nil {
		t.Fatalf("setTabLabel failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "SetWindowOption" || mock.Calls[0].Args[0] != "@11" || mock.Calls[0].Args[1] != tablabel.Option || mock.Calls[0].Args[2] != "pi" {
		t.Fatalf("unexpected calls: %#v", mock.Calls)
	}
	if mock.Calls[1].Method != "SetWindowOption" || mock.Calls[1].Args[1] != tablabel.SourceOption || mock.Calls[1].Args[2] != tablabel.SourceManual {
		t.Fatalf("expected manual source call, got %#v", mock.Calls[1])
	}
	if !strings.Contains(out.String(), "tab label: pi") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestSetTabLabelClearsBlankLabel(t *testing.T) {
	a, mock := newTestApp(t)
	cmd, _ := outputCommand()
	if err := setTabLabel(a, cmd, "@11", "", false); err != nil {
		t.Fatalf("setTabLabel clear failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "UnsetWindowOption" || mock.Calls[0].Args[1] != tablabel.Option || mock.Calls[1].Args[1] != tablabel.SourceOption {
		t.Fatalf("expected unset calls, got %#v", mock.Calls)
	}
}

func TestRefreshDuplicateWindowNameMarkersMarksOnlyDuplicateNames(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Windows["dev"] = []tmux.Window{
		{Index: 1, Name: "pi", Dir: "/home/user/zmux"},
		{Index: 2, Name: "pi", Dir: "/home/user/pi-extensions"},
		{Index: 3, Name: "bash", Dir: "/home/user/zmux"},
	}

	if err := refreshDuplicateWindowNameMarkersForSession(a, "dev"); err != nil {
		t.Fatalf("refreshDuplicateWindowNameMarkersForSession failed: %v", err)
	}

	var setTargets, unsetTargets []string
	for _, call := range mock.Calls {
		switch call.Method {
		case "SetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption && call.Args[2] == "1" {
				setTargets = append(setTargets, call.Args[0])
			}
		case "UnsetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption {
				unsetTargets = append(unsetTargets, call.Args[0])
			}
		}
	}
	if strings.Join(setTargets, ",") != "dev:1,dev:2" {
		t.Fatalf("duplicate targets = %v, want dev:1/dev:2 (calls=%#v)", setTargets, mock.Calls)
	}
	if strings.Join(unsetTargets, ",") != "dev:3" {
		t.Fatalf("unset targets = %v, want dev:3 (calls=%#v)", unsetTargets, mock.Calls)
	}
}

// Two same-named tabs that also share a cwd basename get NO marker: the
// [basename] bracket would render identically on both and differentiate
// nothing (the window index already does). This is the common same-worktree
// case — e.g. two "claude" tabs both opened in the same repo.
func TestRefreshDuplicateWindowNameMarkersSuppressesSharedCwd(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Windows["dev"] = []tmux.Window{
		{Index: 1, Name: "claude", Dir: "/home/user/skills"},
		{Index: 2, Name: "claude", Dir: "/home/user/skills"}, // same name + same cwd
		{Index: 3, Name: "bun", Dir: "/home/user/proj/api"},
		{Index: 4, Name: "bun", Dir: "/home/user/proj/web"}, // same name, DIFFERENT cwd
	}

	if err := refreshDuplicateWindowNameMarkersForSession(a, "dev"); err != nil {
		t.Fatalf("refreshDuplicateWindowNameMarkersForSession failed: %v", err)
	}

	var setTargets, unsetTargets []string
	for _, call := range mock.Calls {
		switch call.Method {
		case "SetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption {
				setTargets = append(setTargets, call.Args[0])
			}
		case "UnsetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption {
				unsetTargets = append(unsetTargets, call.Args[0])
			}
		}
	}
	// claude/claude share a cwd basename -> suppressed; bun/bun differ -> marked.
	if strings.Join(setTargets, ",") != "dev:3,dev:4" {
		t.Fatalf("set targets = %v, want dev:3,dev:4 (differing cwd)", setTargets)
	}
	if strings.Join(unsetTargets, ",") != "dev:1,dev:2" {
		t.Fatalf("unset targets = %v, want dev:1,dev:2 (shared cwd)", unsetTargets)
	}
}

// Three same-named tabs: two share a cwd basename, one is unique. Only the
// unique one earns the bracket — the shared pair would render an identical
// [api] on both, so they stay bare and rely on the index.
func TestRefreshDuplicateWindowNameMarkersMixedCwd(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Windows["dev"] = []tmux.Window{
		{Index: 1, Name: "bun", Dir: "/home/user/proj/api"},
		{Index: 2, Name: "bun", Dir: "/home/user/proj/api"}, // shares basename with :1
		{Index: 3, Name: "bun", Dir: "/home/user/proj/web"}, // unique basename
	}

	if err := refreshDuplicateWindowNameMarkersForSession(a, "dev"); err != nil {
		t.Fatalf("refreshDuplicateWindowNameMarkersForSession failed: %v", err)
	}

	var setTargets, unsetTargets []string
	for _, call := range mock.Calls {
		switch call.Method {
		case "SetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption {
				setTargets = append(setTargets, call.Args[0])
			}
		case "UnsetWindowOption":
			if call.Args[1] == tablabel.DuplicateNameOption {
				unsetTargets = append(unsetTargets, call.Args[0])
			}
		}
	}
	if strings.Join(setTargets, ",") != "dev:3" {
		t.Fatalf("set targets = %v, want dev:3 (unique basename only)", setTargets)
	}
	if strings.Join(unsetTargets, ",") != "dev:1,dev:2" {
		t.Fatalf("unset targets = %v, want dev:1,dev:2 (shared basename)", unsetTargets)
	}
}

func TestRefreshDuplicateWindowNameMarkersUnsetsAllUniqueNames(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Windows["dev"] = []tmux.Window{
		{Index: 1, Name: "pi", Dir: "/home/user/zmux"},
		{Index: 2, Name: "bash", Dir: "/home/user/pi-extensions"},
	}

	if err := refreshDuplicateWindowNameMarkersForSession(a, "dev"); err != nil {
		t.Fatalf("refreshDuplicateWindowNameMarkersForSession failed: %v", err)
	}

	var unset int
	for _, call := range mock.Calls {
		if call.Method == "UnsetWindowOption" && call.Args[1] == tablabel.DuplicateNameOption {
			unset++
		}
		if call.Method == "SetWindowOption" && call.Args[1] == tablabel.DuplicateNameOption {
			t.Fatalf("did not expect duplicate marker set: %#v", call)
		}
	}
	if unset != 2 {
		t.Fatalf("unset count = %d, want 2 (calls=%#v)", unset, mock.Calls)
	}
}

func outputCommand() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{Use: "test"}
	var out bytes.Buffer
	cmd.SetOut(&out)
	return cmd, &out
}
