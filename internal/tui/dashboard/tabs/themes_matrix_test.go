package tabs

import (
	"testing"
)

// T-105 (055 P-001) — Themes dashboard-tab behavior matrix. Pins the S-003
// acceptance rows the existing themes_test.go leaves uncovered: the
// CapturesEscape precedence predicate across every mode, apply-on-enter,
// clone→naming entry, and empty-filter enter.
//
// N-01 RESOLUTION (T-302): the preview/revert path (`previewing` flag,
// q:revert, revert-on-deactivate, emitRevert) was dead — never set true by any
// public flow. The outline.Tree convergence is pure cursor/row management and
// does not emit a live preview, so per the ledger disposition the dead path was
// deleted rather than rewired. The four white-box preview pins that guarded the
// dormant mechanism are gone with it; `q` in list mode is now a plain
// unhandled no-op, pinned below.

// --- CapturesEscape precedence matrix ---

func TestThemesCapturesEscapeMatrix(t *testing.T) {
	t.Run("plain list with no filter does not capture esc", func(t *testing.T) {
		tab, _, _ := newTestThemesTab(t)
		tab = activateTheme(t, tab)
		if tab.CapturesEscape() {
			t.Fatal("plain list mode must let esc close the dashboard")
		}
	})

	t.Run("filter mode captures esc", func(t *testing.T) {
		tab, _, _ := newTestThemesTab(t)
		tab = activateTheme(t, tab)
		tab.mode = themesModeFilter
		if !tab.CapturesEscape() {
			t.Fatal("filter mode must capture esc to exit the input")
		}
	})

	t.Run("committed filter in list mode captures esc", func(t *testing.T) {
		tab, _, _ := newTestThemesTab(t)
		tab = activateTheme(t, tab)
		tab.filter.SetValue("dark")
		if !tab.CapturesEscape() {
			t.Fatal("committed filter must capture esc to clear it")
		}
	})

	t.Run("editing captures esc", func(t *testing.T) {
		tab, _, _ := newTestThemesTab(t)
		tab = activateTheme(t, tab)
		tab.editing = true
		if !tab.CapturesEscape() {
			t.Fatal("editor must capture esc to return to the list")
		}
	})
}

// --- apply-on-enter emits an apply command ---

func TestThemesEnterAppliesHighlightedTheme(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	if len(tab.filtered) == 0 {
		t.Fatal("expected themes after activate")
	}
	want := tab.currentThemeInfo().Name

	_, cmd := sendThemesKey(tab, "enter")
	if cmd == nil {
		t.Fatal("enter on a highlighted theme should return an apply command")
	}
	msg := cmd()
	apply, ok := msg.(themesApplyMsg)
	if !ok {
		t.Fatalf("enter cmd should yield themesApplyMsg, got %T", msg)
	}
	if apply.err != nil {
		t.Fatalf("apply should succeed for bundled theme: %v", apply.err)
	}
	if apply.themeName != want {
		t.Fatalf("applied %q, want highlighted %q", apply.themeName, want)
	}
}

// --- empty filter result: enter is a no-op ---

func TestThemesEnterOnEmptyFilterIsNoop(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	// Narrow to nothing, then commit back to list mode with an empty view.
	tab.filter.SetValue("zzz-no-such-theme-xyz")
	tab.applyFilter()
	tab.mode = themesModeList
	if len(tab.filtered) != 0 {
		t.Fatalf("expected empty filtered set, got %d", len(tab.filtered))
	}

	_, cmd := sendThemesKey(tab, "enter")
	if cmd != nil {
		t.Fatal("enter with no highlighted theme must be a no-op (nil cmd)")
	}
}

// --- clone enters the naming prompt ---

func TestThemesCloneEntersNamingPrompt(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	base := tab.currentThemeInfo().Name

	out, _ := sendThemesKey(tab, "c")
	tab = out
	if !tab.editing || !tab.namingActive {
		t.Fatalf("clone should enter editing+naming: editing=%v naming=%v", tab.editing, tab.namingActive)
	}
	if tab.editName != base+"-custom" {
		t.Fatalf("clone name = %q, want %q", tab.editName, base+"-custom")
	}
	// ShortHelp reflects the naming sub-mode.
	if h := tab.ShortHelp(); h != "enter:save  esc:cancel" {
		t.Fatalf("naming ShortHelp = %q", h)
	}
}

// --- edit enters the color editor ---

func TestThemesEditEntersColorEditor(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	out, _ := sendThemesKey(tab, "e")
	tab = out
	if !tab.editing || tab.namingActive || tab.pickerActive {
		t.Fatalf("edit should enter editing only: editing=%v naming=%v picker=%v",
			tab.editing, tab.namingActive, tab.pickerActive)
	}
	if tab.editName == "" {
		t.Fatal("edit should seed editName from the highlighted theme")
	}
}

// --- q in list mode is a plain unhandled no-op (preview path deleted) ---

func TestThemesQIsNoop(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	before := tab.currentThemeInfo()
	_, cmd := sendThemesKey(tab, "q")
	if cmd != nil {
		t.Fatal("q in list mode must be a no-op (no revert binding)")
	}
	if after := tab.currentThemeInfo(); after != before {
		t.Fatal("q must not move the cursor")
	}
}
