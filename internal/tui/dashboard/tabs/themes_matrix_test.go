package tabs

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
)

// T-105 (055 P-001) — Themes dashboard-tab behavior matrix. Pins the S-003
// acceptance rows the existing themes_test.go leaves uncovered: the
// CapturesEscape precedence predicate across every mode, apply-on-enter,
// clone→naming entry, empty-filter enter, ShortHelp preview gating, and the
// preview/revert mechanism.
//
// DORMANCY NOTE (characterization, NOT a defect to fix here): `previewing` is
// never set to true by any key handler or message in the current dashboard
// tab — Activate/apply/revert only ever clear it. So the reachable behavior of
// `q` in list mode is a no-op, and the q:revert / revert-on-deactivate paths
// are dead through the public flow. The revert *mechanism* is still pinned via
// white-box below so any S-003 convergence (T-302/T-303) that rewires preview
// can prove it preserved these semantics. Flagged in worker-notes for the
// conductor; not repaired under the behavior-preserving mandate.

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
	want := tab.filtered[tab.themeCursor].Name

	out, cmd := sendThemesKey(tab, "enter")
	tab = out
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
	base := tab.filtered[tab.themeCursor].Name

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

// --- ShortHelp gates q:revert on the previewing flag ---

func TestThemesShortHelpGatesRevertOnPreviewing(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	if h := tab.ShortHelp(); strings.Contains(h, "q:revert") {
		t.Fatalf("ShortHelp must omit q:revert when not previewing: %q", h)
	}
	tab.previewing = true
	if h := tab.ShortHelp(); !strings.Contains(h, "q:revert") {
		t.Fatalf("ShortHelp must show q:revert while previewing: %q", h)
	}
}

// --- q in list mode is a no-op while not previewing (dormant preview) ---

func TestThemesQIsNoopWhenNotPreviewing(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	if tab.previewing {
		t.Fatal("previewing should default false")
	}

	_, cmd := sendThemesKey(tab, "q")
	if cmd != nil {
		t.Fatal("q in list mode with no active preview must be a no-op")
	}
	if tab.previewing {
		t.Fatal("q must not toggle previewing on")
	}
}

// --- preview/revert mechanism (white-box; dormant through the public flow) ---

func TestThemesPreviewRevertMechanism(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)

	// Stash a saved palette+styles snapshot and force the previewing flag,
	// reproducing the state a rewired preview would establish.
	resolved, err := tab.resolver.Resolve("ayu-dark")
	if err != nil {
		t.Fatalf("resolve ayu-dark: %v", err)
	}
	pal := resolved.SemanticPalette()
	sty := styles.NewStyles(&pal)
	tab.savedPalette = &pal
	tab.savedStyles = &sty
	tab.previewing = true

	out, cmd := sendThemesKey(tab, "q")
	tab = out
	if tab.previewing {
		t.Fatal("q while previewing must clear the previewing flag")
	}
	if cmd == nil {
		t.Fatal("q while previewing must emit a revert command")
	}
	if _, ok := cmd().(dashboard.ThemeChangeIntent); !ok {
		t.Fatalf("revert cmd should broadcast a ThemeChangeIntent, got %T", cmd())
	}
}

// --- revert-on-deactivate mechanism (also dormant; pinned white-box) ---

func TestThemesDeactivateRevertsActivePreview(t *testing.T) {
	tab, _, _ := newTestThemesTab(t)
	tab = activateTheme(t, tab)
	tab.previewing = true

	tab.Deactivate()
	if tab.previewing {
		t.Fatal("Deactivate must clear an active preview")
	}
}
