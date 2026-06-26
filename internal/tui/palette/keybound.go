package palette

import (
	"strings"

	"github.com/donjor/zmux/internal/actions"
	"github.com/donjor/zmux/internal/keys"
)

// ── Keybound payloads ──
//
// Executable key-bound actions run a tmux primitive directly. The op is a typed
// enum (not a raw argv) so the executor switches on payload type then dispatches
// the op to a typed tmux.Runner method — keeping tmux syntax out of the palette.

// PaneOp identifies a direct pane-layout operation.
type PaneOp int

const (
	PaneSwapLeft PaneOp = iota
	PaneSwapRight
	PaneSwapUp
	PaneSwapDown
	PaneEqualize
	PaneOrient
	PaneFocusLeft
	PaneFocusRight
	PaneFocusUp
	PaneFocusDown
)

// PaneActionPayload runs a pane-layout op on the active pane/window.
type PaneActionPayload struct{ Op PaneOp }

// TabOp identifies a direct tab (tmux window) navigation/reorder operation.
type TabOp int

const (
	TabNext TabOp = iota
	TabPrev
	TabReorderLeft
	TabReorderRight
)

// TabActionPayload runs a tab op relative to the active tab.
type TabActionPayload struct{ Op TabOp }

// keyboundPayloads maps an executable spec id (a keys.Binding.Action) to its
// typed payload. A spec classified Executable with no entry here is a gap the
// coverage gate (palette ↔ actions) fails on.
var keyboundPayloads = map[string]any{
	"tab.next":         TabActionPayload{Op: TabNext},
	"tab.prev":         TabActionPayload{Op: TabPrev},
	"reorder.left":     TabActionPayload{Op: TabReorderLeft},
	"reorder.right":    TabActionPayload{Op: TabReorderRight},
	"pane.swap.left":   PaneActionPayload{Op: PaneSwapLeft},
	"pane.swap.right":  PaneActionPayload{Op: PaneSwapRight},
	"pane.swap.up":     PaneActionPayload{Op: PaneSwapUp},
	"pane.swap.down":   PaneActionPayload{Op: PaneSwapDown},
	"pane.equalize":    PaneActionPayload{Op: PaneEqualize},
	"pane.orient":      PaneActionPayload{Op: PaneOrient},
	"pane.focus.left":  PaneActionPayload{Op: PaneFocusLeft},
	"pane.focus.right": PaneActionPayload{Op: PaneFocusRight},
	"pane.focus.up":    PaneActionPayload{Op: PaneFocusUp},
	"pane.focus.down":  PaneActionPayload{Op: PaneFocusDown},
}

// ── Keybound Provider ──

// KeyboundProvider derives static palette entries for executable key-bound
// actions from the neutral action registry (internal/actions) joined to the
// keybinding registry (internal/keys). New executable keybindings appear here
// automatically once classified; the coverage gate fails the build if one isn't.
type KeyboundProvider struct{}

func (p *KeyboundProvider) Actions() ([]Action, error) {
	var out []Action
	for _, spec := range actions.Specs() {
		if spec.Palette != actions.Executable {
			continue
		}
		payload, ok := keyboundPayloads[spec.ID]
		if !ok {
			continue // executable but unwired — coverage gate reports it
		}
		b, ok := keys.FindKeybound(spec.ID)
		if !ok {
			continue
		}
		out = append(out, Action{
			ID:       "key:" + spec.ID,
			Group:    string(b.Category),
			Title:    b.Help,
			Hint:     b.DisplayKey(),
			Keywords: keyboundKeywords(spec.ID, b),
			Kind:     ActionExec,
			Payload:  payload,
		})
	}
	return out, nil
}

// keyboundKeywords builds fuzzy-match terms from the dotted action id, the
// category, and the humanized key, deduped.
func keyboundKeywords(id string, b keys.Binding) []string {
	seen := map[string]bool{}
	var kw []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		kw = append(kw, s)
	}
	for _, part := range strings.Split(id, ".") {
		add(part)
	}
	add(string(b.Category))
	add(b.Humanize())
	return kw
}
