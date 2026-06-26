package actions_test

import (
	"testing"

	"github.com/donjor/zmux/internal/actions"
	"github.com/donjor/zmux/internal/keys"
)

func TestRegistryValidates(t *testing.T) {
	if err := actions.Validate(); err != nil {
		t.Fatalf("registry invalid: %v", err)
	}
}

func TestNoDuplicateIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range actions.Specs() {
		if seen[s.ID] {
			t.Errorf("duplicate spec id %q", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestExcludedSpecsHaveReasons(t *testing.T) {
	for _, s := range actions.Specs() {
		if s.Palette == actions.Excluded && s.Reason == "" {
			t.Errorf("excluded spec %q has no reason", s.ID)
		}
	}
}

// TestEveryKeyboundActionClassified is the drift pin: every Prefix/NoPrefix
// keybinding must have a matching actions.Spec, so a new keybind can't land
// without a palette classification. Mirrors the spirit of
// TestKeybindingsDocInSync for the palette surface.
func TestEveryKeyboundActionClassified(t *testing.T) {
	bindings := append(append([]keys.Binding(nil), keys.PrefixBindings...), keys.NoPrefixBindings...)
	for _, b := range bindings {
		if _, ok := actions.ByID(b.Action); !ok {
			t.Errorf("keybinding action %q (%s) has no actions.Spec — classify it in internal/actions", b.Action, b.Key)
		}
	}
}
