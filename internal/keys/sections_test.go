package keys_test

import (
	"testing"

	"github.com/donjor/zmux/internal/keys"
)

// sectionByTitle finds a section in a set by title.
func sectionByTitle(sections []keys.KeySection, title string) (keys.KeySection, bool) {
	for _, s := range sections {
		if s.Title == title {
			return s, true
		}
	}
	return keys.KeySection{}, false
}

func TestDashboardHelpSectionsAddCopyMode(t *testing.T) {
	got := keys.DashboardHelpSections()
	// Dashboard shows everything the CLI help does, plus copy mode.
	if _, ok := sectionByTitle(got, "Copy Mode (vi keys)"); !ok {
		t.Errorf("dashboard sections omit Copy Mode")
	}
	for _, title := range []string{
		"Prefix Keys (Ctrl+Space)",
		"Inherited from tmux",
		"No-Prefix Keys",
	} {
		if _, ok := sectionByTitle(got, title); !ok {
			t.Errorf("dashboard sections missing %q", title)
		}
	}
}

// TestEveryPrefixBindingInDashboardSections is the drift pin: every prefix
// binding (and so any newly added one) appears in the dashboard help section set
// without render code changes. The cli/viewer surface (help.Sections) carries
// its own pin in internal/help. Asserted by Action identity so a renamed/added
// binding flows.
func TestEveryPrefixBindingInDashboardSections(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range keys.DashboardHelpSections() {
		for _, b := range s.Bindings {
			seen[b.Action] = true
		}
	}
	for _, b := range keys.PrefixBindings {
		if !seen[b.Action] {
			t.Errorf("prefix binding %q absent from DashboardHelpSections", b.Action)
		}
	}
}

// TestHelpSectionsNeverEmpty guards against a dropped slice (an empty section
// means a registry slice silently went missing from the help surface).
func TestHelpSectionsNeverEmpty(t *testing.T) {
	for _, set := range [][]keys.KeySection{keys.DashboardHelpSections()} {
		for _, s := range set {
			if len(s.Bindings) == 0 {
				t.Errorf("section %q has no bindings", s.Title)
			}
		}
	}
}
