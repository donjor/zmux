package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// failGroupedRunner overrides NewGroupedSession to fail while delegating every
// other call to the embedded MockRunner — MockRunner.Err is a single field that
// would poison ListSessions too, so a wrapper is the cleanest way to fail just
// the clone-create step.
type failGroupedRunner struct {
	*tmux.MockRunner
}

func (f *failGroupedRunner) NewGroupedSession(target, name string) error {
	return errors.New("grouped session create failed")
}

func methodCalls(m *tmux.MockRunner, method string) []tmux.MockCall {
	var out []tmux.MockCall
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func callIndex(m *tmux.MockRunner, method string, args ...string) int {
	for i, c := range m.Calls {
		if c.Method != method || len(c.Args) < len(args) {
			continue
		}
		ok := true
		for j, a := range args {
			if c.Args[j] != a {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

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

func TestCreateStampsFirstWindow(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageFunc = func(target, format string) (string, error) {
		return "%7\n", nil // first window's pane id (trailing newline like real tmux)
	}

	if err := Create(m, "proj", "/tmp"); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// The first window's pane must be claimed with a tab id so it's a managed
	// logical tab — otherwise `tab pane` joining into it fails. Stamp writes the
	// id via ApplyOptions(scope=pane, target=%7, key=@zmux_tab_id, value=ztab_…).
	var stamped bool
	for _, c := range m.Calls {
		if c.Method == "ApplyOptions" && len(c.Args) >= 4 &&
			c.Args[1] == "%7" && c.Args[2] == tabs.OptTabID &&
			strings.HasPrefix(c.Args[3], "ztab_") {
			stamped = true
		}
	}
	if !stamped {
		t.Fatalf("first window pane not stamped with %s; calls: %v", tabs.OptTabID, m.Calls)
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

func TestSwitchViewPlainWhenUnattached(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "target", Attached: false}}

	actual, err := SwitchView(m, "target")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "target" {
		t.Errorf("expected actual %q, got %q", "target", actual)
	}
	if len(methodCalls(m, "NewGroupedSession")) != 0 {
		t.Error("should not clone when target is unattached")
	}
	if callIndex(m, "SwitchClient", "target") < 0 {
		t.Error("expected plain SwitchClient(target)")
	}
}

func TestSwitchViewClonesWhenAttached(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "target", Attached: true}}

	actual, err := SwitchView(m, "target")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "target-b" {
		t.Errorf("expected actual %q, got %q", "target-b", actual)
	}

	// Order is load-bearing: create clone, switch to it, THEN arm teardown.
	iNew := callIndex(m, "NewGroupedSession", "target", "target-b")
	iSwitch := callIndex(m, "SwitchClient", "target-b")
	iArm := callIndex(m, "SetSessionOption", "target-b", "destroy-unattached", "on")
	if iNew < 0 || iSwitch < 0 || iArm < 0 {
		t.Fatalf("missing call: NewGroupedSession=%d SwitchClient=%d SetSessionOption=%d", iNew, iSwitch, iArm)
	}
	if iNew >= iSwitch || iSwitch >= iArm {
		t.Errorf("expected order NewGroupedSession < SwitchClient < SetSessionOption, got %d < %d < %d", iNew, iSwitch, iArm)
	}
}

func TestSwitchViewManagedCloneNaming(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "zws_w__s", Attached: true}}

	actual, err := SwitchView(m, "zws_w__s")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "zws_w__s__clone_b" {
		t.Errorf("expected managed clone name, got %q", actual)
	}
	if callIndex(m, "NewGroupedSession", "zws_w__s", "zws_w__s__clone_b") < 0 {
		t.Error("expected managed __clone_b grouped session")
	}
}

func TestSwitchViewNoOpSameRoot(t *testing.T) {
	for _, prev := range []string{"target", "target-b"} {
		m := tmux.NewMockRunner()
		m.DisplayMessageResult = prev
		m.Sessions = []tmux.Session{
			{Name: "target", Attached: true},
			{Name: "target-b", Group: "target", Clone: true, Attached: true},
		}

		actual, err := SwitchView(m, "target")
		if err != nil {
			t.Fatalf("SwitchView() error: %v", err)
		}
		if actual != prev {
			t.Errorf("prev %q: expected no-op return %q, got %q", prev, prev, actual)
		}
		if len(methodCalls(m, "SwitchClient")) != 0 {
			t.Errorf("prev %q: should not switch when already on the target root", prev)
		}
		if len(methodCalls(m, "NewGroupedSession")) != 0 {
			t.Errorf("prev %q: should not clone on a no-op", prev)
		}
	}
}

func TestSwitchViewGCsLeftClone(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "foo-b" // leaving a grouped clone
	m.Sessions = []tmux.Session{
		{Name: "target", Attached: false},
		{Name: "foo", Attached: false},
		{Name: "foo-b", Group: "foo", Clone: true, Attached: false}, // zmux clone, clientless after the switch
	}

	if _, err := SwitchView(m, "target"); err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if callIndex(m, "KillSession", "foo-b") < 0 {
		t.Error("expected the left clone foo-b to be garbage-collected")
	}
}

func TestSwitchViewDoesNotKillManuallyGroupedSession(t *testing.T) {
	// A user-created group member (tmux new-session -t foo -s foo-b) reports a
	// non-empty session_group just like a zmux clone, but carries no @zmux_clone
	// marker. It must never be garbage-collected.
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "foo-b"
	m.Sessions = []tmux.Session{
		{Name: "target", Attached: false},
		{Name: "foo", Attached: false},
		{Name: "foo-b", Group: "foo", Clone: false, Attached: false}, // grouped by hand, not zmux
	}

	if _, err := SwitchView(m, "target"); err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if callIndex(m, "KillSession", "foo-b") >= 0 {
		t.Error("must not kill a session grouped by the user (no @zmux_clone marker)")
	}
}

func TestSwitchViewDoesNotGCStillAttachedClone(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "foo-b"
	m.Sessions = []tmux.Session{
		{Name: "target", Attached: false},
		{Name: "foo-b", Group: "foo", Clone: true, Attached: true}, // another client still on it
	}

	if _, err := SwitchView(m, "target"); err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if callIndex(m, "KillSession", "foo-b") >= 0 {
		t.Error("must not kill a clone that still has an attached client")
	}
}

func TestSwitchViewDoesNotGCPinnedClone(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "foo-b"
	m.Sessions = []tmux.Session{
		{Name: "target", Attached: false},
		{Name: "foo-b", Group: "foo", Clone: true, PinnedView: true, Attached: false},
	}

	if _, err := SwitchView(m, "target"); err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if callIndex(m, "KillSession", "foo-b") >= 0 {
		t.Error("must not kill a pinned clone")
	}
}

func TestAttachPinnedViewCreatesPersistentGroupedViewport(t *testing.T) {
	m := tmux.NewMockRunner()
	m.InsideTmux = true
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "target", Attached: true, Managed: true, Workspace: "dev", SessionLabel: "main", SessionID: "s_1"}}

	actual, err := AttachPinnedView(m, "target")
	if err != nil {
		t.Fatalf("AttachPinnedView() error: %v", err)
	}
	if actual != "target-b" {
		t.Fatalf("AttachPinnedView() = %q; want target-b", actual)
	}
	if callIndex(m, "NewGroupedSession", "target", "target-b") < 0 {
		t.Error("expected grouped viewport creation")
	}
	if callIndex(m, "SetSessionOption", "target-b", optionPinnedView, "1") < 0 {
		t.Error("expected pinned-view marker")
	}
	if callIndex(m, "SetSessionOption", "target-b", optionViewRoot, "target") < 0 {
		t.Error("expected view-root marker")
	}
	if callIndex(m, "SetSessionOption", "target-b", optionWorkspace, "dev") < 0 || callIndex(m, "SetSessionOption", "target-b", optionSessionLabel, "main") < 0 || callIndex(m, "SetSessionOption", "target-b", optionSessionID, "s_1") < 0 {
		t.Error("expected pinned view to copy root workspace metadata")
	}
	if callIndex(m, "SetSessionOption", "target-b", "destroy-unattached", "on") >= 0 {
		t.Error("pinned views must not be destroy-unattached")
	}
	if callIndex(m, "SwitchClient", "target-b") < 0 {
		t.Error("expected switch to pinned viewport")
	}
}

