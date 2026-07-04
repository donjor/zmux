package setup

import (
	"os"
	"strings"
	"testing"
	"time"
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
func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return fakeFileInfo{}, nil
	}
	return nil, os.ErrNotExist
}
func (m *memFS) UserHomeDir() (string, error)  { return "/home/u", nil }
func (m *memFS) Glob(string) ([]string, error) { return nil, nil }

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func TestPlanShellIntegration_UnsupportedShell(t *testing.T) {
	if _, ok := PlanShellIntegration(ShellInput{Shell: Unknown, Home: "/home/u"}); ok {
		t.Fatal("expected unsupported shell to yield ok=false")
	}
}

func TestPlanShellIntegration_BashIncludesLoginBridge(t *testing.T) {
	p, ok := PlanShellIntegration(ShellInput{Shell: Bash, Home: "/home/u", BashProfile: "/home/u/.profile"})
	if !ok || len(p.Edits) != 2 {
		t.Fatalf("expected bash rc + login bridge edits, got ok=%v edits=%d", ok, len(p.Edits))
	}
	if p.Edits[0].File != "/home/u/.bashrc" || p.Edits[1].File != "/home/u/.profile" {
		t.Fatalf("unexpected bash files: %#v", p.Edits)
	}
	if !strings.Contains(p.Edits[1].Block, ". \"$HOME/.bashrc\"") {
		t.Fatalf("bash login bridge must source .bashrc: %s", p.Edits[1].Block)
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

func TestPlanShellIntegration_IncludesLifecycleHooks(t *testing.T) {
	cases := []struct {
		shell Shell
		want  []string
	}{
		{Bash, []string{"shell-event start", "shell-event end", "PROMPT_COMMAND", "DEBUG", "ble-attach", "blehook PREEXEC", "blehook PRECMD", "${1:-$BASH_COMMAND}", "ZMUX_SHELL_ROOT", "${TMUX%%,*}", "basename", "__zmux_prompt_ready"}},
		{Zsh, []string{"preexec_functions=(__zmux_preexec", "precmd_functions=(__zmux_precmd", "shell-event start", "shell-event end", "ZMUX_SHELL_ROOT", "${TMUX%%,*}", "basename"}},
		{Fish, []string{"fish_preexec", "fish_postexec", "shell-event start", "shell-event end", "set -l __zmux_ec $status", "ZMUX_SHELL_ROOT", "string split -m1", "basename"}},
	}
	for _, tc := range cases {
		plan, ok := PlanShellIntegration(ShellInput{Shell: tc.shell, Home: "/home/u", Bin: "zzmux"})
		if !ok || len(plan.Edits) == 0 {
			t.Fatalf("%s: expected edits", tc.shell)
		}
		block := plan.Edits[0].Block
		if !strings.Contains(block, "zzmux") {
			t.Fatalf("%s: profile binary missing from block: %s", tc.shell, block)
		}
		for _, want := range tc.want {
			if !strings.Contains(block, want) {
				t.Fatalf("%s: missing %q in block:\n%s", tc.shell, want, block)
			}
		}
	}
}
