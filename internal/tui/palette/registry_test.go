package palette

import (
	"errors"
	"testing"
)

// stubProvider implements ActionProvider with either a canned set or an error.
type stubProvider struct {
	actions []Action
	err     error
}

func (p *stubProvider) Actions() ([]Action, error) {
	return p.actions, p.err
}

func TestRegistryAllConcatenatesInOrder(t *testing.T) {
	p1 := &stubProvider{actions: []Action{{ID: "a1"}, {ID: "a2"}}}
	p2 := &stubProvider{actions: []Action{{ID: "b1"}}}
	r := NewRegistry(p1, p2)

	got := r.All()
	if len(got) != 3 {
		t.Fatalf("All() len = %d, want 3", len(got))
	}
	if got[0].ID != "a1" || got[1].ID != "a2" || got[2].ID != "b1" {
		t.Errorf("All() order wrong: %v", []string{got[0].ID, got[1].ID, got[2].ID})
	}
}

func TestRegistryAllSwallowsProviderErrors(t *testing.T) {
	// p1 errors; p2 returns normally — p2's actions must still appear.
	p1 := &stubProvider{err: errors.New("boom")}
	p2 := &stubProvider{actions: []Action{{ID: "ok"}}}
	r := NewRegistry(p1, p2)

	got := r.All()
	if len(got) != 1 {
		t.Fatalf("All() len = %d, want 1 (p1 errored, p2 ok)", len(got))
	}
	if got[0].ID != "ok" {
		t.Errorf("All()[0].ID = %q, want ok", got[0].ID)
	}
}

func TestRegistryAllEmpty(t *testing.T) {
	r := NewRegistry()
	if got := r.All(); len(got) != 0 {
		t.Errorf("empty registry All() = %v, want empty", got)
	}
}
