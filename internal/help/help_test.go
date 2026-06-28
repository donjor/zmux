package help_test

import (
	"testing"

	"github.com/donjor/zmux/internal/help"
	"github.com/donjor/zmux/internal/keys"
)

func sectionTitles(sections []help.Section) map[string]bool {
	m := map[string]bool{}
	for _, s := range sections {
		m[s.Title] = true
	}
	return m
}

// TestSectionsSpanCommandsAndKeybindings: Sections() carries both the command
// reference and the keybinding reference (the latter from the keys registry).
func TestSectionsSpanCommandsAndKeybindings(t *testing.T) {
	titles := sectionTitles(help.Sections())
	for _, want := range []string{
		"Session Management",  // command reference
		"Panes",               // keybinding reference, grouped by category
		"Inherited from tmux", // inherited keybinding reference
	} {
		if !titles[want] {
			t.Errorf("Sections() missing section %q", want)
		}
	}
}

// TestScopeTagging: command and keybinding sections carry distinct scopes that
// partition the full set, so the viewer's commands/keys/all toggle is exact.
func TestScopeTagging(t *testing.T) {
	all := help.Sections()
	cmds := help.FilterScope(all, help.ScopeCommand)
	keySecs := help.FilterScope(all, help.ScopeKeybinding)

	if len(cmds)+len(keySecs) != len(all) {
		t.Fatalf("scopes don't partition: %d + %d != %d", len(cmds), len(keySecs), len(all))
	}
	if !sectionTitles(cmds)["Session Management"] {
		t.Errorf("command scope missing Session Management")
	}
	if sectionTitles(cmds)["Panes"] {
		t.Errorf("command scope leaked a keybinding section (Panes)")
	}
	if !sectionTitles(keySecs)["Panes"] || !sectionTitles(keySecs)["Inherited from tmux"] {
		t.Errorf("keybinding scope missing category/inherited sections")
	}
}

// TestKnownCommandEntryPresent pins the contract shorthand_test relies on, so a
// refactor of the source can't silently drop it.
func TestKnownCommandEntryPresent(t *testing.T) {
	want := "zmux open <ws> [session]"
	for _, s := range help.Sections() {
		for _, e := range s.Entries {
			if e.Label == want {
				return
			}
		}
	}
	t.Errorf("Sections() missing command entry %q", want)
}

// TestEveryPrefixBindingSurfaces is the drift pin: every prefix binding appears
// as a help entry (by its DisplayKey), so a newly added binding shows up in the
// help viewer + `zmux help` without touching help code.
func TestEveryPrefixBindingSurfaces(t *testing.T) {
	labels := map[string]bool{}
	for _, s := range help.Sections() {
		for _, e := range s.Entries {
			labels[e.Label] = true
		}
	}
	for _, b := range keys.PrefixBindings {
		if !labels[b.DisplayKey()] {
			t.Errorf("prefix binding %q (%s) absent from help.Sections()", b.Action, b.DisplayKey())
		}
	}
}

// TestNoEmptySection guards against a dropped slice rendering a bare header.
func TestNoEmptySection(t *testing.T) {
	for _, s := range help.Sections() {
		if len(s.Entries) == 0 {
			t.Errorf("section %q has no entries", s.Title)
		}
	}
}
