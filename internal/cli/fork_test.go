package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func TestForkCommandCreatesManagedSessionFromCurrentWorkspace(t *testing.T) {
	app, mock := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)

	if err := app.WorkspaceStore.CreateWorkspace("dev", "/repo"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	sourceRec, err := workspace.NewSessionRecord("dev", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSessionRecord("dev", sourceRec); err != nil {
		t.Fatalf("add source: %v", err)
	}

	dest := workspace.RawSessionName("dev", "feature")
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		switch format {
		case "#{session_name}":
			return sourceRec.TmuxName, nil
		case "#{pane_id}":
			return "%1", nil
		default:
			return "", nil
		}
	}
	mock.Sessions = []tmux.Session{{Name: sourceRec.TmuxName, Windows: 2}}
	mock.Windows[sourceRec.TmuxName] = []tmux.Window{
		{Index: 1, Name: "editor", Active: true},
		{Index: 2, Name: "server"},
	}
	mock.Windows[dest] = []tmux.Window{{Index: 4, Name: "zsh", Active: true}}
	mock.NewWindowPaneID = "%2"

	rootCmd.SetArgs([]string{"fork", "feature", "--dir", "/repo/feature"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("fork: %v", err)
	}

	if !mockHasCallCLI(mock, "NewSession", dest, "/repo/feature") {
		t.Fatalf("expected managed destination NewSession, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "RenameWindow", dest, "4", "editor") {
		t.Fatalf("expected first window rename by actual index, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "NewWindow", dest, "server", "/repo/feature", "detached=true") {
		t.Fatalf("expected copied detached server tab, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "SwitchClient", dest) {
		t.Fatalf("expected switch to forked session, calls=%+v", mock.Calls)
	}
	if ws, _ := app.WorkspaceStore.GetWorkspace("dev"); ws.LastActiveSession != "feature" {
		t.Fatalf("last active = %q; want feature", ws.LastActiveSession)
	}
}

func TestForkCommandRequiresTmux(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"fork", "feature"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected outside-tmux error")
	}
}

func TestForkCommandRequiresTrackedWorkspaceSession(t *testing.T) {
	app, mock := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)
	mock.DisplayMessageResult = "loose"
	mock.Windows["loose"] = []tmux.Window{{Index: 1, Name: "shell"}}

	rootCmd.SetArgs([]string{"fork", "feature"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected untracked session error")
	}
}

func mockHasCallCLI(m *tmux.MockRunner, method string, args ...string) bool {
	for _, c := range m.Calls {
		if c.Method != method {
			continue
		}
		if len(args) > len(c.Args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
