package cli

import (
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func addFallbackSession(t *testing.T, app *apppkg.App, wsName, label string) string {
	t.Helper()
	if err := app.WorkspaceStore.AddSession(wsName, label); err != nil {
		t.Fatal(err)
	}
	return workspace.RawSessionName(wsName, label)
}

func TestNextAttachFallbackTargetLastActiveWins(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	server := addFallbackSession(t, app, "proj", "server")
	if err := app.WorkspaceStore.SetLastActive("proj", "server"); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: main}, {Name: server}}

	got, ok := nextAttachFallbackTarget(app, "proj", map[string]bool{main: true})
	if !ok || got != server {
		t.Fatalf("nextAttachFallbackTarget = %q, %v; want %q, true", got, ok, server)
	}
}

func TestNextAttachFallbackTargetStaleLastActiveFallsBack(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	server := addFallbackSession(t, app, "proj", "server")
	if err := app.WorkspaceStore.SetLastActive("proj", "server"); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: main}}

	got, ok := nextAttachFallbackTarget(app, "proj", map[string]bool{server: true})
	if !ok || got != main {
		t.Fatalf("nextAttachFallbackTarget = %q, %v; want %q, true", got, ok, main)
	}
}

func TestNextAttachFallbackTargetNoLiveSession(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	mock.Sessions = nil

	if got, ok := nextAttachFallbackTarget(app, "proj", map[string]bool{main: true}); ok || got != "" {
		t.Fatalf("nextAttachFallbackTarget = %q, %v; want empty, false", got, ok)
	}
}

func TestNextAttachFallbackTargetRepeatBails(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	mock.Sessions = []tmux.Session{{Name: main}}

	if got, ok := nextAttachFallbackTarget(app, "proj", map[string]bool{main: true}); ok || got != "" {
		t.Fatalf("nextAttachFallbackTarget = %q, %v; want empty, false", got, ok)
	}
}

func TestAttachOwnedSessionLoopReattachesWorkspaceFallback(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	main := addFallbackSession(t, app, "proj", "main")
	server := addFallbackSession(t, app, "proj", "server")
	if err := app.WorkspaceStore.SetLastActive("proj", "server"); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: server}}

	fallbackCalled := false
	err := attachOwnedSessionLoop(app, main, session.Attach, func(*apppkg.App) error {
		fallbackCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("attachOwnedSessionLoop: %v", err)
	}
	if fallbackCalled {
		t.Fatal("fallback dashboard should not run when a sibling session is live")
	}
	if !fallbackMockHasCall(mock.Calls, "AttachSession", main) {
		t.Fatalf("expected first attach to vanished target %q, calls = %v", main, mock.Calls)
	}
	if !fallbackMockHasCall(mock.Calls, "AttachSession", server) {
		t.Fatalf("expected fallback attach to %q, calls = %v", server, mock.Calls)
	}
}

func TestAttachOwnedSessionLoopNormalDetachStops(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	main := addFallbackSession(t, app, "proj", "main")
	mock.Sessions = []tmux.Session{{Name: main}}

	fallbackCalled := false
	err := attachOwnedSessionLoop(app, main, session.Attach, func(*apppkg.App) error {
		fallbackCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("attachOwnedSessionLoop: %v", err)
	}
	if fallbackCalled {
		t.Fatal("fallback dashboard should not run after a normal detach")
	}
	if got := fallbackMockCallCount(mock.Calls, "AttachSession", main); got != 1 {
		t.Fatalf("AttachSession(%s) count = %d, want 1; calls = %v", main, got, mock.Calls)
	}
}

func TestAttachOwnedSessionLoopFallsBackWhenNoWorkspaceTarget(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	main := addFallbackSession(t, app, "proj", "main")
	mock.Sessions = nil

	fallbackCalled := false
	err := attachOwnedSessionLoop(app, main, session.Attach, func(*apppkg.App) error {
		fallbackCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("attachOwnedSessionLoop: %v", err)
	}
	if !fallbackCalled {
		t.Fatal("fallback dashboard should run when no live workspace session remains")
	}
}

func TestAttachOwnedSessionLoopForcesDetachOnDestroyOn(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "dev"}}

	err := attachOwnedSessionLoop(app, "dev", session.Attach, func(*apppkg.App) error {
		t.Fatal("fallback dashboard should not run when the session attaches")
		return nil
	})
	if err != nil {
		t.Fatalf("attachOwnedSessionLoop: %v", err)
	}
	if !fallbackMockHasCall(mock.Calls, "SetSessionOption", "dev", "detach-on-destroy", "on") {
		t.Fatalf("must force detach-on-destroy=on per-session before attach, calls = %v", mock.Calls)
	}
	if !fallbackMockHasCall(mock.Calls, "AttachSession", "dev") {
		t.Fatalf("should still attach after forcing the option, calls = %v", mock.Calls)
	}
}

func fallbackMockHasCall(calls []tmux.MockCall, method string, args ...string) bool {
	return fallbackMockCallCount(calls, method, args...) > 0
}

func fallbackMockCallCount(calls []tmux.MockCall, method string, args ...string) int {
	count := 0
	for _, c := range calls {
		if c.Method != method || len(c.Args) < len(args) {
			continue
		}
		matches := true
		for i := range args {
			if c.Args[i] != args[i] {
				matches = false
				break
			}
		}
		if matches {
			count++
		}
	}
	return count
}
