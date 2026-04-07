package outline

// Tree is a flat outline with a moving cursor and a set of expanded parent
// IDs. Callers rebuild Rows from upstream data whenever it changes; the
// Tree keeps cursor and expansion stable across rebuilds by ID.
type Tree struct {
	Rows   []Row
	Cursor int

	// expanded holds the set of expanded parent IDs. Keyed by stable ID
	// (not row index) so it survives rebuilds.
	expanded map[string]bool
}

// NewTree returns an empty tree with expansion state initialized.
func NewTree() *Tree {
	return &Tree{expanded: make(map[string]bool)}
}

// ── Rows + cursor restoration ──

// SetRows replaces the rows and re-pins the cursor by its previous ID.
// If the previous ID is gone, walks the 5-step fallback chain documented
// in restoreCursor.
func (t *Tree) SetRows(rows []Row) {
	prevID, prevParentID, prevDepth := t.snapshotCursor()
	t.Rows = rows
	t.restoreCursor(prevID, prevParentID, prevDepth)
}

// SetRowsAndJumpTo is the explicit "I just mutated this row's identity"
// hook. Used by rename/move flows where the row's ID changes as a result
// of the mutation, so cursor-by-old-ID would lose the user.
//
// If targetID exists in the new rows, the cursor lands exactly there.
// Otherwise falls through to the same restore logic as SetRows.
func (t *Tree) SetRowsAndJumpTo(rows []Row, targetID string) {
	prevID, prevParentID, prevDepth := t.snapshotCursor()
	t.Rows = rows
	if idx := t.indexOf(targetID); idx >= 0 {
		t.Cursor = idx
		return
	}
	t.restoreCursor(prevID, prevParentID, prevDepth)
}

// snapshotCursor returns the previous cursor row's id, parent id, and
// depth. Used by cursor restore to look up the old position after rows
// are replaced.
func (t *Tree) snapshotCursor() (id, parent string, depth int) {
	if t.Cursor < 0 || t.Cursor >= len(t.Rows) {
		return "", "", 0
	}
	r := t.Rows[t.Cursor]
	return r.ID, r.ParentID, r.Depth
}

// restoreCursor walks the 5-step fallback chain:
//
//  1. Try previous row's ID — exact match.
//  2. Try previous row's ParentID — lands on the parent header.
//  3. Try first selectable row at the previous depth.
//  4. Clamp to first selectable row in Rows.
//  5. If nothing is selectable, Cursor = 0.
//
// This is the single source of truth for the "row disappeared" case.
func (t *Tree) restoreCursor(prevID, prevParent string, prevDepth int) {
	if len(t.Rows) == 0 {
		t.Cursor = 0
		return
	}

	// 1. Exact previous ID.
	if prevID != "" {
		if i := t.indexOf(prevID); i >= 0 {
			t.Cursor = i
			return
		}
	}

	// 2. Parent ID.
	if prevParent != "" {
		if i := t.indexOf(prevParent); i >= 0 {
			t.Cursor = i
			return
		}
	}

	// 3. First selectable row at the same depth.
	for i, r := range t.Rows {
		if r.Depth == prevDepth && r.Selectable {
			t.Cursor = i
			return
		}
	}

	// 4. First selectable row anywhere.
	for i, r := range t.Rows {
		if r.Selectable {
			t.Cursor = i
			return
		}
	}

	// 5. No selectable rows at all.
	t.Cursor = 0
}

// ── Expansion state ──

// IsExpanded reports whether the row with id is expanded.
func (t *Tree) IsExpanded(id string) bool { return t.expanded[id] }

// ToggleExpand flips the expansion state for id.
func (t *Tree) ToggleExpand(id string) { t.expanded[id] = !t.expanded[id] }

// SetExpanded sets the expansion state for id directly.
func (t *Tree) SetExpanded(id string, v bool) {
	if v {
		t.expanded[id] = true
	} else {
		delete(t.expanded, id)
	}
}

// ResetExpansion clears all expansion state.
func (t *Tree) ResetExpansion() {
	t.expanded = make(map[string]bool)
}

// ── Accessors ──

// Current returns the row under the cursor, or nil if out of range.
func (t *Tree) Current() *Row {
	if t.Cursor < 0 || t.Cursor >= len(t.Rows) {
		return nil
	}
	return &t.Rows[t.Cursor]
}

// CurrentSelectable returns the current row if it's selectable, else nil.
// Callers that perform actions should use this to avoid operating on
// dividers / placeholders.
func (t *Tree) CurrentSelectable() *Row {
	r := t.Current()
	if r == nil || !r.Selectable {
		return nil
	}
	return r
}

// FindByID returns the row with the given ID and its index, or nil, -1.
// Linear scan — the row count is bounded by the user's workspace count
// and comfortably small.
func (t *Tree) FindByID(id string) (*Row, int) {
	if id == "" {
		return nil, -1
	}
	for i := range t.Rows {
		if t.Rows[i].ID == id {
			return &t.Rows[i], i
		}
	}
	return nil, -1
}

// indexOf returns the row index of id, or -1 if not found.
func (t *Tree) indexOf(id string) int {
	if id == "" {
		return -1
	}
	for i := range t.Rows {
		if t.Rows[i].ID == id {
			return i
		}
	}
	return -1
}
