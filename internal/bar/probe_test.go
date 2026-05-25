package bar

import "testing"

// fakeProber is a deterministic Prober for testing GatherContext without
// shelling out.
type fakeProber struct {
	branch        string
	dirty         bool
	ahead, behind int
	langIcon      string
	langVer       string
}

func (f fakeProber) GitBranch(string) string            { return f.branch }
func (f fakeProber) GitDirty(string) bool               { return f.dirty }
func (f fakeProber) GitAheadBehind(string) (int, int)   { return f.ahead, f.behind }
func (f fakeProber) DetectLang(string) (string, string) { return f.langIcon, f.langVer }

func TestGatherContext_UsesProber(t *testing.T) {
	p := fakeProber{branch: "main", dirty: true, ahead: 2, behind: 1, langIcon: "go", langVer: "1.24"}
	ctx := GatherContext(p, "sess", "/some/dir", "nvim", "0", "", "ws")

	if ctx.GitBranch != "main" || !ctx.GitDirty || ctx.GitAhead != 2 || ctx.GitBehind != 1 {
		t.Errorf("git fields not mapped from prober: %+v", ctx)
	}
	if ctx.LangIcon != "go" || ctx.LangVersion != "1.24" {
		t.Errorf("lang fields not mapped from prober: %+v", ctx)
	}
	if ctx.Session != "sess" || ctx.Workspace != "ws" {
		t.Errorf("static fields wrong: %+v", ctx)
	}
}

func TestGatherContext_EmptyDirSkipsProbe(t *testing.T) {
	// With no pane dir, the prober must not be consulted for git/lang.
	p := fakeProber{branch: "should-not-appear"}
	ctx := GatherContext(p, "sess", "", "bash", "0", "", "ws")
	if ctx.GitBranch != "" {
		t.Errorf("expected no git branch for empty dir, got %q", ctx.GitBranch)
	}
}

func TestGatherContext_NoUpstreamWhenNoBranch(t *testing.T) {
	// When there is no branch, dirty/ahead/behind must stay zero-valued.
	p := fakeProber{branch: "", dirty: true, ahead: 5}
	ctx := GatherContext(p, "sess", "/dir", "bash", "0", "", "ws")
	if ctx.GitDirty || ctx.GitAhead != 0 {
		t.Errorf("dirty/ahead should be skipped without a branch: %+v", ctx)
	}
}
