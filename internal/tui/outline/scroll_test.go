package outline

import "testing"

func TestComputeWindowSmallList(t *testing.T) {
	// total < height — render everything.
	start, end := ComputeWindow(0, 5, 20)
	if start != 0 || end != 5 {
		t.Errorf("small list: got [%d, %d), want [0, 5)", start, end)
	}
}

func TestComputeWindowCursorAtTop(t *testing.T) {
	// Cursor is at top, window starts at 0.
	start, end := ComputeWindow(0, 100, 10)
	if start != 0 || end != 10 {
		t.Errorf("cursor at top: got [%d, %d), want [0, 10)", start, end)
	}
}

func TestComputeWindowCursorAtBottom(t *testing.T) {
	// Cursor near bottom, window ends at total.
	start, end := ComputeWindow(99, 100, 10)
	if end != 100 || start != 90 {
		t.Errorf("cursor near bottom: got [%d, %d), want [90, 100)", start, end)
	}
}

func TestComputeWindowCursorCentered(t *testing.T) {
	// Cursor in middle, honours scroll margin.
	start, end := ComputeWindow(50, 100, 10)
	if end-start != 10 {
		t.Errorf("window should have width 10, got %d", end-start)
	}
	if 50 < start || 50 >= end {
		t.Errorf("cursor 50 must be inside window [%d, %d)", start, end)
	}
}

func TestComputeWindowZeroHeight(t *testing.T) {
	start, end := ComputeWindow(0, 10, 0)
	if start != 0 || end != 0 {
		t.Errorf("height=0: got [%d, %d), want [0, 0)", start, end)
	}
}

func TestComputeWindowZeroTotal(t *testing.T) {
	start, end := ComputeWindow(0, 0, 10)
	if start != 0 || end != 0 {
		t.Errorf("total=0: got [%d, %d), want [0, 0)", start, end)
	}
}

func TestComputeWindowCursorOutOfRange(t *testing.T) {
	// Cursor > total gets clamped, window still valid.
	_, end := ComputeWindow(999, 100, 10)
	if end != 100 {
		t.Errorf("clamped cursor: end should be 100, got %d", end)
	}
}

func TestComputeWindowNegativeCursor(t *testing.T) {
	// Negative cursor gets clamped to 0.
	start, end := ComputeWindow(-5, 100, 10)
	if start != 0 || end != 10 {
		t.Errorf("negative cursor: got [%d, %d), want [0, 10)", start, end)
	}
}
