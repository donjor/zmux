package outline

import "testing"

// Test helpers.

func mkRow(id string, kind RowKind, depth int, parent string, selectable bool) Row {
	return Row{
		ID:         id,
		Kind:       kind,
		Depth:      depth,
		ParentID:   parent,
		Label:      id,
		Selectable: selectable,
	}
}

// ── SetRows cursor preservation ──

func TestSetRowsPreservesCursorByID(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
		mkRow("c", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 1 // on "b"

	// Rebuild with reordered rows — cursor should still land on "b".
	tr.SetRows([]Row{
		mkRow("c", RowWorkspaceHeader, 0, "", true),
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	if got := tr.Current(); got == nil || got.ID != "b" {
		t.Errorf("cursor should restore to 'b', got %+v", got)
	}
}

func TestSetRowsFallsBackToParent(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:main", RowSession, 1, "ws:app", true),
		mkRow("session:other", RowSession, 1, "ws:app", true),
	})
	tr.Cursor = 1 // on session:main

	// Remove session:main, keep parent workspace.
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:other", RowSession, 1, "ws:app", true),
	})
	if got := tr.Current(); got == nil || got.ID != "ws:app" {
		t.Errorf("cursor should fall back to parent 'ws:app', got %+v", got)
	}
}

func TestSetRowsFallsBackToSameDepth(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:main", RowSession, 1, "ws:app", true),
	})
	tr.Cursor = 1 // depth 1

	// Remove both the row and its parent; introduce a new depth-1 row
	// under a different parent.
	tr.SetRows([]Row{
		mkRow("ws:other", RowWorkspaceHeader, 0, "", true),
		mkRow("session:dev", RowSession, 1, "ws:other", true),
	})
	if got := tr.Current(); got == nil || got.Depth != 1 {
		t.Errorf("cursor should fall back to first selectable at depth 1, got %+v", got)
	}
}

func TestSetRowsAllNonSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("d1", RowDivider, 0, "", false),
		mkRow("p1", RowPlaceholder, 0, "", false),
	})
	if tr.Cursor != 0 {
		t.Errorf("cursor should be 0 when nothing is selectable, got %d", tr.Cursor)
	}
	if tr.CurrentSelectable() != nil {
		t.Errorf("CurrentSelectable should be nil")
	}
}

// ── SetRowsAndJumpTo ──

func TestSetRowsAndJumpToHitsTarget(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0

	tr.SetRowsAndJumpTo([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b-renamed", RowWorkspaceHeader, 0, "", true),
	}, "b-renamed")

	if got := tr.Current(); got == nil || got.ID != "b-renamed" {
		t.Errorf("cursor should land on target 'b-renamed', got %+v", got)
	}
}

func TestSetRowsAndJumpToFallsThrough(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0

	// Target doesn't exist — should fall through to restore.
	tr.SetRowsAndJumpTo([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
	}, "nonexistent")

	if got := tr.Current(); got == nil || got.ID != "a" {
		t.Errorf("cursor should fall back to prev ID 'a', got %+v", got)
	}
}

// ── Expansion ──

func TestExpansionPersistsAcrossSetRows(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
	})
	tr.SetExpanded("ws:app", true)

	// Rebuild rows — expansion state should still be there.
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:a", RowSession, 1, "ws:app", true),
	})
	if !tr.IsExpanded("ws:app") {
		t.Error("expansion state should persist across SetRows")
	}
}

func TestResetExpansionClears(t *testing.T) {
	tr := NewTree()
	tr.SetExpanded("ws:app", true)
	tr.SetExpanded("ws:other", true)
	tr.ResetExpansion()
	if tr.IsExpanded("ws:app") || tr.IsExpanded("ws:other") {
		t.Error("ResetExpansion should clear all state")
	}
}

func TestToggleExpand(t *testing.T) {
	tr := NewTree()
	tr.ToggleExpand("ws:a")
	if !tr.IsExpanded("ws:a") {
		t.Error("ToggleExpand should expand from false")
	}
	tr.ToggleExpand("ws:a")
	if tr.IsExpanded("ws:a") {
		t.Error("ToggleExpand should collapse from true")
	}
}

// ── Accessors ──

func TestCurrentSelectableSkipsNonSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("d", RowDivider, 0, "", false),
	})
	tr.Cursor = 0
	if tr.CurrentSelectable() != nil {
		t.Error("CurrentSelectable should be nil on a divider")
	}
}

func TestFindByID(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	r, idx := tr.FindByID("b")
	if r == nil || idx != 1 {
		t.Errorf("FindByID(b) = (%+v, %d), want (&{b ...}, 1)", r, idx)
	}
	_, idx = tr.FindByID("nonexistent")
	if idx != -1 {
		t.Errorf("FindByID(nonexistent) should return -1, got %d", idx)
	}
}
