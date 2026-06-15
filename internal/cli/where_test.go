package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func TestBuildWhereContext(t *testing.T) {
	managed := workspace.WorkspaceSession{Label: "main", TmuxName: "zws_myapp__main"}

	cases := []struct {
		name     string
		pane     tmux.Pane
		wsName   string
		rec      workspace.WorkspaceSession
		wsFound  bool
		tabName  string
		wantWS   string
		wantSess string
		wantRaw  string
		wantTab  string
	}{
		{
			name:     "managed session uses workspace + local label + tab name",
			pane:     tmux.Pane{ID: "%0", Session: "zws_myapp__main", Dir: "/repo", WindowIndex: 1},
			wsName:   "myapp",
			rec:      managed,
			wsFound:  true,
			tabName:  "claude",
			wantWS:   "myapp",
			wantSess: "main",
			wantRaw:  "zws_myapp__main",
			wantTab:  "claude",
		},
		{
			name:     "unmanaged session: no workspace, label falls back to root name",
			pane:     tmux.Pane{ID: "%3", Session: "scratch", Dir: "/tmp", Title: "vim"},
			wsFound:  false,
			tabName:  "",
			wantWS:   "", // rendered as — ; empty in JSON signals 'not in a workspace'
			wantSess: "scratch",
			wantRaw:  "scratch",
			wantTab:  "vim", // unclaimed pane falls back to its title
		},
		{
			name:     "grouped clone collapses to its root in the raw name",
			pane:     tmux.Pane{ID: "%9", Session: "zws_app__main__clone_b", Dir: "/repo"},
			wsFound:  false,
			tabName:  "shell",
			wantWS:   "",
			wantSess: "zws_app__main", // RootName collapse
			wantRaw:  "zws_app__main",
			wantTab:  "shell",
		},
		{
			name:     "unmanaged pane prefers window name over an app-set pane title",
			pane:     tmux.Pane{ID: "%7", Session: "scratch", Dir: "/tmp", WindowName: "logs", Title: "tail -f app.log"},
			wsFound:  false,
			tabName:  "",
			wantWS:   "",
			wantSess: "scratch",
			wantRaw:  "scratch",
			wantTab:  "logs", // window name is the tab-like identity; title is last resort
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildWhereContext(tc.pane, tc.wsName, tc.rec, tc.wsFound, tc.tabName)
			if got.Workspace != tc.wantWS {
				t.Errorf("Workspace = %q, want %q", got.Workspace, tc.wantWS)
			}
			if got.Session != tc.wantSess {
				t.Errorf("Session = %q, want %q", got.Session, tc.wantSess)
			}
			if got.SessionTmux != tc.wantRaw {
				t.Errorf("SessionTmux = %q, want %q", got.SessionTmux, tc.wantRaw)
			}
			if got.Tab != tc.wantTab {
				t.Errorf("Tab = %q, want %q", got.Tab, tc.wantTab)
			}
			if got.PaneID != tc.pane.ID {
				t.Errorf("PaneID = %q, want %q", got.PaneID, tc.pane.ID)
			}
		})
	}
}

// whereTestApp seeds a workspace + a current pane that is a logical tab, so the
// full resolve path (pane → workspace/label → tab) is exercised end to end.
func whereTestApp(t *testing.T) (rootCmd *cobra.Command, mock *tmux.MockRunner) {
	t.Helper()
	a, m := newTestApp(t)
	if err := a.WorkspaceStore.CreateWorkspace("myapp", "/repo"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := a.WorkspaceStore.AddSession("myapp", "main"); err != nil {
		t.Fatalf("add session: %v", err)
	}
	m.Panes = map[string][]tmux.Pane{
		"": {{ID: "%0", Session: "zws_myapp__main", Dir: "/repo/sub", WindowIndex: 2, Title: "bash"}},
	}
	m.LogicalRows = []tmux.LogicalPaneRow{
		{PaneID: "%0", Session: "zws_myapp__main", WindowID: "@1", TabID: "ztab_1", Label: "claude"},
	}
	t.Setenv("TMUX_PANE", "%0")
	return NewRootCmd(a, testVersion), m
}

func TestWhereReportsCurrentContext(t *testing.T) {
	rootCmd, _ := whereTestApp(t)
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"where"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("where failed: %v", err)
		}
	})
	for _, want := range []string{"myapp", "main", "zws_myapp__main", "claude", "%0", "/repo/sub"} {
		if !strings.Contains(out, want) {
			t.Errorf("where output missing %q:\n%s", want, out)
		}
	}
}

func TestWhereJSON(t *testing.T) {
	rootCmd, _ := whereTestApp(t)
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"where", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("where --json failed: %v", err)
		}
	})
	for _, want := range []string{`"workspace": "myapp"`, `"session": "main"`, `"session_tmux": "zws_myapp__main"`, `"tab": "claude"`, `"pane": "%0"`} {
		if !strings.Contains(out, want) {
			t.Errorf("where --json missing %q:\n%s", want, out)
		}
	}
}

func TestWhoamiAlias(t *testing.T) {
	rootCmd, _ := whereTestApp(t)
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"whoami"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("whoami failed: %v", err)
		}
	})
	if !strings.Contains(out, "myapp") || !strings.Contains(out, "claude") {
		t.Errorf("whoami alias should match where output, got:\n%s", out)
	}
}

func TestWhereRequiresTmux(t *testing.T) {
	a, _ := newTestApp(t)
	t.Setenv("TMUX_PANE", "")
	rootCmd := NewRootCmd(a, testVersion)
	rootCmd.SetArgs([]string{"where"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires tmux") {
		t.Fatalf("expected requires-tmux error, got %v", err)
	}
}
