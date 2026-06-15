package workspaceoutline_test

import (
	"testing"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceoutline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

func wsModel(name string, attached bool, sessions ...session.SessionInfo) workspaceview.WorkspaceViewModel {
	return workspaceview.WorkspaceViewModel{
		Workspace:    workspace.Workspace{Name: name},
		HasAttached:  attached,
		LiveSessions: sessions,
	}
}

func ids(rows []outline.Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func findRow(rows []outline.Row, id string) *outline.Row {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

// TestBuildDashboardStyle covers the full-manager policy: chevrons, a current
// marker, and an empty-workspace placeholder under an expanded workspace.
func TestBuildDashboardStyle(t *testing.T) {
	tree := outline.NewTree()
	tree.SetExpanded(outline.WorkspaceID("dev"), true)
	tree.SetExpanded(outline.WorkspaceID("empty"), true)
	// "api" stays collapsed.

	workspaces := []workspaceview.WorkspaceViewModel{
		wsModel("dev", true,
			session.SessionInfo{Name: "dev", Attached: true},
			session.SessionInfo{Name: "dev-2"},
		),
		wsModel("api", false, session.SessionInfo{Name: "api"}),
		wsModel("empty", false),
	}

	p := workspaceoutline.Policy{
		WorkspaceLabel: func(w *workspaceview.WorkspaceViewModel) string { return w.Name },
		ShowChevron:    true,
		Expanded:       func(id string, _ *workspaceview.WorkspaceViewModel) bool { return tree.IsExpanded(id) },
		Sessions:       func(w *workspaceview.WorkspaceViewModel) []session.SessionInfo { return w.LiveSessions },
		DecorateSession: func(r *outline.Row, s *session.SessionInfo) {
			if s.Name == "dev" {
				r.Current = true
			}
		},
		EmptyWorkspaceRow: func(w *workspaceview.WorkspaceViewModel) *outline.Row {
			return &outline.Row{ID: "placeholder:" + w.Name, Kind: outline.RowPlaceholder, Depth: 1, Label: "(no live sessions)"}
		},
	}

	rows := workspaceoutline.Build(workspaces, nil, tree, p)

	want := []string{
		outline.WorkspaceID("dev"),
		outline.SessionID("dev"),
		outline.SessionID("dev-2"),
		outline.WorkspaceID("api"), // collapsed: header only
		outline.WorkspaceID("empty"),
		"placeholder:empty", // expanded + no sessions
	}
	got := ids(rows)
	if len(got) != len(want) {
		t.Fatalf("row IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row[%d] = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}

	if h := findRow(rows, outline.WorkspaceID("dev")); h == nil || !h.Expanded {
		t.Error("dev header should carry Expanded=true with ShowChevron")
	}
	if s := findRow(rows, outline.SessionID("dev")); s == nil || !s.Current {
		t.Error("current session marker not applied to dev")
	}
	if s := findRow(rows, outline.SessionID("dev-2")); s == nil || s.Current {
		t.Error("dev-2 should not be marked current")
	}
}

// TestBuildPickerStyle covers the flat-picker policy: a top-action row,
// focus-based expansion, and no chevron state recorded on headers.
func TestBuildPickerStyle(t *testing.T) {
	tree := outline.NewTree()
	focused := outline.WorkspaceID("dev")

	workspaces := []workspaceview.WorkspaceViewModel{
		wsModel("dev", false, session.SessionInfo{Name: "dev"}),
		wsModel("api", false, session.SessionInfo{Name: "api"}),
	}

	p := workspaceoutline.Policy{
		TopAction:      func() string { return "+ new tmp session" },
		WorkspaceLabel: func(w *workspaceview.WorkspaceViewModel) string { return w.Name },
		Expanded:       func(id string, _ *workspaceview.WorkspaceViewModel) bool { return id == focused },
		Sessions:       func(w *workspaceview.WorkspaceViewModel) []session.SessionInfo { return w.LiveSessions },
	}

	rows := workspaceoutline.Build(workspaces, nil, tree, p)

	want := []string{
		outline.TopActionID(),
		outline.WorkspaceID("dev"),
		outline.SessionID("dev"),   // dev focused -> expanded
		outline.WorkspaceID("api"), // not focused -> collapsed
	}
	got := ids(rows)
	if len(got) != len(want) {
		t.Fatalf("row IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row[%d] = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
	if h := findRow(rows, outline.WorkspaceID("dev")); h == nil || h.Expanded {
		t.Error("picker headers must not record Expanded (ShowChevron is false)")
	}
	if rows[0].Kind != outline.RowTopAction {
		t.Errorf("first row kind = %v, want RowTopAction", rows[0].Kind)
	}
}

// TestBuildExternalRowsNilCatalog guards the empty-section base case.
func TestBuildExternalRowsNilCatalog(t *testing.T) {
	if rows := workspaceoutline.BuildExternalRows(nil, outline.NewTree(), ""); rows != nil {
		t.Errorf("nil catalog should yield nil rows, got %v", rows)
	}
}
