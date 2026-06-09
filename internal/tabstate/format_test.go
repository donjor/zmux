package tabstate

import (
	"strings"
	"testing"
)

func TestParseValidatesStates(t *testing.T) {
	for _, st := range All {
		got, err := Parse(string(st))
		if err != nil || got != st {
			t.Fatalf("Parse(%q) = %v, %v", st, got, err)
		}
	}
	if _, err := Parse("clear"); err == nil {
		t.Fatal("clear is an action, not a state — Parse must reject it")
	}
	if _, err := Parse(""); err == nil {
		t.Fatal("empty state must be rejected")
	}
}

func TestStatusFragmentNestsAllStatesWithEmptyDefault(t *testing.T) {
	frag := StatusFragment(func(st State) string { return "<" + string(st) + ">" })

	for _, st := range All {
		if !strings.Contains(frag, "#{?#{==:#{"+OptState+"},"+string(st)+"},<"+string(st)+">") {
			t.Fatalf("fragment missing conditional for %s:\n%s", st, frag)
		}
	}
	// unset state renders nothing: innermost alternative is empty
	if !strings.Contains(frag, "<done>,}") {
		t.Fatalf("fragment must fall through to empty for unset state:\n%s", frag)
	}
	// highest urgency first
	if strings.Index(frag, string(StateAttention)) > strings.Index(frag, string(StateDone)) {
		t.Fatalf("attention must be the outermost conditional:\n%s", frag)
	}
}
