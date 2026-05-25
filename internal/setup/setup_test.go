package setup

import (
	"os"
	"strings"
	"testing"
)

// memFS is an in-memory config.FS for verifying Apply behavior.
type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS { return &memFS{files: map[string][]byte{}} }

func (m *memFS) ReadFile(path string) ([]byte, error) {
	b, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return b, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = append([]byte(nil), data...)
	return nil
}
func (m *memFS) MkdirAll(string, os.FileMode) error { return nil }
func (m *memFS) Stat(string) (os.FileInfo, error)   { return nil, os.ErrNotExist }
func (m *memFS) UserHomeDir() (string, error)       { return "/home/u", nil }
func (m *memFS) Glob(string) ([]string, error)      { return nil, nil }

func TestPlanShellIntegration_UnsupportedShell(t *testing.T) {
	if _, ok := PlanShellIntegration(ShellInput{Shell: Unknown, Home: "/home/u"}); ok {
		t.Fatal("expected unsupported shell to yield ok=false")
	}
}

func TestPlanShellIntegration_ZshPath(t *testing.T) {
	p, ok := PlanShellIntegration(ShellInput{Shell: Zsh, Home: "/home/u"})
	if !ok || len(p.Edits) != 1 {
		t.Fatalf("expected one edit, got ok=%v edits=%d", ok, len(p.Edits))
	}
	if p.Edits[0].File != "/home/u/.zshrc" {
		t.Errorf("unexpected rc path: %s", p.Edits[0].File)
	}
}

func TestApply_AddIsIdempotent(t *testing.T) {
	fs := newMemFS()
	fs.files["/home/u/.zshrc"] = []byte("export PATH=/usr/bin\n")
	plan, _ := PlanShellIntegration(ShellInput{Shell: Zsh, Home: "/home/u"})

	// First apply: adds the block.
	res, err := plan.Apply(fs, ApplyOptions{Backup: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Changed || res[0].Note != "added" {
		t.Fatalf("expected added, got %+v", res[0])
	}
	got := string(fs.files["/home/u/.zshrc"])
	if !strings.Contains(got, markerBegin) || !strings.Contains(got, "zmux") {
		t.Fatalf("block not written: %q", got)
	}
	if !strings.HasPrefix(got, "export PATH=/usr/bin\n") {
		t.Errorf("existing content not preserved: %q", got)
	}
	// Backup written.
	if string(fs.files["/home/u/.zshrc.bak"]) != "export PATH=/usr/bin\n" {
		t.Errorf("backup not written correctly")
	}

	// Second apply: no change (idempotent).
	res2, _ := plan.Apply(fs, ApplyOptions{Backup: true})
	if res2[0].Changed || res2[0].Note != "no change" {
		t.Fatalf("expected idempotent no-op, got %+v", res2[0])
	}
}

func TestApply_Remove(t *testing.T) {
	fs := newMemFS()
	add, _ := PlanShellIntegration(ShellInput{Shell: Zsh, Home: "/home/u"})
	fs.files["/home/u/.zshrc"] = []byte("line1\n")
	if _, err := add.Apply(fs, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	rm, _ := PlanShellIntegration(ShellInput{Shell: Zsh, Home: "/home/u", Remove: true})
	res, err := rm.Apply(fs, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Changed || res[0].Note != "removed" {
		t.Fatalf("expected removed, got %+v", res[0])
	}
	got := string(fs.files["/home/u/.zshrc"])
	if strings.Contains(got, markerBegin) || strings.Contains(got, "zmux") {
		t.Fatalf("block not removed: %q", got)
	}
	if !strings.Contains(got, "line1") {
		t.Errorf("user content lost on removal: %q", got)
	}
}

func TestApply_DryRunDoesNotWrite(t *testing.T) {
	fs := newMemFS()
	fs.files["/home/u/.zshrc"] = []byte("orig\n")
	plan, _ := PlanShellIntegration(ShellInput{Shell: Zsh, Home: "/home/u"})

	res, err := plan.Apply(fs, ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0].Changed || res[0].Note != "would add" {
		t.Fatalf("expected 'would add', got %+v", res[0])
	}
	if string(fs.files["/home/u/.zshrc"]) != "orig\n" {
		t.Errorf("dry-run modified the file: %q", fs.files["/home/u/.zshrc"])
	}
	if _, ok := fs.files["/home/u/.zshrc.bak"]; ok {
		t.Error("dry-run wrote a backup")
	}
}

func TestUpsertBlock_ReplaceInPlace(t *testing.T) {
	original := upsertBlock("head\n", "OLD")
	updated := upsertBlock(original, "NEW")
	if strings.Contains(updated, "OLD") {
		t.Errorf("old block not replaced: %q", updated)
	}
	if !strings.Contains(updated, "NEW") {
		t.Errorf("new block missing: %q", updated)
	}
	if strings.Count(updated, markerBegin) != 1 {
		t.Errorf("expected exactly one managed block, got: %q", updated)
	}
}
