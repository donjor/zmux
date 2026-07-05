package setup

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestPlanShellIntegration_BashBleLoginBridgeIsIdempotent(t *testing.T) {
	plan, ok := PlanShellIntegration(ShellInput{Shell: Bash, Home: "/home/u", Bin: "zzmux", BashProfile: "/home/u/.profile"})
	if !ok || len(plan.Edits) != 2 {
		t.Fatalf("expected bash rc + profile edits, got ok=%v edits=%d", ok, len(plan.Edits))
	}

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(plan.Edits[0].Block), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := "if [ -f \"$HOME/.bashrc\" ]; then\n  . \"$HOME/.bashrc\"\nfi\n" + plan.Edits[1].Block + "\n"
	if err := os.WriteFile(filepath.Join(home, ".profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(home, "simulate-login.sh")
	if err := os.WriteFile(script, []byte(`
ble_attach_calls=0
blehook_calls=0
ble-attach() { ble_attach_calls=$((ble_attach_calls + 1)); }
blehook() { blehook_calls=$((blehook_calls + 1)); }
tmux() { printf '%%99\n'; }
zzmux() { :; }
. "$HOME/.profile"
printf 'ble_attach_calls=%s\n' "$ble_attach_calls"
printf 'blehook_calls=%s\n' "$blehook_calls"
printf 'debug_trap=%s\n' "$(trap -p DEBUG)"
printf 'prompt_command=%s\n' "${PROMPT_COMMAND-}"
printf 'integration_version=%s\n' "${ZMUX_SHELL_INTEGRATION_VERSION-}"
`), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "--noprofile", "--norc", "-i", script)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"TMUX=/tmp/zzmux,1,0",
		"TMUX_PANE=%99",
		"ZMUX_BIN=zzmux",
		"BLE_VERSION=stub",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("simulated bash login failed: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "ble_attach_calls=0\n") {
		t.Fatalf("bash lifecycle setup must not force ble-attach; output:\n%s", text)
	}
	if !strings.Contains(text, "blehook_calls=4\n") {
		t.Fatalf("bash lifecycle hooks should be registered exactly once; output:\n%s", text)
	}
	if strings.Contains(text, "__zmux_preexec") && strings.Contains(text, "debug_trap=trap") {
		t.Fatalf("blehook path must not leave a DEBUG trap installed; output:\n%s", text)
	}
	if !strings.Contains(text, "integration_version="+ShellIntegrationVersion+"\n") {
		t.Fatalf("bash lifecycle setup must export integration version; output:\n%s", text)
	}
}

func TestPlanShellIntegration_BashHasVersionAndDoesNotForceBleAttach(t *testing.T) {
	plan, ok := PlanShellIntegration(ShellInput{Shell: Bash, Home: "/home/u", Bin: "zzmux"})
	if !ok || len(plan.Edits) == 0 {
		t.Fatalf("expected bash integration plan")
	}
	if !strings.Contains(plan.Edits[0].Block, "ZMUX_SHELL_INTEGRATION_VERSION='"+ShellIntegrationVersion+"'") {
		t.Fatalf("bash lifecycle block must embed integration version %s:\n%s", ShellIntegrationVersion, plan.Edits[0].Block)
	}
	if strings.Contains(plan.Edits[0].Block, "ble-attach") {
		t.Fatalf("bash lifecycle block must not force ble-attach; it breaks interactive TUIs in ble.sh shells:\n%s", plan.Edits[0].Block)
	}
}

func TestDevShZzmuxSkipsShellSetupByDefault(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "dev.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	want := `if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_SHELL_SETUP:-0}" != "1" ]; then`
	if !strings.Contains(script, want) {
		t.Fatalf("dev.sh must not rewrite live shell integration when TARGET=zzmux by default; expected guarded condition %q", want)
	}
}

func TestPlanShellIntegration_IncludesLifecycleHooks(t *testing.T) {
	cases := []struct {
		shell Shell
		want  []string
	}{
		{Bash, []string{"shell-event start", "shell-event end", "PROMPT_COMMAND", "DEBUG", "blehook PREEXEC", "blehook PRECMD", "${1:-$BASH_COMMAND}", "ZMUX_SHELL_ROOT", "ZMUX_SHELL_INTEGRATION_VERSION", "${TMUX%%,*}", "basename", "__zmux_prompt_ready"}},
		{Zsh, []string{"preexec_functions=(__zmux_preexec", "precmd_functions=(__zmux_precmd", "shell-event start", "shell-event end", "ZMUX_SHELL_ROOT", "ZMUX_SHELL_INTEGRATION_VERSION", "${TMUX%%,*}", "basename"}},
		{Fish, []string{"fish_preexec", "fish_postexec", "shell-event start", "shell-event end", "set -l __zmux_ec $status", "ZMUX_SHELL_ROOT", "ZMUX_SHELL_INTEGRATION_VERSION", "string split -m1", "basename"}},
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
