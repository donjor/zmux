// Package actions is the neutral source of truth for which canonical zmux verbs
// are surfaced in the command palette and how they execute. It sits between
// internal/keys (the keybinding/conf/doc SSOT) and the palette:
//
//	keys  <-  actions  ->  palette
//
// actions imports neither. keys does not import actions — Binding.Action is a
// stable name, not an execution contract (the tmux command strings live in
// internal/tmux/conf.go). The palette composes this registry with keys (for key
// labels / help / category) and its own dynamic providers.
//
// A spec's ID matches keys.Binding.Action for key-bound verbs; the cross-check
// in actions_test fails the build when a Prefix/NoPrefix binding has no spec —
// that is what stops the palette from silently drifting behind new keybindings.
package actions

import (
	"errors"
	"strings"
)

// PalettePolicy is the coverage class of an action — the load-bearing field for
// the palette coverage gate.
type PalettePolicy string

const (
	// Executable is a static, no-target palette row that runs directly.
	Executable PalettePolicy = "executable"
	// Dynamic needs a target/enumeration and is surfaced by a provider family.
	Dynamic PalettePolicy = "dynamic"
	// OpenSurface opens a dashboard/help surface rather than mutating state.
	OpenSurface PalettePolicy = "open-surface"
	// Excluded is deliberately not a palette action; Reason is required.
	Excluded PalettePolicy = "excluded"
)

// ExecKind selects the executor payload for an action.
type ExecKind string

const (
	ExecNone            ExecKind = "none"
	ExecTmux            ExecKind = "tmux"
	ExecLogicalTab      ExecKind = "logical-tab"
	ExecOpenSurface     ExecKind = "open-surface"
	ExecDynamicProvider ExecKind = "dynamic-provider"
)

// Spec classifies one canonical action for the palette surface.
type Spec struct {
	ID      string        // matches keys.Binding.Action where key-bound
	Palette PalettePolicy // coverage class
	Exec    ExecKind      // executor payload selector
	Reason  string        // required for Excluded; documents the call elsewhere
}

