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

func TestTmuxHelpSectionsCoverTheTmuxSlices(t *testing.T) {
	got := keys.TmuxHelpSections()
	want := map[string][]keys.Binding{
		"tmux Prefix Keys (Ctrl+Space)":        keys.PrefixBindings,
		"No-Prefix Keys":                       keys.NoPrefixBindings,
		"Inherited tmux Defaults (Ctrl+Space)": keys.InheritedBindings,
	}
	if len(got) != len(want) {
		t.Fatalf("TmuxHelpSections len = %d, want %d", len(got), len(want))
	}
	for title, slice := range want {
		s, ok := sectionByTitle(got, title)
		if !ok {
			t.Errorf("missing section %q", title)
			continue
		}
		if len(s.Bindings) != len(slice) {
			t.Errorf("section %q has %d bindings, want %d", title, len(s.Bindings), len(slice))
		}
	}
}

func TestDashboardHelpSectionsAddCopyMode(t *testing.T) {
	got := keys.DashboardHelpSections()
	// Dashboard shows everything the CLI help does, plus copy mode.
	if _, ok := sectionByTitle(got, "Copy Mode (vi keys)"); !ok {
		t.Errorf("dashboard sections omit Copy Mode")
	}
	for _, title := range []string{
		"tmux Prefix Keys (Ctrl+Space)",
		"Inherited tmux Defaults (Ctrl+Space)",
		"No-Prefix Keys",
	} {
		if _, ok := sectionByTitle(got, title); !ok {
			t.Errorf("dashboard sections missing %q", title)
		}
	}
}

// TestNewBindingSurfacesInBothSurfaces is the drift pin: every prefix binding
// (and so any newly added one) appears in both help section sets without render
// code changes. Asserted by Action identity so a renamed/added binding flows.
func TestNewBindingSurfacesInBothSurfaces(t *testing.T) {
	inPrefixSection := func(sections []keys.KeySection) map[string]bool {
		seen := map[string]bool{}
		for _, s := range sections {
			for _, b := range s.Bindings {
				seen[b.Action] = true
			}
		}
		return seen
	}
	cli := inPrefixSection(keys.TmuxHelpSections())
	dash := inPrefixSection(keys.DashboardHelpSections())
	for _, b := range keys.PrefixBindings {
		if !cli[b.Action] {
			t.Errorf("prefix binding %q absent from TmuxHelpSections", b.Action)
		}
		if !dash[b.Action] {
			t.Errorf("prefix binding %q absent from DashboardHelpSections", b.Action)
		}
	}
}

// TestHelpSectionsNeverEmpty guards against a dropped slice (an empty section
// means a registry slice silently went missing from the help surface).
func TestHelpSectionsNeverEmpty(t *testing.T) {
	for _, set := range [][]keys.KeySection{keys.TmuxHelpSections(), keys.DashboardHelpSections()} {
		for _, s := range set {
			if len(s.Bindings) == 0 {
				t.Errorf("section %q has no bindings", s.Title)
			}
		}
	}
}
