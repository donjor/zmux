package outline

import "testing"

func TestMoveDownSkipsNonSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("d", RowDivider, 0, "", false),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0
	tr.MoveDown()
	if tr.Cursor != 2 {
		t.Errorf("MoveDown should skip divider and land on index 2, got %d", tr.Cursor)
	}
}

func TestMoveUpSkipsNonSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("d", RowDivider, 0, "", false),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 2
	tr.MoveUp()
	if tr.Cursor != 0 {
		t.Errorf("MoveUp should skip divider and land on index 0, got %d", tr.Cursor)
	}
}

func TestMoveUpAtTopIsNoop(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0
	tr.MoveUp()
	if tr.Cursor != 0 {
		t.Errorf("MoveUp at top should be no-op, got %d", tr.Cursor)
	}
}

func TestMoveDownAtBottomIsNoop(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 1
	tr.MoveDown()
	if tr.Cursor != 1 {
		t.Errorf("MoveDown at bottom should be no-op, got %d", tr.Cursor)
	}
}

func TestJumpTopLandsOnFirstSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("d", RowDivider, 0, "", false),
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 2
	tr.JumpTop()
	if tr.Cursor != 1 {
		t.Errorf("JumpTop should skip divider to index 1, got %d", tr.Cursor)
	}
}

func TestJumpBottomLandsOnLastSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
		mkRow("d", RowDivider, 0, "", false),
	})
	tr.Cursor = 0
	tr.JumpBottom()
	if tr.Cursor != 1 {
		t.Errorf("JumpBottom should skip trailing divider to index 1, got %d", tr.Cursor)
	}
}

func TestJumpToIDHits(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	if !tr.JumpToID("b") || tr.Cursor != 1 {
		t.Errorf("JumpToID('b') should move cursor to 1")
	}
}

func TestJumpToIDMisses(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
	})
	if tr.JumpToID("nonexistent") {
		t.Errorf("JumpToID should return false on miss")
	}
}

// ── NeighborID (delete-jump primitive) ──

func TestNeighborIDMiddleRowTakesNext(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
		mkRow("c", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 1 // on "b"
	if got := tr.NeighborID(); got != "c" {
		t.Errorf("NeighborID on middle row should be next 'c', got %q", got)
	}
}

func TestNeighborIDLastRowFallsBackToPrev(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
		mkRow("c", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 2 // on "c", nothing after
	if got := tr.NeighborID(); got != "b" {
		t.Errorf("NeighborID on last row should fall back to prev 'b', got %q", got)
	}
}

func TestNeighborIDSkipsNonSelectable(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("a", RowWorkspaceHeader, 0, "", true),
		mkRow("sep", RowDivider, 0, "", false),
		mkRow("b", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0 // on "a"; next selectable skips the divider
	if got := tr.NeighborID(); got != "b" {
		t.Errorf("NeighborID should skip divider and take 'b', got %q", got)
	}

	tr.Cursor = 2 // on "b"; only the divider precedes, fall back to "a"
	if got := tr.NeighborID(); got != "a" {
		t.Errorf("NeighborID should skip divider backwards to 'a', got %q", got)
	}
}

func TestNeighborIDSkipsRemovedSubtree(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:a", RowSession, 1, "ws:app", true),
		mkRow("session:b", RowSession, 1, "ws:app", true),
		mkRow("ws:other", RowWorkspaceHeader, 0, "", true),
	})
	tr.Cursor = 0 // deleting the whole "ws:app" workspace + its sessions
	if got := tr.NeighborID(); got != "ws:other" {
		t.Errorf("NeighborID should skip the workspace's sessions to 'ws:other', got %q", got)
	}
}

func TestNeighborIDSessionTakesSibling(t *testing.T) {
	tr := NewTree()
	tr.SetRows([]Row{
		mkRow("ws:app", RowWorkspaceHeader, 0, "", true),
		mkRow("session:a", RowSession, 1, "ws:app", true),
		mkRow("session:b", RowSession, 1, "ws:app", true),
	})
	tr.Cursor = 1 // deleting session:a; the next cleanup row is its sibling
	if got := tr.NeighborID(); got != "session:b" {
		t.Errorf("NeighborID on a session should take the sibling 'session:b', got %q", got)
	}
}

func TestNeighborIDEmptyAndSingle(t *testing.T) {
	tr := NewTree()
	if got := tr.NeighborID(); got != "" {
		t.Errorf("NeighborID on empty tree should be \"\", got %q", got)
	}
	tr.SetRows([]Row{mkRow("only", RowWorkspaceHeader, 0, "", true)})
	tr.Cursor = 0
	if got := tr.NeighborID(); got != "" {
		t.Errorf("NeighborID on single-row tree should be \"\", got %q", got)
	}
}
