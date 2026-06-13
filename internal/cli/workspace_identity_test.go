package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func TestRepairManagedSessionNameRestoresGeneratedRawName(t *testing.T) {
	app, mock := newTestApp(t)
	expected := workspace.RawSessionName("proj", "main")
	mock.Sessions = []tmux.Session{{Name: "renamed-by-hand"}}

	got := repairManagedSessionName(app, session.SessionInfo{
		Name:      "renamed-by-hand",
		Managed:   true,
		Workspace: "proj",
		Label:     "main",
	})

	if got != expected {
		t.Fatalf("repairManagedSessionName = %q, want %q", got, expected)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "RenameSession", "renamed-by-hand", expected) {
		t.Fatalf("expected RenameSession to generated raw name, calls = %v", mock.Calls)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "SetSessionOption", expected, workspace.OptionSessionLabel, "main") {
		t.Fatalf("expected metadata restamp, calls = %v", mock.Calls)
	}
}

func TestRepairLegacySessionNameRenamesAndStampsTrackedSession(t *testing.T) {
	app, mock := newTestApp(t)
	rec, err := workspace.NewSessionRecord("hello", "main")
	if err != nil {
		t.Fatal(err)
	}
	rec.LegacyTmuxName = "main-hello"
	if err := app.WorkspaceStore.AddSessionRecord("hello", rec); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: "main-hello"}}

	got := repairManagedSessionName(app, session.SessionInfo{Name: "main-hello"})
	if got != rec.TmuxName {
		t.Fatalf("repairManagedSessionName = %q, want %q", got, rec.TmuxName)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "RenameSession", "main-hello", rec.TmuxName) {
		t.Fatalf("expected legacy RenameSession call, calls = %v", mock.Calls)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "SetSessionOption", rec.TmuxName, workspace.OptionWorkspace, "hello") {
		t.Fatalf("expected metadata stamp, calls = %v", mock.Calls)
	}
	stored, ok := app.WorkspaceStore.SessionRecord("hello", "main")
	if !ok {
		t.Fatal("expected repaired session to remain tracked")
	}
	if stored.LegacyTmuxName != "" {
		t.Fatalf("LegacyTmuxName = %q; want cleared after repair", stored.LegacyTmuxName)
	}
}

func TestRepairLegacySessionNameStampsGeneratedRawWithoutMetadata(t *testing.T) {
	app, mock := newTestApp(t)
	rec, err := workspace.NewSessionRecord("hello", "main")
	if err != nil {
		t.Fatal(err)
	}
	rec.LegacyTmuxName = "main-hello"
	if err := app.WorkspaceStore.AddSessionRecord("hello", rec); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: rec.TmuxName}}

	got := repairManagedSessionName(app, session.SessionInfo{Name: rec.TmuxName})
	if got != rec.TmuxName {
		t.Fatalf("repairManagedSessionName = %q, want %q", got, rec.TmuxName)
	}
	if workspaceIdentityMockHasCall(mock.Calls, "RenameSession", rec.TmuxName, rec.TmuxName) {
		t.Fatalf("did not expect self-rename, calls = %v", mock.Calls)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "SetSessionOption", rec.TmuxName, workspace.OptionWorkspace, "hello") {
		t.Fatalf("expected metadata stamp, calls = %v", mock.Calls)
	}
	stored, ok := app.WorkspaceStore.SessionRecord("hello", "main")
	if !ok {
		t.Fatal("expected session to remain tracked")
	}
	if stored.LegacyTmuxName != "" {
		t.Fatalf("LegacyTmuxName = %q; want cleared after stamp", stored.LegacyTmuxName)
	}
}

func TestLiveWorkspaceSessionTargetRepairsLegacyNameBeforePrefixMatch(t *testing.T) {
	app, mock := newTestApp(t)
	rec, err := workspace.NewSessionRecord("skills", "skills")
	if err != nil {
		t.Fatal(err)
	}
	rec.LegacyTmuxName = "skills"
	if err := app.WorkspaceStore.AddSessionRecord("skills", rec); err != nil {
		t.Fatal(err)
	}
	peer, err := workspace.NewSessionRecord("skills", "skills-peer")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSessionRecord("skills", peer); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: "skills"}, {Name: peer.TmuxName}}

	got := liveWorkspaceSessionTarget(app, "skills", rec)
	if got != rec.TmuxName {
		t.Fatalf("liveWorkspaceSessionTarget = %q, want %q", got, rec.TmuxName)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "RenameSession", "skills", rec.TmuxName) {
		t.Fatalf("expected legacy rename before targeting peer prefix, calls = %v", mock.Calls)
	}
	if !workspaceIdentityMockHasCall(mock.Calls, "SetSessionOption", rec.TmuxName, workspace.OptionSessionLabel, "skills") {
		t.Fatalf("expected repaired session metadata stamp, calls = %v", mock.Calls)
	}
	stored, ok := app.WorkspaceStore.SessionRecord("skills", "skills")
	if !ok {
		t.Fatal("expected skills session to remain tracked")
	}
	if stored.LegacyTmuxName != "" {
		t.Fatalf("LegacyTmuxName = %q; want cleared after repair", stored.LegacyTmuxName)
	}
}

func TestResolveSessionTargetPrefersCurrentWorkspaceLabelOverRawName(t *testing.T) {
	app, mock := newTestApp(t)
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatal(err)
	}
	expected := workspace.RawSessionName("proj", "main")
	mock.DisplayMessageResult = expected
	mock.Sessions = []tmux.Session{{Name: expected}, {Name: "main"}}

	got, err := resolveSessionTarget(app, "main")
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("resolveSessionTarget(main) = %q, want %q", got, expected)
	}
}

func workspaceIdentityMockHasCall(calls []tmux.MockCall, method string, args ...string) bool {
	for _, c := range calls {
		if c.Method != method || len(c.Args) != len(args) {
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