func TestSwitchViewSwitchesToStandaloneCloneNamedTarget(t *testing.T) {
	// Target is a standalone session named "foo-b" (no @zmux_clone marker).
	// SwitchView must switch to it literally, not strip to "foo".
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "dev"
	m.Sessions = []tmux.Session{
		{Name: "dev", Attached: true},
		{Name: "foo", Attached: false},
		{Name: "foo-b", Group: "", Clone: false, Attached: false}, // standalone target
	}

	actual, err := SwitchView(m, "foo-b")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "foo-b" {
		t.Errorf("expected literal switch to %q, got %q", "foo-b", actual)
	}
	if callIndex(m, "SwitchClient", "foo-b") < 0 {
		t.Error("must switch to the standalone foo-b, not its name-root foo")
	}
	if callIndex(m, "SwitchClient", "foo") >= 0 {
		t.Error("must not redirect a standalone foo-b target to foo")
	}
}

func TestSwitchViewSwitchesFromStandaloneCloneName(t *testing.T) {
	// On a standalone session genuinely named "foo-b" (no @zmux_clone marker),
	// switching to the distinct real session "foo" must actually switch — the
	// no-op guard must not mistake the name for a clone-of-foo view.
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "foo-b"
	m.Sessions = []tmux.Session{
		{Name: "foo", Attached: false},
		{Name: "foo-b", Group: "", Clone: false, Attached: false}, // standalone, not a clone
	}

	actual, err := SwitchView(m, "foo")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "foo" {
		t.Errorf("expected a real switch to %q, got no-op %q", "foo", actual)
	}
	if callIndex(m, "SwitchClient", "foo") < 0 {
		t.Error("must switch to foo, not no-op on the standalone foo-b")
	}
	if callIndex(m, "KillSession", "foo-b") >= 0 {
		t.Error("must not kill the standalone foo-b")
	}
}

