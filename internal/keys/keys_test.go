package keys

import "testing"

// TestNoDuplicateKeysPerContext guards against two bindings claiming the same
// key within the same tmux table — that would make conf.go emit a conflicting
// bind. Aliases are checked too.
func TestNoDuplicateKeysPerContext(t *testing.T) {
	seen := map[Context]map[string]string{}
	for _, b := range All() {
		if seen[b.Context] == nil {
			seen[b.Context] = map[string]string{}
		}
		for _, k := range append([]string{b.Key}, b.Aliases...) {
			if prev, ok := seen[b.Context][k]; ok {
				t.Errorf("duplicate key %q in context %q: %q and %q", k, b.Context, prev, b.Action)
			}
			seen[b.Context][k] = b.Action
		}
	}
}

// TestBindingsArePopulated ensures every registered binding carries the metadata
// the help and docs renderers depend on.
func TestBindingsArePopulated(t *testing.T) {
	for _, b := range All() {
		if b.Action == "" || b.Key == "" || b.Help == "" || b.Category == "" || b.Context == "" {
			t.Errorf("incomplete binding: %+v", b)
		}
	}
}
