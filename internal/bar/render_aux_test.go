package bar

import (
	"strings"
	"testing"
)

// P2 row-ownership (plan 024): in two-line mode (TopRowActive) the top row owns
// workspace/session identity, so RenderLeft drops identity chrome and renders
// only the compact per-preset aux. The viewport letter moves to the top row's
// current-session entry and disappears from the bottom-left.

// auxCtx is a fully-populated, collision-free context for the aux tests: every
// identity/aux token is a unique string so "contains" assertions can't alias.
func auxCtx() BarContext {
	c := baseCtx()
	c.Session = "sess-uniq"
	c.Workspace = "ws-uniq"
	c.PaneDir = "/x/dir-uniq" // not under $HOME → shortenDir keeps it verbatim
	c.PaneCmd = "proc-uniq"
	c.GitBranch = "git-uniq"
	c.TopRowActive = true
	return c
}

// TestRenderLeftAux_FieldPolicy locks the per-preset bottom-left field policy in
// two-line mode: identity (session/workspace) is always dropped; cwd never
// appears because it lives in the top overlay; aux follows hacker→process,
// starship→git, others→empty.
func TestRenderLeftAux_FieldPolicy(t *testing.T) {
	p := testPalette()
	for _, preset := range AllPresets() {
		visible := stripANSI(RenderLeft(p, auxCtx(), preset))

		// Identity never leaks to the bottom-left when the top row owns it.
		if strings.Contains(visible, "sess-uniq") {
			t.Errorf("%s: TopRowActive bottom-left leaked session name: %q", preset, visible)
		}
		if strings.Contains(visible, "ws-uniq") {
			t.Errorf("%s: TopRowActive bottom-left leaked workspace name: %q", preset, visible)
		}

		hasDir := strings.Contains(visible, "dir-uniq")
		hasProc := strings.Contains(visible, "proc-uniq")
		hasGit := strings.Contains(visible, "git-uniq")

		switch preset {
		case Hacker:
			if hasDir || !hasProc || hasGit {
				t.Errorf("%s: want process only, got dir=%v proc=%v git=%v: %q", preset, hasDir, hasProc, hasGit, visible)
			}
		case Starship:
			if hasDir || !hasGit || hasProc {
				t.Errorf("%s: want git only, got dir=%v proc=%v git=%v: %q", preset, hasDir, hasProc, hasGit, visible)
			}
		default:
			if strings.TrimSpace(visible) != "" {
				t.Errorf("%s: want empty bottom-left, got %q", preset, visible)
			}
		}
	}
}

// TestRenderLeftAux_RespectsSegmentToggles confirms the aux path still honors
// segment visibility — RenderLeft runs applySegmentVisibility before the
// short-circuit, so disabling process/git empties their aux. Directory is
// already absent from this row.
func TestRenderLeftAux_RespectsSegmentToggles(t *testing.T) {
	p := testPalette()
	ctx := auxCtx()
	ctx.ShowProcess = false
	ctx.ShowGit = false
	for _, preset := range AllPresets() {
		visible := stripANSI(RenderLeft(p, ctx, preset))
		if strings.TrimSpace(visible) != "" {
			t.Errorf("%s: all aux segments disabled should empty bottom-left, got %q", preset, visible)
		}
	}
}

func TestRenderTopOverlay_DirectoryAndProfileShareRightAlignment(t *testing.T) {
	p := testPalette()
	ctx := auxCtx()
	got := RenderTopOverlay(p, ctx, "zzmux")

	if !strings.HasPrefix(got, "#[align=right]") {
		t.Fatalf("top overlay must own one right alignment, got %q", got)
	}
	if strings.Count(got, "#[align=right]") != 1 {
		t.Fatalf("top overlay should contain one align directive, got %q", got)
	}
	if !strings.Contains(got, "dir-uniq") {
		t.Fatalf("top overlay should include cwd, got %q", got)
	}
	if !strings.Contains(got, "zzmux") {
		t.Fatalf("top overlay should include non-default profile badge, got %q", got)
	}
}

func TestRenderTopOverlay_RespectsDirectoryToggleButKeepsProfile(t *testing.T) {
	p := testPalette()
	ctx := auxCtx()
	ctx.ShowDirectory = false
	got := RenderTopOverlay(p, ctx, "zzmux")

	if strings.Contains(got, "dir-uniq") {
		t.Fatalf("ShowDirectory=false should hide cwd overlay, got %q", got)
	}
	if !strings.Contains(got, "zzmux") {
		t.Fatalf("profile badge should still render when directory is hidden, got %q", got)
	}
}

func TestRenderTopOverlay_DefaultProfileShowsDirectoryOnly(t *testing.T) {
	p := testPalette()
	ctx := auxCtx()
	got := RenderTopOverlay(p, ctx, "zmux")

	if !strings.Contains(got, "dir-uniq") {
		t.Fatalf("default profile should still show cwd overlay, got %q", got)
	}
	if strings.Contains(got, "zzmux") || strings.Contains(got, " zmux ") {
		t.Fatalf("default profile should not render an edge badge, got %q", got)
	}
}

// TestRenderTop_ViewportOnCurrentSessionOnly: a grouped clone shows its viewport
// letter exactly once on the top row — on the current session's entry — and the
// bottom-left aux never carries it.
func TestRenderTop_ViewportOnCurrentSessionOnly(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.Session = "main"
	ctx.WorkspaceSessions = []string{"main", "dev"}
	ctx.ViewportID = "b"
	ctx.ShowGroup = true

	for _, preset := range AllPresets() {
		top := stripANSI(RenderTop(p, ctx, preset))
		if n := strings.Count(top, "·b"); n != 1 {
			t.Errorf("%s: viewport suffix should appear once on top row, got %d: %q", preset, n, top)
		}
		if strings.Contains(top, "dev·b") {
			t.Errorf("%s: viewport leaked onto a non-current session: %q", preset, top)
		}

		// Same grouped context, bottom-left in two-line mode: no viewport.
		left := stripANSI(RenderLeft(p, withTopRow(ctx), preset))
		if strings.Contains(left, "·b") {
			t.Errorf("%s: viewport must not appear on the bottom-left, got %q", preset, left)
		}
	}
}

// TestRenderTop_ViewportHiddenWhenGroupDisabled: ShowGroup=false suppresses the
// viewport on the top row (applySegmentVisibility does not run there, so
// topSessionLabel gates on ShowGroup directly).
func TestRenderTop_ViewportHiddenWhenGroupDisabled(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.Session = "main"
	ctx.WorkspaceSessions = []string{"main", "dev"}
	ctx.ViewportID = "b"
	ctx.ShowGroup = false

	for _, preset := range AllPresets() {
		top := stripANSI(RenderTop(p, ctx, preset))
		if strings.Contains(top, "·b") {
			t.Errorf("%s: ShowGroup=false should hide the viewport suffix, got %q", preset, top)
		}
	}
}

func withTopRow(ctx BarContext) BarContext {
	ctx.TopRowActive = true
	return ctx
}