func TestSwitchViewGroupedSessionFailureFallsBack(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "target", Attached: true}}
	r := &failGroupedRunner{MockRunner: m}

	actual, err := SwitchView(r, "target")
	if err != nil {
		t.Fatalf("SwitchView() error: %v", err)
	}
	if actual != "target" {
		t.Errorf("expected fallback to root %q, got %q", "target", actual)
	}
	if callIndex(m, "SwitchClient", "target") < 0 {
		t.Error("expected fallback SwitchClient(target) after clone-create failure")
	}
	if len(methodCalls(m, "SetSessionOption")) != 0 {
		t.Error("should not arm teardown when no clone was created")
	}
}

func TestSwitchViewDisplayMessageErrorProceeds(t *testing.T) {
	m := tmux.NewMockRunner()
	m.DisplayMessageFunc = func(target, format string) (string, error) {
		return "", errors.New("no current client")
	}
	m.Sessions = []tmux.Session{{Name: "target", Attached: false}}

	actual, err := SwitchView(m, "target")
	if err != nil {
		t.Fatalf("SwitchView() should not fail on DisplayMessage error: %v", err)
	}
	if actual != "target" {
		t.Errorf("expected %q, got %q", "target", actual)
	}
	if callIndex(m, "SwitchClient", "target") < 0 {
		t.Error("expected SwitchClient(target) despite DisplayMessage error")
	}
}

func TestAttachInsideTmuxClonesWhenAttached(t *testing.T) {
	m := tmux.NewMockRunner()
	m.InsideTmux = true
	m.DisplayMessageResult = "other"
	m.Sessions = []tmux.Session{{Name: "target", Attached: true}}

	if err := Attach(m, "target"); err != nil {
		t.Fatalf("Attach() error: %v", err)
	}
	if callIndex(m, "NewGroupedSession", "target", "target-b") < 0 {
		t.Error("expected Attach inside tmux to clone an already-attached target")
	}
	if callIndex(m, "SwitchClient", "target-b") < 0 {
		t.Error("expected switch to the clone")
	}
	for _, c := range m.Calls {
		if c.Method == "AttachSession" {
			t.Error("should not AttachSession when inside tmux")
		}
	}
}
