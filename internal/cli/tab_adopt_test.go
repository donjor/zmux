package cli

import (
	"errors"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

var errTmux = errors.New("tmux boom")

// stampedID reports whether pane got an @zmux_tab_id write.
func stampedID(m *tmux.MockRunner, paneID string) bool {
	for _, c := range m.Calls {
		if c.Method == "ApplyOptions" && c.Args[1] == paneID && c.Args[2] == tabs.OptTabID {
			return true
		}
	}
	return false
}

// labelWrite returns the pane-scoped label value written to pane, if any.
func labelWrite(m *tmux.MockRunner, paneID string) (string, bool) {
	for _, c := range m.Calls {
		if c.Method == "ApplyOptions" && c.Args[1] == paneID && c.Args[2] == tablabel.Option {
			return c.Args[3], true
		}
	}
	return "", false
}

func adoptApp(mock *tmux.MockRunner) *apppkg.App { return &apppkg.App{Runner: mock} }

func TestAdoptStampsFreshSinglePaneWindow(t *testing.T) {
	mock := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"@10": {{ID: "%1", Session: "work", Active: true}},
	}}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adoptWindow: %v", err)
	}
	if !stampedID(mock, "%1") {
		t.Error("fresh single-pane window should be stamped with @zmux_tab_id")
	}
	if v, ok := labelWrite(mock, "%1"); ok && v != "" {
		t.Errorf("fresh window should get an empty-label stamp, got label %q", v)
	}
}

// Scoping guard: adopt acts on the hook window only, so a sibling raw window
// (the bug the peer caught — session-wide scan would retro-adopt it) is never
// touched.
func TestAdoptDoesNotTouchSiblingWindow(t *testing.T) {
	mock := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"@9":  {{ID: "%0", Session: "work", Active: true}}, // old raw sibling
		"@10": {{ID: "%1", Session: "work", Active: true}}, // the just-created one
	}}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adoptWindow: %v", err)
	}
	if !stampedID(mock, "%1") {
		t.Error("hook window should be stamped")
	}
	if stampedID(mock, "%0") {
		t.Error("sibling raw window must NOT be adopted (027 floor)")
	}
}

func TestAdoptSkipsAlreadyManaged(t *testing.T) {
	mock := &tmux.MockRunner{
		Panes:       map[string][]tmux.Pane{"@10": {{ID: "%1", Session: "work", Active: true}}},
		PaneOptions: map[string]string{"%1\x00" + tabs.OptTabID: "ztab_existing"},
	}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adoptWindow: %v", err)
	}
	if stampedID(mock, "%1") {
		t.Error("already-managed pane should not be re-stamped")
	}
}

func TestAdoptSkipsMultiPaneWindow(t *testing.T) {
	mock := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
		"@10": {{ID: "%1", Session: "work", Active: true}, {ID: "%2", Session: "work"}},
	}}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adoptWindow: %v", err)
	}
	if stampedID(mock, "%1") || stampedID(mock, "%2") {
		t.Error("multi-pane raw window must not be adopted (only single-pane create claims)")
	}
}

func TestAdoptSkipsReservedSession(t *testing.T) {
	// Both the dock and any other __zmux_ reserved session: the guard is a
	// prefix match (IsReservedSession), not DockSession equality.
	for _, sess := range []string{tabs.DockSession, tabs.ReservedPrefix + "scratch"} {
		mock := &tmux.MockRunner{Panes: map[string][]tmux.Pane{
			"@10": {{ID: "%1", Session: sess, Active: true}},
		}}
		if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
			t.Fatalf("adoptWindow(%s): %v", sess, err)
		}
		if stampedID(mock, "%1") {
			t.Errorf("reserved session %q window must not be adopted", sess)
		}
	}
}

// Legacy window-scoped label migrates onto the pane (canonical) rather than
// being clobbered by an empty-label stamp — else MigrateWindowLabel would be
// permanently blocked and the label left non-canonical.
func TestAdoptMigratesLegacyWindowLabel(t *testing.T) {
	mock := &tmux.MockRunner{
		Panes:         map[string][]tmux.Pane{"@10": {{ID: "%1", Session: "work", Active: true}}},
		WindowOptions: map[string]string{"@10\x00" + tablabel.Option: "mylabel"},
	}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adoptWindow: %v", err)
	}
	if !stampedID(mock, "%1") {
		t.Error("legacy-labeled window should still be stamped with an id")
	}
	if v, ok := labelWrite(mock, "%1"); !ok || v != "mylabel" {
		t.Errorf("legacy label should migrate onto the pane, got %q (present=%v)", v, ok)
	}
}

func TestAdoptBestEffortOnError(t *testing.T) {
	mock := &tmux.MockRunner{
		Panes: map[string][]tmux.Pane{"@10": {{ID: "%1", Session: "work", Active: true}}},
		Err:   errTmux,
	}
	if err := adoptWindow(adoptApp(mock), "@10"); err != nil {
		t.Fatalf("adopt must swallow tmux errors (hook-driven), got %v", err)
	}
}

func TestAdoptEmptyWindowNoop(t *testing.T) {
	mock := &tmux.MockRunner{}
	if err := adoptWindow(adoptApp(mock), ""); err != nil {
		t.Fatalf("empty window should no-op, got %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("empty window should not touch tmux, made %d calls", len(mock.Calls))
	}
}
