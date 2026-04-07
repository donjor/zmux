package palette

import (
	"strings"
	"testing"
)

func TestActionSearchTextCombinesEverything(t *testing.T) {
	a := Action{
		Title:    "Switch to dev",
		Subtitle: "attached",
		Group:    "Sessions",
		Keywords: []string{"session", "attach"},
	}
	got := a.searchText()

	for _, want := range []string{"Switch to dev", "attached", "Sessions", "session", "attach"} {
		if !strings.Contains(got, want) {
			t.Errorf("searchText() = %q, missing %q", got, want)
		}
	}
}

func TestActionSearchTextTitleOnly(t *testing.T) {
	a := Action{Title: "New session"}
	got := a.searchText()
	if got != "New session" {
		t.Errorf("searchText() = %q, want %q", got, "New session")
	}
}

func TestActionSearchTextNoKeywords(t *testing.T) {
	a := Action{Title: "T", Group: "G", Subtitle: "S"}
	got := a.searchText()
	// Order: title, subtitle, group, then keywords (none).
	if got != "T S G" {
		t.Errorf("searchText() = %q, want %q", got, "T S G")
	}
}
