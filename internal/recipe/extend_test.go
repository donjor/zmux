package recipe

import (
	"fmt"
	"strings"
	"testing"
)

// T-101 (055 P-001) — behavioral characterization of recipe inheritance
// (resolveDefinitions/mergeRecipe). Pins CURRENT merge semantics before any
// movement. Covers S-005 acceptance: missing names, unknown parents,
// self/indirect cycles, declaration order, multi-level extends, scalar/default/
// options inheritance + overrides, zero-value boolean semantics, tab
// replace/append, session + default-session merge, nested tab merge,
// post-merge validation, and parent non-mutation via deep values.

// resolveAll resolves defs built from the given recipes (declaration order =
// slice order) and returns the resolved recipes keyed by name.
func resolveAll(t *testing.T, recipes ...Recipe) map[string]Recipe {
	t.Helper()
	defs := makeDefs(recipes)
	if err := resolveDefinitions(defs); err != nil {
		t.Fatalf("resolveDefinitions: %v", err)
	}
	out := map[string]Recipe{}
	for _, d := range defs {
		out[d.Recipe.Name] = d.Recipe
	}
	return out
}

// resolveErr resolves and expects a failure whose message contains want.
func resolveErr(t *testing.T, want string, recipes ...Recipe) {
	t.Helper()
	err := resolveDefinitions(makeDefs(recipes))
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

func makeDefs(recipes []Recipe) []Definition {
	defs := make([]Definition, len(recipes))
	for i, r := range recipes {
		defs[i] = Definition{Recipe: r, Source: SourceUser, Path: fmt.Sprintf("mem-%d.toml", i)}
	}
	return defs
}

// baseSession is a minimal valid session recipe with one tab.
func baseSession(name string, tabs ...TabSpec) Recipe {
	if len(tabs) == 0 {
		tabs = []TabSpec{{Name: "main", Command: "echo hi"}}
	}
	return Recipe{Name: name, Kind: KindSession, Tabs: tabs}
}

func TestResolveRejectsMissingName(t *testing.T) {
	resolveErr(t, "missing required field: name",
		Recipe{Name: "", Kind: KindSession, Tabs: []TabSpec{{Name: "main"}}})
}

func TestResolveRejectsUnknownParent(t *testing.T) {
	resolveErr(t, `extends unknown recipe "ghost"`,
		Recipe{Name: "child", Kind: KindSession, Extends: "ghost"})
}

func TestResolveRejectsSelfCycle(t *testing.T) {
	resolveErr(t, "extends cycle",
		Recipe{Name: "loop", Kind: KindSession, Extends: "loop", Tabs: []TabSpec{{Name: "main"}}})
}

func TestResolveRejectsIndirectCycle(t *testing.T) {
	resolveErr(t, "extends cycle",
		Recipe{Name: "a", Kind: KindSession, Extends: "b"},
		Recipe{Name: "b", Kind: KindSession, Extends: "a"},
	)
}

func TestResolveOrderIndependent(t *testing.T) {
	// Child declared BEFORE parent still resolves (byName index built up-front).
	childFirst := resolveAll(t,
		Recipe{Name: "child", Kind: KindSession, Extends: "parent", Description: "kid"},
		baseSession("parent", TabSpec{Name: "main", Command: "run"}),
	)
	parentFirst := resolveAll(t,
		baseSession("parent", TabSpec{Name: "main", Command: "run"}),
		Recipe{Name: "child", Kind: KindSession, Extends: "parent", Description: "kid"},
	)
	if childFirst["child"].Tabs[0].Command != "run" {
		t.Fatalf("child-first: inherited tab command = %q", childFirst["child"].Tabs[0].Command)
	}
	if parentFirst["child"].Tabs[0].Command != "run" {
		t.Fatalf("parent-first: inherited tab command = %q", parentFirst["child"].Tabs[0].Command)
	}
	if childFirst["child"].Description != parentFirst["child"].Description {
		t.Fatalf("resolution order affected result: %q vs %q",
			childFirst["child"].Description, parentFirst["child"].Description)
	}
}

func TestResolveMultiLevelExtends(t *testing.T) {
	// c -> b -> a. a supplies tabs + description; b overrides nothing but adds
	// context; c overrides description. Everything flows down the chain.
	got := resolveAll(t,
		Recipe{
			Name: "a", Kind: KindSession, Description: "root", Context: ContextInside,
			Tabs: []TabSpec{{Name: "main", Command: "a-cmd"}},
		},
		Recipe{Name: "b", Kind: KindSession, Extends: "a", Session: "b-sess"},
		Recipe{Name: "c", Kind: KindSession, Extends: "b", Description: "leaf"},
	)
	c := got["c"]
	if c.Description != "leaf" {
		t.Fatalf("c.Description = %q, want leaf", c.Description)
	}
	if c.Context != ContextInside {
		t.Fatalf("c.Context = %q, want inherited inside", c.Context)
	}
	if c.Session != "b-sess" {
		t.Fatalf("c.Session = %q, want inherited b-sess", c.Session)
	}
	if len(c.Tabs) != 1 || c.Tabs[0].Command != "a-cmd" {
		t.Fatalf("c.Tabs = %+v, want inherited a-cmd", c.Tabs)
	}
}

func TestResolveScalarOverrideAndInheritance(t *testing.T) {
	got := resolveAll(t,
		Recipe{
			Name: "parent", Kind: KindSession,
			Description: "pdesc", Context: ContextOutside, Workspace: "pws",
			Session: "psess", CWD: "/parent", ForEach: "items",
			Tabs: []TabSpec{{Name: "main", Command: "pcmd"}},
		},
		Recipe{
			Name: "child", Kind: KindWorkspace, Extends: "parent",
			Description: "cdesc", CWD: "/child",
		},
	)
	c := got["child"]
	// Overridden (child non-empty wins):
	if c.Description != "cdesc" || c.CWD != "/child" || c.Kind != KindWorkspace {
		t.Fatalf("child overrides not applied: %+v", c)
	}
	// Inherited (child empty keeps parent):
	if c.Context != ContextOutside || c.Workspace != "pws" || c.Session != "psess" || c.ForEach != "items" {
		t.Fatalf("child did not inherit parent scalars: %+v", c)
	}
}

func TestResolveDefaultsAndOptionsMerge(t *testing.T) {
	got := resolveAll(t,
		Recipe{
			Name: "parent", Kind: KindSession,
			Defaults: Defaults{Workspace: "pw", Session: "ps", CWD: "/pd", TabMode: TabModeReady},
			Options:  Options{FocusSession: "pf", FocusTab: "pt", Rerun: "skip", TabMode: TabModeReady},
			Tabs:     []TabSpec{{Name: "main"}},
		},
		Recipe{
			Name: "child", Kind: KindSession, Extends: "parent",
			Defaults: Defaults{Session: "cs"},
			Options:  Options{Rerun: "send"},
		},
	)
	c := got["child"]
	if c.Defaults.Session != "cs" || c.Defaults.Workspace != "pw" || c.Defaults.CWD != "/pd" {
		t.Fatalf("defaults merge wrong: %+v", c.Defaults)
	}
	if c.Options.Rerun != "send" || c.Options.FocusSession != "pf" || c.Options.FocusTab != "pt" {
		t.Fatalf("options merge wrong: %+v", c.Options)
	}
}

func TestResolveZeroValueBooleanCannotUnsetParent(t *testing.T) {
	// mergeInputs OR-merges booleans: a child's false (zero value) cannot turn
	// off a parent's true. A child true does set a parent false to true.
	got := resolveAll(t,
		Recipe{
			Name: "parent", Kind: KindSession,
			Inputs: Inputs{Session: true, CWD: true, Prompt: "P"},
			Tabs:   []TabSpec{{Name: "main"}},
		},
		Recipe{
			Name: "child", Kind: KindSession, Extends: "parent",
			Inputs: Inputs{Workspace: true, Session: false},
		},
	)
	in := got["child"].Inputs
	if !in.Session {
		t.Fatalf("child false must not unset parent Session=true: %+v", in)
	}
	if !in.CWD {
		t.Fatalf("child unset CWD must inherit parent true: %+v", in)
	}
	if !in.Workspace {
		t.Fatalf("child Workspace=true must set: %+v", in)
	}
	if in.Prompt != "P" {
		t.Fatalf("child empty Prompt must inherit parent: %+v", in)
	}
}

func TestResolveTabReplaceAppendByName(t *testing.T) {
	// Parent [A, B]; child [B' (override), C]. Result [A, B', C] — replace in
	// place by name, append new, preserve parent order.
	got := resolveAll(t,
		Recipe{Name: "parent", Kind: KindSession, Tabs: []TabSpec{
			{Name: "A", Command: "a1"}, {Name: "B", Command: "b1"},
		}},
		Recipe{Name: "child", Kind: KindSession, Extends: "parent", Tabs: []TabSpec{
			{Name: "B", Command: "b2"}, {Name: "C", Command: "c1"},
		}},
	)
	tabs := got["child"].Tabs
	want := []TabSpec{{Name: "A", Command: "a1"}, {Name: "B", Command: "b2"}, {Name: "C", Command: "c1"}}
	if fmt.Sprintf("%+v", tabs) != fmt.Sprintf("%+v", want) {
		t.Fatalf("tab merge = %+v, want %+v", tabs, want)
	}
}

func TestResolveEmptyChildTabsKeepParentTabs(t *testing.T) {
	// child.Tabs empty -> parent tabs inherited untouched (len(child.Tabs)==0
	// branch skips mergeTabs).
	got := resolveAll(t,
		Recipe{Name: "parent", Kind: KindSession, Tabs: []TabSpec{{Name: "A", Command: "a1"}}},
		Recipe{Name: "child", Kind: KindSession, Extends: "parent", Description: "c"},
	)
	tabs := got["child"].Tabs
	if len(tabs) != 1 || tabs[0].Name != "A" || tabs[0].Command != "a1" {
		t.Fatalf("empty child tabs should inherit parent: %+v", tabs)
	}
}

func TestResolveSessionMergeAndNestedTabs(t *testing.T) {
	// Named session merge: parent session "web" {CWD:/p, tabs:[A,B]};
	// child session "web" {CWD:/c, tabs:[B',C]} and a new session "api".
	got := resolveAll(t,
		Recipe{Name: "parent", Kind: KindWorkspace, Sessions: []SessionSpec{
			{Name: "web", CWD: "/p", ForEach: "items", Tabs: []TabSpec{
				{Name: "A", Command: "a1"}, {Name: "B", Command: "b1"},
			}},
		}},
		Recipe{Name: "child", Kind: KindWorkspace, Extends: "parent", Sessions: []SessionSpec{
			{Name: "web", CWD: "/c", Tabs: []TabSpec{
				{Name: "B", Command: "b2"}, {Name: "C", Command: "c1"},
			}},
			{Name: "api", Tabs: []TabSpec{{Name: "srv", Command: "run"}}},
		}},
	)
	sessions := got["child"].Sessions
	if len(sessions) != 2 {
		t.Fatalf("expected web+api, got %d sessions: %+v", len(sessions), sessions)
	}
	web := sessions[0]
	if web.Name != "web" || web.CWD != "/c" {
		t.Fatalf("web CWD override wrong: %+v", web)
	}
	if web.ForEach != "items" {
		t.Fatalf("web empty child ForEach must inherit parent: %+v", web)
	}
	wantTabs := []TabSpec{{Name: "A", Command: "a1"}, {Name: "B", Command: "b2"}, {Name: "C", Command: "c1"}}
	if fmt.Sprintf("%+v", web.Tabs) != fmt.Sprintf("%+v", wantTabs) {
		t.Fatalf("nested tab merge = %+v, want %+v", web.Tabs, wantTabs)
	}
	if sessions[1].Name != "api" {
		t.Fatalf("new session api not appended: %+v", sessions[1])
	}
}

func TestResolveDefaultSessionMergesUnderPlaceholderKey(t *testing.T) {
	// Two nameless sessions collapse to the same "<default>" merge key.
	got := resolveAll(t,
		Recipe{Name: "parent", Kind: KindWorkspace, Sessions: []SessionSpec{
			{CWD: "/p", Tabs: []TabSpec{{Name: "A", Command: "a1"}}},
		}},
		Recipe{Name: "child", Kind: KindWorkspace, Extends: "parent", Sessions: []SessionSpec{
			{CWD: "/c", Tabs: []TabSpec{{Name: "B", Command: "b1"}}},
		}},
	)
	sessions := got["child"].Sessions
	if len(sessions) != 1 {
		t.Fatalf("default sessions must merge to one: %+v", sessions)
	}
	if sessions[0].CWD != "/c" {
		t.Fatalf("default session CWD override wrong: %+v", sessions[0])
	}
	wantTabs := []TabSpec{{Name: "A", Command: "a1"}, {Name: "B", Command: "b1"}}
	if fmt.Sprintf("%+v", sessions[0].Tabs) != fmt.Sprintf("%+v", wantTabs) {
		t.Fatalf("default session nested tabs = %+v, want %+v", sessions[0].Tabs, wantTabs)
	}
}

func TestResolveExtendsBypassesNeedsTabsValidation(t *testing.T) {
	// A child with no tabs of its own is valid BECAUSE it extends a parent that
	// supplies them (Validate's Extends!="" carve-out + inherited tabs).
	got := resolveAll(t,
		baseSession("parent", TabSpec{Name: "main", Command: "run"}),
		Recipe{Name: "child", Kind: KindSession, Extends: "parent"},
	)
	if len(got["child"].Tabs) != 1 {
		t.Fatalf("child should inherit parent tab: %+v", got["child"])
	}
}

func TestResolvePostMergeValidationRejectsBadResult(t *testing.T) {
	// Validation runs AFTER merge+defaults: a child inheriting a valid parent
	// but declaring an unsupported context still fails.
	resolveErr(t, "unsupported context",
		baseSession("parent"),
		Recipe{Name: "child", Kind: KindSession, Extends: "parent", Context: "bogus"},
	)
}

func TestResolveDoesNotMutateParentTabs(t *testing.T) {
	// Deep-value guarantee: resolving a child that replaces a shared tab name
	// must not mutate the parent's resolved Tabs backing array.
	defs := makeDefs([]Recipe{
		{Name: "parent", Kind: KindSession, Tabs: []TabSpec{
			{Name: "A", Command: "a1"}, {Name: "B", Command: "b1"},
		}},
		{Name: "child", Kind: KindSession, Extends: "parent", Tabs: []TabSpec{
			{Name: "B", Command: "MUTATED"},
		}},
	})
	if err := resolveDefinitions(defs); err != nil {
		t.Fatalf("resolveDefinitions: %v", err)
	}
	var parent, child Recipe
	for _, d := range defs {
		switch d.Recipe.Name {
		case "parent":
			parent = d.Recipe
		case "child":
			child = d.Recipe
		}
	}
	if parent.Tabs[1].Command != "b1" {
		t.Fatalf("parent tab B mutated by child merge: %q", parent.Tabs[1].Command)
	}
	if child.Tabs[1].Command != "MUTATED" {
		t.Fatalf("child tab B override not applied: %q", child.Tabs[1].Command)
	}
}

func TestResolveDoesNotMutateParentSessionTabs(t *testing.T) {
	// Same deep-value guarantee one level down: nested session tab merge must
	// not reach back into the parent's session tab slice.
	defs := makeDefs([]Recipe{
		{Name: "parent", Kind: KindWorkspace, Sessions: []SessionSpec{
			{Name: "web", Tabs: []TabSpec{{Name: "A", Command: "a1"}}},
		}},
		{Name: "child", Kind: KindWorkspace, Extends: "parent", Sessions: []SessionSpec{
			{Name: "web", Tabs: []TabSpec{{Name: "A", Command: "MUTATED"}, {Name: "B", Command: "b1"}}},
		}},
	})
	if err := resolveDefinitions(defs); err != nil {
		t.Fatalf("resolveDefinitions: %v", err)
	}
	for _, d := range defs {
		if d.Recipe.Name == "parent" {
			if got := d.Recipe.Sessions[0].Tabs[0].Command; got != "a1" {
				t.Fatalf("parent session tab mutated: %q", got)
			}
		}
	}
}
