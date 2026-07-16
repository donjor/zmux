package keys

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// press builds the same v2 KeyPressMsg the TUI surfaces receive at runtime, so
// these tests exercise key.Matches through the real fmt.Stringer path rather
// than comparing raw key slices.
func press(code rune) tea.KeyPressMsg   { return tea.KeyPressMsg{Code: code, Text: string(code)} }
func special(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func TestTUIKeymapSpellings(t *testing.T) {
	cases := []struct {
		name string
		b    key.Binding
		keys []string
	}{
		{"list up", TUIListUp, []string{"up", "k"}},
		{"list down", TUIListDown, []string{"down", "j"}},
		{"list top", TUIListTop, []string{"g"}},
		{"list bottom", TUIListBottom, []string{"G"}},
		{"confirm", TUIConfirm, []string{"enter"}},
		{"cancel", TUICancel, []string{"esc"}},
		{"filter", TUIFilter, []string{"/"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.b.Enabled() {
				t.Fatalf("%s must be enabled", c.name)
			}
			got := c.b.Keys()
			if len(got) != len(c.keys) {
				t.Fatalf("%s keys = %v, want %v", c.name, got, c.keys)
			}
			for i, want := range c.keys {
				if got[i] != want {
					t.Fatalf("%s keys[%d] = %q, want %q", c.name, i, got[i], want)
				}
			}
		})
	}
}

func TestTUIListNavAliasesMatch(t *testing.T) {
	// Primary and vi alias both activate; the opposite direction does not.
	if !key.Matches(special(tea.KeyUp), TUIListUp) || !key.Matches(press('k'), TUIListUp) {
		t.Fatalf("TUIListUp must match both ↑ and k")
	}
	if key.Matches(press('j'), TUIListUp) || key.Matches(special(tea.KeyDown), TUIListUp) {
		t.Fatalf("TUIListUp must not match down/j")
	}
	if !key.Matches(special(tea.KeyDown), TUIListDown) || !key.Matches(press('j'), TUIListDown) {
		t.Fatalf("TUIListDown must match both ↓ and j")
	}
	if key.Matches(press('k'), TUIListDown) || key.Matches(special(tea.KeyUp), TUIListDown) {
		t.Fatalf("TUIListDown must not match up/k")
	}
}

func TestTUITopBottomAreCaseDistinct(t *testing.T) {
	if !key.Matches(press('g'), TUIListTop) || key.Matches(press('G'), TUIListTop) {
		t.Fatalf("TUIListTop matches g only")
	}
	if !key.Matches(press('G'), TUIListBottom) || key.Matches(press('g'), TUIListBottom) {
		t.Fatalf("TUIListBottom matches G only")
	}
}

func TestTUIConfirmCancelFilterAreDisjoint(t *testing.T) {
	// Confirm is bare Enter — Space must NOT leak in (bar/settings own that).
	if !key.Matches(special(tea.KeyEnter), TUIConfirm) {
		t.Fatalf("TUIConfirm must match enter")
	}
	if key.Matches(special(tea.KeySpace), TUIConfirm) {
		t.Fatalf("TUIConfirm must not match space")
	}
	// Cancel is bare Esc — ctrl+c must NOT leak in (help viewer owns that).
	if !key.Matches(special(tea.KeyEsc), TUICancel) {
		t.Fatalf("TUICancel must match esc")
	}
	if key.Matches(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, TUICancel) {
		t.Fatalf("TUICancel must not match ctrl+c")
	}
	if !key.Matches(press('/'), TUIFilter) {
		t.Fatalf("TUIFilter must match /")
	}
}
