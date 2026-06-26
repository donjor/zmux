package palette

import (
	"testing"

	"github.com/donjor/zmux/internal/actions"
	"github.com/donjor/zmux/internal/tmux"
)

func TestKeyboundProviderDerivesExecutableActions(t *testing.T) {
	got, err := (&KeyboundProvider{}).Actions()
	if err != nil {
		t.Fatalf("Actions() error: %v", err)
	}

	byID := map[string]Action{}
	for _, a := range got {
		byID[a.ID] = a
	}

	// A representative pane action carries the right group, hint, and payload.
	swap, ok := byID["key:pane.swap.left"]
	if !ok {
		t.Fatalf("missing key:pane.swap.left; got ids %v", ids(got))
	}
	if swap.Group != "Panes" {
		t.Errorf("group = %q, want Panes", swap.Group)
	}
	if swap.Hint == "" {
		t.Errorf("hint empty; want a prefix-decorated key")
	}
	if p, ok := swap.Payload.(PaneActionPayload); !ok || p.Op != PaneSwapLeft {
		t.Errorf("payload = %#v, want PaneActionPayload{PaneSwapLeft}", swap.Payload)
	}

	// A tab nav action lands in the Tabs group with a TabActionPayload.
	if next, ok := byID["key:tab.next"]; !ok {
		t.Errorf("missing key:tab.next")
	} else if p, ok := next.Payload.(TabActionPayload); !ok || p.Op != TabNext {
		t.Errorf("tab.next payload = %#v, want TabActionPayload{TabNext}", next.Payload)
	}

	// Excluded specs never produce a row.
	for _, excluded := range []string{"key:palette", "key:kill", "key:copy.mode", "key:new"} {
		if _, ok := byID[excluded]; ok {
			t.Errorf("excluded action surfaced as %q", excluded)
		}
	}
}

// TestKeyboundProviderCoversEveryExecutableSpec ensures the payload map keeps up
// with the registry: every spec classified Executable yields exactly one row.
func TestKeyboundProviderCoversEveryExecutableSpec(t *testing.T) {
	got, _ := (&KeyboundProvider{}).Actions()
	have := map[string]bool{}
	for _, a := range got {
		have[a.ID] = true
	}
	for _, s := range actions.Specs() {
		if s.Palette != actions.Executable {
			continue
		}
		if !have["key:"+s.ID] {
			t.Errorf("executable spec %q has no keybound palette row (wire it in keyboundPayloads)", s.ID)
		}
	}
}

// TestDefaultRegistryComposesKeyboundWithDynamic verifies the wired registry
// surfaces both the new keybound group and the pre-existing dynamic groups.
func TestDefaultRegistryComposesKeyboundWithDynamic(t *testing.T) {
	mock := tmux.NewMockRunner()
	reg := NewDefaultRegistry(mock, nil, newFakeFS("/home/u"))

	groups := map[string]bool{}
	for _, a := range reg.All() {
		groups[a.Group] = true
	}
	for _, want := range []string{"Panes", "Tabs", "Bar", "Dashboard"} {
		if !groups[want] {
			t.Errorf("registry missing %q group; got %v", want, groups)
		}
	}
}

func ids(as []Action) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}
