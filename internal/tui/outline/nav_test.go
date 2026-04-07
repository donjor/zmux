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
