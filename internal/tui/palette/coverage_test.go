package palette

import (
	"testing"

	"github.com/donjor/zmux/internal/actions"
	"github.com/donjor/zmux/internal/keys"
	"github.com/donjor/zmux/internal/tmux"
)

// TestPaletteCoversEveryInScopeBinding is the drift pin for the palette surface,
// mirroring keys.TestKeybindingsDocInSync. Every prefix/no-prefix keybinding must
// be classified in internal/actions, and each policy class must be honored by the
// palette: executable → a static row + a live executor case; dynamic/open-surface
// → a provider family declares it (Covers); excluded → a documented reason. A new
// keybind with no classification, or a classified one with no surface, fails here.
func TestPaletteCoversEveryInScopeBinding(t *testing.T) {
	mock := tmux.NewMockRunner()
	reg := NewDefaultRegistry(mock, nil, newFakeFS("/home/u"))

	// Coverage declared by providers (dynamic families + open surfaces).
	covered := map[string]bool{}
	for _, p := range reg.providers {
		if dc, ok := p.(CoverageDeclarer); ok {
			for _, id := range dc.Covers() {
				covered[id] = true
			}
		}
	}

	// Static rows the keybound provider can render.
	keybound, _ := (&KeyboundProvider{}).Actions()
	staticRows := map[string]bool{}
	for _, a := range keybound {
		staticRows[a.ID] = true
	}

	inScope := append(append([]keys.Binding(nil), keys.PrefixBindings...), keys.NoPrefixBindings...)
	for _, b := range inScope {
		spec, ok := actions.ByID(b.Action)
		if !ok {
			t.Errorf("binding %q (%s) has no actions.Spec — classify it in internal/actions", b.Action, b.Key)
			continue
		}
		switch spec.Palette {
		case actions.Executable:
			if !staticRows["key:"+spec.ID] {
				t.Errorf("executable %q renders no keybound palette row", spec.ID)
			}
			assertExecutorHandles(t, spec.ID)
		case actions.Dynamic, actions.OpenSurface:
			if !covered[spec.ID] {
				t.Errorf("%s spec %q is not declared by any provider family (Covers())", spec.Palette, spec.ID)
			}
		case actions.Excluded:
			if spec.Reason == "" {
				t.Errorf("excluded %q has no reason", spec.ID)
			}
		default:
			t.Errorf("spec %q has unknown palette policy %q", spec.ID, spec.Palette)
		}
		assertExecConsistent(t, spec)
	}

	// Palette-only dynamic specs (tab.hide/show — no keybinding) must also be
	// declared by a family, or they would silently never surface.
	for _, s := range actions.Specs() {
		if s.Palette == actions.Dynamic && !covered[s.ID] {
			t.Errorf("dynamic spec %q not declared by any provider family", s.ID)
		}
	}
}

// assertExecutorHandles proves an executable action's payload hits a real
// executor case rather than the silent default close: a handled op records at
// least one Runner call against a fresh mock; the default case records none.
func assertExecutorHandles(t *testing.T, id string) {
	t.Helper()
	payload, ok := keyboundPayloads[id]
	if !ok {
		t.Errorf("executable %q has no executor payload (wire keyboundPayloads)", id)
		return
	}
	mock := tmux.NewMockRunner()
	exe := NewExecutor(mock, newFakeFS("/h"), noopOvermind{}, nil)
	exe.Run(Action{Payload: payload})
	if len(mock.Calls) == 0 {
		t.Errorf("executable %q payload %T hit no executor case (default close)", id, payload)
	}
}

// assertExecConsistent checks the policy ↔ exec invariant so a misclassified
// spec can't slip through.
func assertExecConsistent(t *testing.T, s actions.Spec) {
	t.Helper()
	want := map[actions.PalettePolicy]actions.ExecKind{
		actions.Executable:  actions.ExecTmux,
		actions.Dynamic:     actions.ExecDynamicProvider,
		actions.OpenSurface: actions.ExecOpenSurface,
		actions.Excluded:    actions.ExecNone,
	}
	if w, ok := want[s.Palette]; ok && s.Exec != w {
		t.Errorf("spec %q: policy %q wants exec %q, got %q", s.ID, s.Palette, w, s.Exec)
	}
}
