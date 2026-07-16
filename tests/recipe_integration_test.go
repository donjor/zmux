//go:build integration

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// T-403 (055 P-004, S-011) — non-tmux recipe integration slice. These drive the
// built ./zmux binary against an isolated HOME (its own ~/.zmux/recipes fixture)
// with TMUX unset, so recipe list/show and `run --dry-run` planning exercise the
// real command tree with zero tmux daemon and zero developer state. tmux-verb
// coverage is deliberately out of scope here (post-release qa-harness).

const fixtureRecipe = `name = "fixture"
description = "T-403 non-tmux integration fixture"
context = "outside"
kind = "session"
session = "fixture-main"

[[tabs]]
name = "shell"
command = "echo hello"
`

// isolatedHome returns a throwaway HOME seeded with a single user recipe under
// ~/.zmux/recipes, keeping recipe discovery, config, and state off the real
// profile. t.TempDir is cleaned up automatically.
func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	recipesDir := filepath.Join(home, ".zmux", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatalf("mkdir recipes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "fixture.toml"), []byte(fixtureRecipe), 0o644); err != nil {
		t.Fatalf("write fixture recipe: %v", err)
	}
	return home
}

// runZmuxHome runs the binary with HOME and TMUX forced (not merely appended),
// so the isolated profile wins regardless of the host env ordering.
func runZmuxHome(t *testing.T, home string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	bin := zmuxBin(t)
	cmd := exec.Command(bin, args...)
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "TMUX=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, "TMUX=", "HOME="+home)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func TestRecipeListShowsUserFixture(t *testing.T) {
	home := isolatedHome(t)
	stdout, stderr, err := runZmuxHome(t, home, "recipe", "list")
	if err != nil {
		t.Fatalf("recipe list failed: %v\nstderr: %s", err, stderr)
	}
	// The user fixture is listed with its source tag and description; the
	// bundled recipes are still discovered alongside it.
	for _, want := range []string{"fixture", "user", "T-403 non-tmux integration fixture", "dev"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("recipe list missing %q, got:\n%s", want, stdout)
		}
	}
}

func TestRecipeShowUserFixture(t *testing.T) {
	home := isolatedHome(t)
	stdout, stderr, err := runZmuxHome(t, home, "recipe", "show", "fixture")
	if err != nil {
		t.Fatalf("recipe show failed: %v\nstderr: %s", err, stderr)
	}
	for _, want := range []string{`name = "fixture"`, `command = "echo hello"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("recipe show missing %q, got:\n%s", want, stdout)
		}
	}
}

func TestRunRecipeDryRunPlansWithoutTmux(t *testing.T) {
	home := isolatedHome(t)
	stdout, stderr, err := runZmuxHome(t, home, "run", "fixture", "--dry-run")
	if err != nil {
		t.Fatalf("run --dry-run failed: %v\nstderr: %s", err, stderr)
	}
	// Stable, environment-independent plan fragments (cwd-derived workspace
	// name is intentionally not asserted).
	for _, want := range []string{
		"Recipe: fixture",
		"create-session fixture-main",
		"send-command fixture-main:shell",
		"echo hello",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("dry-run plan missing %q, got:\n%s", want, stdout)
		}
	}
}

func TestUnknownRecipeShowFailsWithDiagnostic(t *testing.T) {
	home := isolatedHome(t)
	stdout, stderr, err := runZmuxHome(t, home, "recipe", "show", "nope-xyz")
	if err == nil {
		t.Fatal("expected non-zero exit for unknown recipe")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, `recipe "nope-xyz" not found`) {
		t.Errorf("expected stable not-found diagnostic, got:\n%s", combined)
	}
	if !strings.Contains(combined, "available:") {
		t.Errorf("expected diagnostic to list available recipes, got:\n%s", combined)
	}
}
