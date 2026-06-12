package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func findMockCall(calls []tmux.MockCall, method string) (tmux.MockCall, bool) {
	for _, call := range calls {
		if call.Method == method {
			return call, true
		}
	}
	return tmux.MockCall{}, false
}

func TestNewWorkspaceDefaultsSessionToMain(t *testing.T) {
	app, mock := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)
	mock.InsideTmux = false

	rootCmd.SetArgs([]string{"new", "myapp"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("new workspace: %v", err)
	}

	call, ok := findMockCall(mock.Calls, "NewSession")
	if !ok {
		t.Fatal("expected NewSession call")
	}
	if got, want := call.Args[0], workspace.RawSessionName("myapp", "main"); got != want {
		t.Fatalf("NewSession name = %q, want %q", got, want)
	}

	ws, err := app.WorkspaceStore.GetWorkspace("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Sessions) != 1 || ws.Sessions[0].Label != "main" || ws.Sessions[0].TmuxName != workspace.RawSessionName("myapp", "main") {
		t.Fatalf("workspace sessions = %v, want [main]", ws.Sessions)
	}
	if ws.LastActiveSession != "main" {
		t.Fatalf("last active = %q, want main", ws.LastActiveSession)
	}
}

func TestNewWorkspaceDefaultSessionAvoidsGlobalMainCollision(t *testing.T) {
	app, mock := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "main"}}

	rootCmd.SetArgs([]string{"new", "hello"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("new workspace: %v", err)
	}

	call, ok := findMockCall(mock.Calls, "NewSession")
	if !ok {
		t.Fatal("expected NewSession call")
	}
	if got, want := call.Args[0], workspace.RawSessionName("hello", "main"); got != want {
		t.Fatalf("NewSession name = %q, want %q", got, want)
	}

	ws, err := app.WorkspaceStore.GetWorkspace("hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Sessions) != 1 || ws.Sessions[0].Label != "main" || ws.Sessions[0].TmuxName != workspace.RawSessionName("hello", "main") {
		t.Fatalf("workspace sessions = %v, want [main]", ws.Sessions)
	}
	if ws.LastActiveSession != "main" {
		t.Fatalf("last active = %q, want main", ws.LastActiveSession)
	}
}

func TestWorkspaceSessionNameDefaultCollision(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "main"}}

	if got, want := workspaceSessionName(app, "", "hello"), workspace.RawSessionName("hello", "main"); got != want {
		t.Fatalf("workspaceSessionName = %q, want %q", got, want)
	}
}
