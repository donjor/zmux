package session

import (
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

func TestCreateCallsNewSession(t *testing.T) {
	m := tmux.NewMockRunner()

	err := Create(m, "myproject", "/home/user/myproject")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if len(m.Calls) < 2 {
		t.Fatalf("expected at least 2 calls (HasSession + NewSession), got %d", len(m.Calls))
	}

	// First call should be HasSession.
	if m.Calls[0].Method != "HasSession" {
		t.Errorf("expected first call to be HasSession, got %q", m.Calls[0].Method)
	}

	// Second call should be NewSession.
	if m.Calls[1].Method != "NewSession" {
		t.Errorf("expected second call to be NewSession, got %q", m.Calls[1].Method)
	}
	if m.Calls[1].Args[0] != "myproject" {
		t.Errorf("expected session name 'myproject', got %q", m.Calls[1].Args[0])
	}
	if m.Calls[1].Args[1] != "/home/user/myproject" {
		t.Errorf("expected dir '/home/user/myproject', got %q", m.Calls[1].Args[1])
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{{Name: "existing"}}

	err := Create(m, "existing", "/tmp")
	if err == nil {
		t.Fatal("expected error when session already exists")
	}
}

func TestAttachInsideTmux(t *testing.T) {
	m := tmux.NewMockRunner()
	m.InsideTmux = true

	err := Attach(m, "target")
	if err != nil {
		t.Fatalf("Attach() error: %v", err)
	}

	// Should use SwitchClient when inside tmux.
	found := false
	for _, c := range m.Calls {
		if c.Method == "SwitchClient" && len(c.Args) > 0 && c.Args[0] == "target" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SwitchClient call when inside tmux")
	}

	// Should NOT call AttachSession.
	for _, c := range m.Calls {
		if c.Method == "AttachSession" {
			t.Error("should not call AttachSession when inside tmux")
		}
	}
}

func TestAttachOutsideTmux(t *testing.T) {
	m := tmux.NewMockRunner()
	m.InsideTmux = false

	err := Attach(m, "target")
	if err != nil {
		t.Fatalf("Attach() error: %v", err)
	}

	// Should use AttachSession when outside tmux.
	found := false
	for _, c := range m.Calls {
		if c.Method == "AttachSession" && len(c.Args) > 0 && c.Args[0] == "target" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AttachSession call when outside tmux")
	}

	// Should NOT call SwitchClient.
	for _, c := range m.Calls {
		if c.Method == "SwitchClient" {
			t.Error("should not call SwitchClient when outside tmux")
		}
	}
}

func TestCleanupTmpKillsOnlyUnattachedTmp(t *testing.T) {
	now := time.Now()
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Dir: "/home"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Dir: "/tmp"},
		{Name: "tmp-2", Windows: 1, Attached: true, Activity: now, Dir: "/tmp"},
		{Name: "tmp-3", Windows: 1, Attached: false, Activity: now, Dir: "/tmp"},
		{Name: "work", Windows: 2, Attached: false, Activity: now, Dir: "/home"},
	}

	killed, err := CleanupTmp(m)
	if err != nil {
		t.Fatalf("CleanupTmp() error: %v", err)
	}

	// Should kill tmp-1 and tmp-3 (unattached tmp sessions).
	if len(killed) != 2 {
		t.Fatalf("expected 2 killed sessions, got %d: %v", len(killed), killed)
	}

	// Check the killed names.
	killedSet := make(map[string]bool)
	for _, name := range killed {
		killedSet[name] = true
	}
	if !killedSet["tmp-1"] {
		t.Error("expected tmp-1 to be killed")
	}
	if !killedSet["tmp-3"] {
		t.Error("expected tmp-3 to be killed")
	}

	// Verify KillSession was called for the right sessions.
	killCalls := 0
	for _, c := range m.Calls {
		if c.Method == "KillSession" {
			killCalls++
			name := c.Args[0]
			if name != "tmp-1" && name != "tmp-3" {
				t.Errorf("unexpected KillSession call for %q", name)
			}
		}
	}
	if killCalls != 2 {
		t.Errorf("expected 2 KillSession calls, got %d", killCalls)
	}
}

func TestCleanupTmpNoTmpSessions(t *testing.T) {
	m := tmux.NewMockRunner()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3},
		{Name: "work", Windows: 2},
	}

	killed, err := CleanupTmp(m)
	if err != nil {
		t.Fatalf("CleanupTmp() error: %v", err)
	}

	if len(killed) != 0 {
		t.Errorf("expected 0 killed sessions, got %d", len(killed))
	}
}

func TestKill(t *testing.T) {
	m := tmux.NewMockRunner()
	err := Kill(m, "doomed")
	if err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	if len(m.Calls) == 0 || m.Calls[0].Method != "KillSession" {
		t.Fatal("expected KillSession call")
	}
	if m.Calls[0].Args[0] != "doomed" {
		t.Errorf("expected session name 'doomed', got %q", m.Calls[0].Args[0])
	}
}

func TestRename(t *testing.T) {
	m := tmux.NewMockRunner()
	err := Rename(m, "old-name", "new-name")
	if err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	found := false
	for _, c := range m.Calls {
		if c.Method == "RenameSession" && c.Args[0] == "old-name" && c.Args[1] == "new-name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RenameSession call with correct args")
	}
}
