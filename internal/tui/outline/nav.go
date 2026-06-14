package outline

// MoveUp moves the cursor one selectable row up. Skips non-selectable
// rows (dividers, placeholders). No-op at the top.
func (t *Tree) MoveUp() { t.moveBy(-1) }

// MoveDown moves the cursor one selectable row down. Skips non-selectable
// rows. No-op at the bottom.
func (t *Tree) MoveDown() { t.moveBy(+1) }

// JumpTop moves the cursor to the first selectable row.
func (t *Tree) JumpTop() {
	for i := range t.Rows {
		if t.Rows[i].Selectable {
			t.Cursor = i
			return
		}
	}
}

// JumpBottom moves the cursor to the last selectable row.
func (t *Tree) JumpBottom() {
	for i := len(t.Rows) - 1; i >= 0; i-- {
		if t.Rows[i].Selectable {
			t.Cursor = i
			return
		}
	}
}

// NeighborID returns the stable ID of the nearest selectable row the cursor
// should land on when the current row (and its descendants) is removed.
//
// It prefers the next selectable row after the cursor's subtree, falling
// back to the previous selectable row before the cursor. Descendants of the
// cursor row — the contiguous deeper-depth block immediately after it — are
// skipped, since removing a parent removes its children too. Non-selectable
// dividers/placeholders are skipped. Returns "" when no other selectable row
// exists (single-row or fully-non-selectable lists).
//
// Shared by the picker's delete flow (land on the next cleanup row, not the
// workspace header) and any list surface that removes the focused row.
func (t *Tree) NeighborID() string {
	if t.Cursor < 0 || t.Cursor >= len(t.Rows) {
		return ""
	}
	depth := t.Rows[t.Cursor].Depth

	// Forward: skip the removed row's descendants (contiguous deeper rows),
	// then take the first selectable row.
	inSubtree := true
	for i := t.Cursor + 1; i < len(t.Rows); i++ {
		if inSubtree && t.Rows[i].Depth > depth {
			continue
		}
		inSubtree = false
		if t.Rows[i].Selectable {
			return t.Rows[i].ID
		}
	}

	// Nothing usable after the cursor — fall back to the previous selectable
	// row (rows before the cursor are never its descendants).
	for i := t.Cursor - 1; i >= 0; i-- {
		if t.Rows[i].Selectable {
			return t.Rows[i].ID
		}
	}
	return ""
}

// JumpToID moves the cursor to the row with id. Returns true on success.
// Non-selectable target rows are still jumped to — callers that want to
// skip them should check CurrentSelectable after.
func (t *Tree) JumpToID(id string) bool {
	if idx := t.indexOf(id); idx >= 0 {
		t.Cursor = idx
		return true
	}
	return false
}

// moveBy walks the cursor by delta until it finds a selectable row.
// Guards against infinite loops if every row is non-selectable.
func (t *Tree) moveBy(delta int) {
	if len(t.Rows) == 0 || delta == 0 {
		return
	}
	i := t.Cursor + delta
	for i >= 0 && i < len(t.Rows) && !t.Rows[i].Selectable {
		i += delta
	}
	if i < 0 || i >= len(t.Rows) {
		return // hit an edge without finding a selectable row
	}
	t.Cursor = i
}
