package bar

import (
	"strings"
	"testing"
)

// Characterization tests for the public render API.
//
// Goal: pin the visible behavior of RenderLeft/RenderRight/RenderTop and the
// helper methods on BarContext, so the file can be split by preset without
// silent regressions.
//
// Preview-level coverage (preview_test.go) already exercises every preset's
// renderer through RenderPreview. These tests add direct coverage of the
// public render entry points and BarContext semantics.

func baseCtx() BarContext {
	return BarContext{
		Session:        "main",
		Workspace:      "zmux",
		WorkspacePos:   1,
		WorkspaceCount: 1,
		Time:           "14:30",
		Date:           "Apr 07",
		PaneDir:        "~/src/zmux",
		PaneCmd:        "nvim",
		ShowWorkspace:  true,
		ShowGit:        true,
		ShowLang:       true,
		ShowClock:      true,
		ShowDirectory:  true,
		ShowProcess:    true,
		ShowGroup:      true,
	}
}

// ── BarContext.SessionLabel ─────────────────────────────────────────────────

func TestSessionLabel_SingleSessionShowsNameOnly(t *testing.T) {
	ctx := baseCtx()
	ctx.WorkspaceCount = 1
	if got := ctx.SessionLabel(); got != "main" {
		t.Errorf("single session: want %q, got %q", "main", got)
	}
}

func TestSessionLabel_MultipleSessionsShowsPosition(t *testing.T) {
	ctx := baseCtx()
	ctx.WorkspacePos = 2
	ctx.WorkspaceCount = 3
	if got := ctx.SessionLabel(); got != "main 2/3" {
		t.Errorf("multi session: want %q, got %q", "main 2/3", got)
	}
}

func TestSessionLabel_IndicatorOverridesPosition(t *testing.T) {
	ctx := baseCtx()
	ctx.WorkspacePos = 2
	ctx.WorkspaceCount = 3
	ctx.SessionIndicator = "○●○"
	if got := ctx.SessionLabel(); got != "main ○●○" {
		t.Errorf("with indicator: want %q, got %q", "main ○●○", got)
	}
}

// ── BarContext.WorkspaceLabel ───────────────────────────────────────────────

func TestWorkspaceLabel_PlainName(t *testing.T) {
	ctx := baseCtx()
	if got := ctx.WorkspaceLabel(); got != "zmux" {
		t.Errorf("workspace label: want %q, got %q", "zmux", got)
	}
}

// ── applySegmentVisibility ──────────────────────────────────────────────────

func TestApplySegmentVisibility_DisabledClockClearsTimeAndDate(t *testing.T) {
	ctx := baseCtx()
	ctx.ShowClock = false
	applySegmentVisibility(&ctx)
	if ctx.Time != "" || ctx.Date != "" {
		t.Errorf("ShowClock=false should clear Time and Date, got Time=%q Date=%q", ctx.Time, ctx.Date)
	}
}

func TestApplySegmentVisibility_DisabledGitClearsAllGitFields(t *testing.T) {
	ctx := baseCtx()
	ctx.GitBranch = "main"
	ctx.GitDirty = true
	ctx.GitAhead = 2
	ctx.GitBehind = 1
	ctx.ShowGit = false
	applySegmentVisibility(&ctx)
	if ctx.GitBranch != "" || ctx.GitDirty || ctx.GitAhead != 0 || ctx.GitBehind != 0 {
		t.Errorf("ShowGit=false should clear all git fields, got branch=%q dirty=%v ahead=%d behind=%d",
			ctx.GitBranch, ctx.GitDirty, ctx.GitAhead, ctx.GitBehind)
	}
}

func TestApplySegmentVisibility_DisabledGroupClearsGroupFields(t *testing.T) {
	ctx := baseCtx()
	ctx.GroupID = "abc"
	ctx.Attached = 2
	ctx.ViewportID = "b"
	ctx.ShowGroup = false
	applySegmentVisibility(&ctx)
	if ctx.GroupID != "" || ctx.Attached != 0 || ctx.ViewportID != "" {
		t.Errorf("ShowGroup=false should clear group fields, got %+v", ctx)
	}
}

