//go:build integration

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// zmuxBin returns the path to the built zmux binary.
// It expects the binary to be at the project root (built via `make build`).
func zmuxBin(t *testing.T) string {
	t.Helper()

	// Walk up from tests/ to find the project root.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	root := filepath.Dir(filepath.Dir(thisFile))
	bin := filepath.Join(root, "zmux")

	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("zmux binary not found at %s — run 'make build' first", bin)
	}
	return bin
}

// runZmux executes the zmux binary with the given args and returns stdout, stderr, and any error.
func runZmux(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	bin := zmuxBin(t)
	cmd := exec.Command(bin, args...)
	// Ensure we are not inside a tmux session for deterministic output.
	cmd.Env = append(os.Environ(), "TMUX=")
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func TestVersionCommand(t *testing.T) {
	stdout, _, err := runZmux(t, "version")
	if err != nil {
		t.Fatalf("zmux version failed: %v", err)
	}

	if !strings.HasPrefix(stdout, "zmux ") {
		t.Errorf("expected output to start with 'zmux ', got: %q", stdout)
	}

	// Should contain OS/arch info.
	if !strings.Contains(stdout, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("expected output to contain %s/%s, got: %q", runtime.GOOS, runtime.GOARCH, stdout)
	}
}

func TestThemeListCommand(t *testing.T) {
	stdout, _, err := runZmux(t, "theme", "list")
	if err != nil {
		t.Fatalf("zmux theme list failed: %v", err)
	}

	// Should list at least the bundled themes.
	bundled := []string{"ayu-dark", "tokyonight", "catppuccin-mocha", "nord", "dracula"}
	for _, name := range bundled {
		if !strings.Contains(stdout, name) {
			t.Errorf("expected theme list to contain %q, got:\n%s", name, stdout)
		}
	}

	// Should have source tags.
	if !strings.Contains(stdout, "bundled") {
		t.Errorf("expected theme list to show 'bundled' source tag, got:\n%s", stdout)
	}
}

func TestStatusCommand(t *testing.T) {
	stdout, _, err := runZmux(t, "status")
	if err != nil {
		t.Fatalf("zmux status failed: %v", err)
	}

	// Should contain key status labels.
	expectedLabels := []string{"Theme", "Bar preset", "Prefix", "Config"}
	for _, label := range expectedLabels {
		if !strings.Contains(stdout, label) {
			t.Errorf("expected status output to contain %q, got:\n%s", label, stdout)
		}
	}
}

func TestCompletionBash(t *testing.T) {
	stdout, _, err := runZmux(t, "completion", "bash")
	if err != nil {
		t.Fatalf("zmux completion bash failed: %v", err)
	}

	// Bash completions should define a completion function.
	if !strings.Contains(stdout, "bash") && !strings.Contains(stdout, "complete") && !strings.Contains(stdout, "_zmux") {
		t.Errorf("expected valid bash completion output, got:\n%.200s...", stdout)
	}

	if len(stdout) < 100 {
		t.Errorf("bash completion output suspiciously short (%d bytes)", len(stdout))
	}
}

func TestCompletionZsh(t *testing.T) {
	stdout, _, err := runZmux(t, "completion", "zsh")
	if err != nil {
		t.Fatalf("zmux completion zsh failed: %v", err)
	}

	if len(stdout) < 100 {
		t.Errorf("zsh completion output suspiciously short (%d bytes)", len(stdout))
	}
}

func TestCompletionFish(t *testing.T) {
	stdout, _, err := runZmux(t, "completion", "fish")
	if err != nil {
		t.Fatalf("zmux completion fish failed: %v", err)
	}

	if !strings.Contains(stdout, "complete") {
		t.Errorf("expected fish completion output to contain 'complete', got:\n%.200s...", stdout)
	}
}

func TestHelpCommand(t *testing.T) {
	stdout, _, err := runZmux(t, "help")
	if err != nil {
		t.Fatalf("zmux help failed: %v", err)
	}

	// Should contain the main help sections. Kept in sync with the
	// section headers rendered by cmd/zmux/help.go.
	sections := []string{
		"Session Management",
		"Terminal Commands",
		"Theming",
		"Configuration",
		"Other",
	}
	for _, section := range sections {
		if !strings.Contains(stdout, section) {
			t.Errorf("expected help output to contain section %q, got:\n%s", section, stdout)
		}
	}

	// Should contain keybindings section.
	if !strings.Contains(stdout, "Keybindings") {
		t.Errorf("expected help output to contain keybindings section")
	}
}

func TestHelpExitCode(t *testing.T) {
	_, _, err := runZmux(t, "help")
	if err != nil {
		t.Errorf("zmux help should exit 0, got: %v", err)
	}
}

func TestVersionExitCode(t *testing.T) {
	_, _, err := runZmux(t, "version")
	if err != nil {
		t.Errorf("zmux version should exit 0, got: %v", err)
	}
}

func TestUnknownCommandExitsNonZero(t *testing.T) {
	_, stderr, err := runZmux(t, "nonexistent-command-xyz")
	if err == nil {
		t.Error("expected non-zero exit for unknown command")
	}
	// Should show an error message.
	if stderr == "" {
		// Some cobra versions write to stdout instead.
		t.Log("stderr was empty; error may be on stdout")
	}
}

func TestCompletionInvalidShellExitsNonZero(t *testing.T) {
	_, _, err := runZmux(t, "completion", "powershell")
	if err == nil {
		t.Error("expected non-zero exit for invalid shell")
	}
}

func TestTestdataFixtures(t *testing.T) {
	// Verify test fixtures exist and are valid.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	testdataDir := filepath.Join(filepath.Dir(thisFile), "testdata")

	// Check test.toml exists.
	configPath := filepath.Join(testdataDir, "test.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("cannot read test.toml: %v", err)
	}
	if !strings.Contains(string(data), "theme") {
		t.Error("test.toml should contain a theme key")
	}

	// Check test-theme exists.
	themePath := filepath.Join(testdataDir, "test-theme")
	data, err = os.ReadFile(themePath)
	if err != nil {
		t.Fatalf("cannot read test-theme: %v", err)
	}
	if !strings.Contains(string(data), "background") {
		t.Error("test-theme should contain a background key")
	}
	if !strings.Contains(string(data), "palette") {
		t.Error("test-theme should contain palette entries")
	}
}

func TestBarListCommand(t *testing.T) {
	stdout, _, err := runZmux(t, "bar")
	if err != nil {
		t.Fatalf("zmux bar failed: %v", err)
	}

	presets := []string{"default", "minimal", "powerline", "blocks"}
	for _, preset := range presets {
		if !strings.Contains(stdout, preset) {
			t.Errorf("expected bar list to contain preset %q, got:\n%s", preset, stdout)
		}
	}
}