// specs classifies every Prefix + NoPrefix keybinding action (cross-checked in
// actions_test) plus palette-only families added in later phases. Copy-mode,
// inherited tmux defaults, and dashboard-local keys are excluded by *context* in
// the coverage gate, so they carry no spec here.
var specs = []Spec{
	// ── Popups / surfaces (prefix) ──
	{ID: "dashboard", Palette: OpenSurface, Exec: ExecOpenSurface},
	{ID: "help", Palette: OpenSurface, Exec: ExecOpenSurface},
	{ID: "palette", Palette: Excluded, Exec: ExecNone, Reason: "opening the palette from within the palette is a no-op"},
	{ID: "scratch.shell", Palette: Excluded, Exec: ExecNone, Reason: "spawns an interactive $SHELL popup bound to the active pane's cwd; not a fire-and-forget row"},

	// ── Tabs (prefix) ──
	{ID: "new", Palette: Excluded, Exec: ExecNone, Reason: "a zmux tab needs the tab-creation path (pane marker), handled via the dashboard create flow; palette new-tab deferred"},
	{ID: "tab.next", Palette: Executable, Exec: ExecTmux},
	{ID: "tab.prev", Palette: Executable, Exec: ExecTmux},
	{ID: "reorder.left", Palette: Executable, Exec: ExecTmux},
	{ID: "reorder.right", Palette: Executable, Exec: ExecTmux},
	{ID: "kill", Palette: Excluded, Exec: ExecNone, Reason: "destructive tab close with an interactive confirm; not a blind palette row"},
	{ID: "label.tab", Palette: Excluded, Exec: ExecNone, Reason: "prompts for a free-text label (command-prompt); needs input UX the palette lacks"},
	{ID: "tab.pane", Palette: Dynamic, Exec: ExecDynamicProvider, Reason: "needs a source tab to join; surfaced as a per-tab family"},
	{ID: "tab.full", Palette: Dynamic, Exec: ExecDynamicProvider, Reason: "needs a pane-tab to promote; surfaced as a per-tab family"},
	// tab.hide/tab.show have no keybinding (CLI + palette only); surfaced by the
	// same per-tab dynamic family as tab.pane/tab.full.
	{ID: "tab.hide", Palette: Dynamic, Exec: ExecDynamicProvider, Reason: "needs a tab to hide; surfaced as a per-tab family"},
	{ID: "tab.show", Palette: Dynamic, Exec: ExecDynamicProvider, Reason: "needs a docked tab to show; surfaced as a per-tab family"},

	// ── Sessions (prefix) ──
	{ID: "session.picker", Palette: Excluded, Exec: ExecNone, Reason: "nested workspace+session picker popup; the Sessions provider lists sessions directly"},
	{ID: "session.goto.N", Palette: Excluded, Exec: ExecNone, Reason: "numeric quick-jump family; the Sessions provider lists each session as a switch row"},
	{ID: "session.prev", Palette: Excluded, Exec: ExecNone, Reason: "relative session nav; the Sessions provider lists every session as an explicit target"},
	{ID: "session.next", Palette: Excluded, Exec: ExecNone, Reason: "relative session nav; the Sessions provider lists every session as an explicit target"},
	{ID: "rename", Palette: Excluded, Exec: ExecNone, Reason: "prompts for free-text input (command-prompt); needs input UX the palette lacks"},
	{ID: "session.new", Palette: Dynamic, Exec: ExecDynamicProvider, Reason: "the Sessions provider already emits a New session row"},

	// ── Pane layout (prefix) ──
	{ID: "pane.swap.left", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.swap.right", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.swap.up", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.swap.down", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.equalize", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.orient", Palette: Executable, Exec: ExecTmux},

	// ── Panes & general (prefix) ──
	{ID: "pane.respawn", Palette: Excluded, Exec: ExecNone, Reason: "respawn-pane restarts a live pane (destructive); only meaningful on a dead pane via the keybind"},
	{ID: "copy.mode", Palette: Excluded, Exec: ExecNone, Reason: "enters modal copy-mode on the active pane; a terminal mode, not a palette action"},
	{ID: "paste", Palette: Excluded, Exec: ExecNone, Reason: "paste-buffer depends on active-pane + buffer context; not a discoverable row"},
	{ID: "reload", Palette: Excluded, Exec: ExecNone, Reason: "runs the full zmux apply pipeline (config reload), not a tmux primitive; palette wiring deferred"},

	// ── No-prefix (instant) ──
	{ID: "tab.goto.N", Palette: Excluded, Exec: ExecNone, Reason: "direct numeric tab jump; per-tab jump rows would need a tab-list provider, out of scope for this pass"},
	{ID: "tab.switch", Palette: Excluded, Exec: ExecNone, Reason: "nested session+tab switcher popup; the dashboard + tab.goto cover navigation"},
	{ID: "workspace.switch", Palette: Excluded, Exec: ExecNone, Reason: "nested workspace switcher popup; the dashboard covers workspace browsing"},
	{ID: "pane.focus.left", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.focus.right", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.focus.up", Palette: Executable, Exec: ExecTmux},
	{ID: "pane.focus.down", Palette: Executable, Exec: ExecTmux},
}

// Specs returns a copy of the action registry in declaration order.
func Specs() []Spec {
	return append([]Spec(nil), specs...)
}

// ByID returns the spec for an action id (a keys.Binding.Action for key-bound
// verbs), and whether it was found.
func ByID(id string) (Spec, bool) {
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

// Validate checks registry integrity: unique IDs and a non-empty Reason on every
// excluded spec. The returned error aggregates all problems.
func Validate() error {
	var problems []string
	seen := map[string]bool{}
	for _, s := range specs {
		if seen[s.ID] {
			problems = append(problems, "duplicate id: "+s.ID)
		}
		seen[s.ID] = true
		if s.Palette == Excluded && s.Reason == "" {
			problems = append(problems, "excluded spec missing reason: "+s.ID)
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
