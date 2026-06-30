package workspace

import (
	"errors"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestForkSessionCopiesWindowNamesAndStampsTabs(t *testing.T) {
	store, _ := newTestStore()
	if err := store.CreateWorkspace("dev", "/repo"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "%1"
	mock.NewWindowPaneID = "%2"
	mock.Windows["source"] = []tmux.Window{
		{Index: 3, Name: "editor", Active: true},
		{Index: 4, Name: "server", Label: "dev-server"},
	}
	dest := RawSessionName("dev", "feature")
	mock.Windows[dest] = []tmux.Window{{Index: 7, Name: "zsh", Active: true}}

	rec, err := ForkSession(mock, store, "dev", "source", "feature", "/repo/feature")
	if err != nil {
		t.Fatalf("ForkSession: %v", err)
	}
	if rec.TmuxName != dest {
		t.Fatalf("dest = %q; want %q", rec.TmuxName, dest)
	}

	if !mockHasCallWorkspace(mock, "RenameWindow", dest, "7", "editor") {
		t.Fatalf("expected first window rename by actual index, calls=%+v", mock.Calls)
	}
	if !mockHasCallWorkspace(mock, "NewWindow", dest, "dev-server", "/repo/feature", "detached=true") {
		t.Fatalf("expected second detached window with stable label name, calls=%+v", mock.Calls)
	}
	if !mockHasCallWorkspace(mock, "ApplyOptions", "-p", "%1", "@zmux_tab_id") {
		t.Fatalf("expected first pane tab stamp, calls=%+v", mock.Calls)
	}
	if !mockHasCallWorkspace(mock, "ApplyOptions", "-p", "%2", "@zmux_tab_id") {
		t.Fatalf("expected second pane tab stamp, calls=%+v", mock.Calls)
	}
	if !mockHasCallWorkspace(mock, "ApplyOptions", "-p", "%2", "@zmux_label", "dev-server") {
		t.Fatalf("expected stable label stamp on copied labeled tab, calls=%+v", mock.Calls)
	}
}

func TestForkSessionRejectsEmptySource(t *testing.T) {
	store, _ := newTestStore()
	mock := tmux.NewMockRunner()

	if _, err := ForkSession(mock, store, "dev", "", "feature", "/repo"); err == nil {
		t.Fatal("expected source-session error")
	}
}

func TestForkSessionRollsBackOnStampFailure(t *testing.T) {
	store, _ := newTestStore()
	if err := store.CreateWorkspace("dev", "/repo"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "%1"
	mock.Windows["source"] = []tmux.Window{{Index: 1, Name: "editor", Active: true}}
	dest := RawSessionName("dev", "feature")
	mock.Windows[dest] = []tmux.Window{{Index: 1, Name: "zsh", Active: true}}
	runner := &failSecondApplyRunner{MockRunner: mock}

	if _, err := ForkSession(runner, store, "dev", "source", "feature", "/repo/feature"); err == nil {
		t.Fatal("expected stamp error")
	}
	if !mockHasCallWorkspace(mock, "KillSession", dest) {
		t.Fatalf("expected rollback KillSession(%s), calls=%+v", dest, mock.Calls)
	}
	if labels := store.SessionLabelsIn("dev"); len(labels) != 0 {
		t.Fatalf("expected rollback to remove store record, got %v", labels)
	}
}

type failSecondApplyRunner struct {
	*tmux.MockRunner
	applies int
}

func (r *failSecondApplyRunner) ApplyOptions(writes []tmux.OptionWrite) error {
	r.applies++
	if err := r.MockRunner.ApplyOptions(writes); err != nil {
		return err
	}
	if r.applies == 2 {
		return errors.New("stamp failed")
	}
	return nil
}

func mockHasCallWorkspace(m *tmux.MockRunner, method string, args ...string) bool {
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