func TestApplySegmentVisibility_EnabledFieldsUnchanged(t *testing.T) {
	ctx := baseCtx()
	original := ctx
	applySegmentVisibility(&ctx)
	if ctx.Time != original.Time || ctx.Workspace != original.Workspace || ctx.PaneDir != original.PaneDir {
		t.Errorf("all segments enabled, should be unchanged. before=%+v after=%+v", original, ctx)
	}
}

// ── RenderLeft / RenderRight non-empty for every preset ─────────────────────

func TestRenderLeft_NonEmptyForEveryPreset(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	for _, preset := range AllPresets() {
		got := RenderLeft(p, ctx, preset)
		if strings.TrimSpace(stripANSI(got)) == "" {
			t.Errorf("preset %s: RenderLeft produced empty visible output", preset)
		}
	}
}

func TestRenderRight_NonEmptyForEveryPreset(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	for _, preset := range AllPresets() {
		got := RenderRight(p, ctx, preset)
		if strings.TrimSpace(stripANSI(got)) == "" {
			t.Errorf("preset %s: RenderRight produced empty visible output", preset)
		}
	}
}

// ── RenderTop renders for a single session (always-2-line, plan 024) ─────────

func TestRenderTop_NonEmptyWhenSingleSession(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.WorkspaceSessions = []string{"main"}
	for _, preset := range AllPresets() {
		got := RenderTop(p, ctx, preset)
		if got == "" {
			t.Errorf("preset %s: RenderTop should render a single-session top row (always-2-line), got empty", preset)
		}
	}
}

// Each top-row variant (tabs/dots/minimal) must degrade sanely to a single
// session — non-empty, no panic — under always-2-line (plan 024).
func TestRenderTopRow_SingleSessionAllVariants(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.WorkspaceSessions = []string{"main"}
	for _, variant := range []string{"tabs", "dots", "minimal"} {
		for _, preset := range AllPresets() {
			got := RenderTopRow(p, ctx, preset, variant)
			if got == "" {
				t.Errorf("variant %s preset %s: single-session top row is empty", variant, preset)
			}
		}
	}
}

func TestRenderTop_EmptyWhenNoSessions(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.WorkspaceSessions = nil
	for _, preset := range AllPresets() {
		got := RenderTop(p, ctx, preset)
		if got != "" {
			t.Errorf("preset %s: RenderTop should be empty for no sessions, got %q", preset, got)
		}
	}
}

func TestRenderTop_NonEmptyWhenMultipleSessions(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.WorkspaceSessions = []string{"main", "dev", "test"}
	for _, preset := range AllPresets() {
		got := RenderTop(p, ctx, preset)
		if strings.TrimSpace(stripANSI(got)) == "" {
			t.Errorf("preset %s: RenderTop should be non-empty for multi-session, got %q", preset, got)
		}
	}
}

// ── Segment visibility flows through render functions ───────────────────────

func TestRenderRight_ClockHiddenDoesNotShowTime(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.ShowClock = false
	for _, preset := range AllPresets() {
		visible := stripANSI(RenderRight(p, ctx, preset))
		if strings.Contains(visible, "14:30") {
			t.Errorf("preset %s: ShowClock=false but time 14:30 visible: %q", preset, visible)
		}
		if strings.Contains(visible, "Apr 07") {
			t.Errorf("preset %s: ShowClock=false but date 'Apr 07' visible: %q", preset, visible)
		}
	}
}

func TestRenderLeft_WorkspaceHiddenDoesNotShowWorkspaceName(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.ShowWorkspace = false
	// Use a workspace name that wouldn't accidentally appear from other segments.
	ctx.Workspace = "unique-workspace-name"
	for _, preset := range AllPresets() {
		visible := stripANSI(RenderLeft(p, ctx, preset))
		if strings.Contains(visible, "unique-workspace-name") {
			t.Errorf("preset %s: ShowWorkspace=false but workspace name visible: %q", preset, visible)
		}
	}
}

// ── Session name appears somewhere when set ─────────────────────────────────

func TestRenderLeft_SessionNamePresent(t *testing.T) {
	p := testPalette()
	ctx := baseCtx()
	ctx.Session = "very-unique-session"
	for _, preset := range AllPresets() {
		visible := stripANSI(RenderLeft(p, ctx, preset))
		if !strings.Contains(visible, "very-unique-session") {
			t.Errorf("preset %s: session name should appear in RenderLeft, got %q", preset, visible)
		}
	}
}
