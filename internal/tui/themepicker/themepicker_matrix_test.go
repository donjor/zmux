package themepicker

import (
	"testing"
)

// T-105 (055 P-001) — standalone theme-picker behavior matrix. Complements
// themepicker_test.go by pinning the S-003 acceptance rows it omits: ctrl+c
// quit parity, filter commit-vs-cancel value handling, bottom-of-list clamp,
// enter on an empty filtered view, and apply-after-filter selection. These fix
// CURRENT behavior ahead of any S-003 convergence work.

func TestThemePickerCtrlCQuits(t *testing.T) {
	model := newTestThemePicker()

	model = sendThemeKey(model, "ctrl+c")
	if !model.Quitting {
		t.Error("expected Quitting after ctrl+c")
	}
	if model.Chosen != "" {
		t.Errorf("ctrl+c should not choose a theme, got %q", model.Chosen)
	}
}

func TestThemePickerNavigateClampsAtBottom(t *testing.T) {
	model := newTestThemePicker()
	last := len(model.filtered) - 1
	if last < 1 {
		t.Skip("need at least 2 bundled themes")
	}

	// Walk past the end; the cursor must clamp to the final row.
	for i := 0; i < last+3; i++ {
		model = sendThemeKey(model, "j")
	}
	if model.tree.Cursor != last {
		t.Errorf("cursor = %d, want clamped to last row %d", model.tree.Cursor, last)
	}
}

// Committing a filter with enter keeps the query and the narrowed view;
// esc (covered in themepicker_test.go) instead clears it. This pins the
// retain-vs-clear split.
func TestThemePickerFilterEnterRetainsQuery(t *testing.T) {
	model := newTestThemePicker()
	fullCount := len(model.filtered)

	model = sendThemeKey(model, "/")
	model.filter.SetValue("dark")
	model.applyFilter()
	narrowed := len(model.filtered)
	if narrowed == 0 || narrowed >= fullCount {
		t.Fatalf("filter 'dark' should narrow the list: %d of %d", narrowed, fullCount)
	}

	model = sendThemeKey(model, "enter")
	if model.mode != themeList {
		t.Error("enter in filter mode should return to list mode")
	}
	if model.filter.Value() != "dark" {
		t.Errorf("enter should retain the committed query, got %q", model.filter.Value())
	}
	if len(model.filtered) != narrowed {
		t.Errorf("committed filter should keep the narrowed view: %d, want %d",
			len(model.filtered), narrowed)
	}
}

func TestThemePickerEnterOnEmptyFilterChoosesNothing(t *testing.T) {
	model := newTestThemePicker()

	model = sendThemeKey(model, "/")
	model.filter.SetValue("zzz-no-such-theme-xyz")
	model.applyFilter()
	if len(model.filtered) != 0 {
		t.Fatalf("expected empty filtered set, got %d", len(model.filtered))
	}
	// Commit back to list mode, then enter on the empty view.
	model = sendThemeKey(model, "enter")
	model = sendThemeKey(model, "enter")
	if model.Chosen != "" {
		t.Errorf("enter with no highlighted theme must not choose, got %q", model.Chosen)
	}
}

func TestThemePickerApplyAfterFilter(t *testing.T) {
	model := newTestThemePicker()
	if len(model.themes) < 3 {
		t.Skip("need at least 3 bundled themes")
	}
	target := model.themes[2].Name

	model = sendThemeKey(model, "/")
	model.filter.SetValue(target)
	model.applyFilter()
	if cur := model.currentTheme(); cur == nil || cur.Name != target {
		t.Fatalf("cursor after filter = %v, want %q", nameOrNil(cur), target)
	}

	// Commit the filter, then choose the highlighted (filtered) theme.
	model = sendThemeKey(model, "enter") // filter -> list, cursor retained
	model = sendThemeKey(model, "enter") // choose
	if model.Chosen != target {
		t.Errorf("apply-after-filter chose %q, want %q", model.Chosen, target)
	}
}
