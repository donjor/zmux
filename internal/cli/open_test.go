package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func TestOpenPinViewCreatesPersistentGroupedViewport(t *testing.T) {
	app, mock := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)

	if err := app.WorkspaceStore.CreateWorkspace("dev", "/repo"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	rec, err := workspace.NewSessionRecord("dev", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSessionRecord("dev", rec); err != nil {
		t.Fatalf("add session: %v", err)
	}

	mock.InsideTmux = true
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": "other"})
	mock.Sessions = []tmux.Session{{Name: rec.TmuxName, Attached: true}}

	rootCmd.SetArgs([]string{"open", "dev", "main", "--pin-view"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("open --pin-view: %v", err)
	}

	clone := rec.TmuxName + "__clone_b"
	if !mockHasCallCLI(mock, "NewGroupedSession", rec.TmuxName, clone) {
		t.Fatalf("expected grouped viewport creation, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "SetSessionOption", clone, "@zmux_pinned_view", "1") {
		t.Fatalf("expected pinned marker, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "SetSessionOption", clone, "@zmux_view_root", rec.TmuxName) {
		t.Fatalf("expected view root marker, calls=%+v", mock.Calls)
	}
	if !mockHasCallCLI(mock, "SwitchClient", clone) {
		t.Fatalf("expected switch to pinned viewport, calls=%+v", mock.Calls)
	}
}

func TestOpenRejectsHijackWithPinView(t *testing.T) {
	app, _ := newTestApp(t)
	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"open", "dev", "--hijack", "--pin-view"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected conflict error")
	}
}
