package qa

import (
	"strings"
	"testing"
)

func TestFindRepoRoot(t *testing.T) {
	fs := newMemFS()
	fs.files["/repo/.git"] = []byte{}

	for _, dir := range []string{"/repo", "/repo/internal/qa"} {
		got, err := FindRepoRoot(fs, dir)
		if err != nil {
			t.Fatalf("FindRepoRoot(%s): %v", dir, err)
		}
		if got != "/repo" {
			t.Errorf("FindRepoRoot(%s) = %q", dir, got)
		}
	}

	if _, err := FindRepoRoot(fs, "/elsewhere"); err == nil {
		t.Error("no .git above: want error")
	}
}

func TestDiscover(t *testing.T) {
	fs := newMemFS()
	fs.files["/repo/checklists/zeta.toml"] = []byte{}
	fs.files["/repo/checklists/alpha.toml"] = []byte{}
	fs.files["/repo/checklists/notes.md"] = []byte{}

	refs, err := Discover(fs, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Stem != "alpha" || refs[1].Stem != "zeta" {
		t.Errorf("Discover = %v, want sorted [alpha zeta]", refs)
	}
}

func TestLintRefs(t *testing.T) {
	if issues := LintRefs([]Ref{{Path: "a/x.toml", Stem: "x"}, {Path: "b/y.toml", Stem: "y"}}); len(issues) > 0 {
		t.Errorf("unique stems: %v", issues)
	}
	issues := LintRefs([]Ref{{Path: "a/x.toml", Stem: "x"}, {Path: "b/x.toml", Stem: "x"}})
	if len(issues) != 1 || !strings.Contains(issues[0], "duplicate checklist stem") {
		t.Errorf("dup stems: %v", issues)
	}
}

func TestResolve(t *testing.T) {
	fs := newMemFS()
	fs.files["/repo/checklists/alpha.toml"] = []byte{}
	fs.files["/repo/checklists/beta.toml"] = []byte{}

	t.Run("stem hit", func(t *testing.T) {
		got, err := Resolve(fs, "/repo", "alpha")
		if err != nil || got != "/repo/checklists/alpha.toml" {
			t.Errorf("got %q, %v", got, err)
		}
	})

	t.Run("explicit path", func(t *testing.T) {
		got, err := Resolve(fs, "/repo", "/repo/checklists/beta.toml")
		if err != nil || got != "/repo/checklists/beta.toml" {
			t.Errorf("got %q, %v", got, err)
		}
	})

	t.Run("explicit path missing", func(t *testing.T) {
		if _, err := Resolve(fs, "/repo", "/repo/checklists/ghost.toml"); err == nil {
			t.Error("want error")
		}
	})

	t.Run("stem miss lists stems", func(t *testing.T) {
		_, err := Resolve(fs, "/repo", "gamma")
		if err == nil || !strings.Contains(err.Error(), "alpha") {
			t.Errorf("want miss listing available stems, got %v", err)
		}
	})

	t.Run("no checklists", func(t *testing.T) {
		_, err := Resolve(newMemFS(), "/repo", "alpha")
		if err == nil || !strings.Contains(err.Error(), "no checklists") {
			t.Errorf("got %v", err)
		}
	})
}
