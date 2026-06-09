package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/qa"
)

// memFS is an in-memory config.FS with a working Glob (Discover needs it).
type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS {
	return &memFS{files: make(map[string][]byte)}
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memFS) MkdirAll(_ string, _ os.FileMode) error { return nil }

func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return fakeFileInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (m *memFS) UserHomeDir() (string, error) { return "/home/test", nil }

func (m *memFS) Glob(pattern string) ([]string, error) {
	var matches []string
	for path := range m.files {
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// memState is an in-memory qa.StateFS.
type memState struct {
	files map[string][]byte
}

func (m *memState) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memState) WriteFileAtomic(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memState) MkdirAll(_ string, _ os.FileMode) error { return nil }

// mockRunner returns canned exec results keyed by command.
type mockRunner struct {
	results map[string]qa.ExecResult
}

func (m *mockRunner) Run(command, _ string, _ time.Duration) (qa.ExecResult, error) {
	return m.results[command], nil
}

const testChecklist = `
[checklist]
name = "Test walkthrough"

[[step]]
id = "build"
name = "Build passes"
cmd = "go-build"
expect = "build succeeds"
check = "ok"

[[step]]
id = "bar"
name = "Bar renders"
cmd = "show-bar"
expect = "the pill is visible"
needs = ["build"]

[[step]]
id = "look"
name = "Eyeball the dock"
expect = "dock looks right"
`

// testEnv wires Deps against an in-memory repo at the real process cwd
// (FindRepoRoot walks deps.FS from os.Getwd, so the virtual .git must
// sit on that path).
func testEnv(t *testing.T) (Deps, *memFS, *memState, *mockRunner) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fs := newMemFS()
	fs.files[filepath.Join(wd, ".git")] = []byte{}
	fs.files[filepath.Join(wd, "checklists", "walk.toml")] = []byte(testChecklist)

	state := &memState{files: map[string][]byte{}}
	runner := &mockRunner{results: map[string]qa.ExecResult{
		"go-build": {Exit: 0, Output: "build ok"},
		"show-bar": {Exit: 0, Output: "bar shown"},
	}}
	return Deps{FS: fs, State: state, Runner: runner}, fs, state, runner
}

func runCLI(t *testing.T, deps Deps, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd(deps)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var coded *codedError
	if !errors.As(err, &coded) {
		t.Fatalf("want codedError, got %v", err)
	}
	return coded.code
}

func TestLsShowsScorecard(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	out, err := runCLI(t, deps, "ls")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "walk") || !strings.Contains(out, "·3") {
		t.Errorf("ls output:\n%s", out)
	}
}

func TestRunSweepsAutomaticSteps(t *testing.T) {
	deps, _, state, _ := testEnv(t)
	out, err := runCLI(t, deps, "run", "walk")

	// Only "build" is automatic; bar (human-judged) and look stay unrun
	// → exit 2 (pending/unrun).
	if code := exitCode(t, err); code != 2 {
		t.Errorf("exit = %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "✓ build") {
		t.Errorf("output:\n%s", out)
	}
	if len(state.files) != 1 {
		t.Fatalf("state files = %v", state.files)
	}
	for path, raw := range state.files {
		// Scorecard lands in the repo-local .qa/ dir.
		if !strings.Contains(path, string(filepath.Separator)+".qa"+string(filepath.Separator)) {
			t.Errorf("state path = %s, want repo-local .qa/", path)
		}
		var run qa.Run
		if err := json.Unmarshal(raw, &run); err != nil {
			t.Fatal(err)
		}
		rec := run.Steps["build"]
		if rec.Result != qa.ResultPass || rec.By != "agent" {
			t.Errorf("build rec = %+v", rec)
		}
		if rec.Evidence == nil || !rec.Evidence.Matched {
			t.Errorf("evidence = %+v", rec.Evidence)
		}
	}
}

func TestRunKeepsHumanVerdict(t *testing.T) {
	deps, _, state, _ := testEnv(t)
	// A human already failed "build" via the picker.
	if _, err := runCLI(t, deps, "run", "walk", "build"); err == nil {
		t.Fatal("seed run should exit non-zero (pending steps remain)")
	}
	seedHumanVerdict(t, state, qa.ResultFail)

	out, _ := runCLI(t, deps, "run", "walk", "build")
	if !strings.Contains(out, "human-fail, kept") {
		t.Errorf("sweep stomped the human verdict:\n%s", out)
	}
	rec := persistedStep(t, state, "build")
	if rec.Result != qa.ResultFail || rec.By != "human" {
		t.Errorf("rec = %+v", rec)
	}

	// --force re-runs and overwrites.
	out, _ = runCLI(t, deps, "run", "walk", "build", "--force")
	if !strings.Contains(out, "✓ build") {
		t.Errorf("force run:\n%s", out)
	}
	if rec := persistedStep(t, state, "build"); rec.By != "agent" {
		t.Errorf("force rec = %+v", rec)
	}
}

// seedHumanVerdict rewrites the persisted "build" record as a human verdict.
func seedHumanVerdict(t *testing.T, state *memState, result qa.Result) {
	t.Helper()
	for path, raw := range state.files {
		var run qa.Run
		if err := json.Unmarshal(raw, &run); err != nil {
			t.Fatal(err)
		}
		rec := run.Steps["build"]
		rec.Result = result
		rec.By = "human"
		run.Steps["build"] = rec
		data, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		state.files[path] = data
		return
	}
	t.Fatal("no persisted run to seed")
}

// persistedStep reads a step record back from the single state file.
func persistedStep(t *testing.T, state *memState, id string) qa.StepRecord {
	t.Helper()
	for _, raw := range state.files {
		var run qa.Run
		if err := json.Unmarshal(raw, &run); err != nil {
			t.Fatal(err)
		}
		return run.Steps[id]
	}
	t.Fatal("no persisted run")
	return qa.StepRecord{}
}

func TestRunNamedHumanStepGoesPending(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	out, err := runCLI(t, deps, "run", "walk", "bar")
	if code := exitCode(t, err); code != 2 {
		t.Errorf("exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "⏳ bar") {
		t.Errorf("output:\n%s", out)
	}
	// "bar" needs "build" which hasn't passed — soft warn, never block.
	if !strings.Contains(out, "⚠ bar needs build") {
		t.Errorf("missing needs warning:\n%s", out)
	}
}

func TestRunFailingAutoStep(t *testing.T) {
	deps, _, _, runner := testEnv(t)
	runner.results["go-build"] = qa.ExecResult{Exit: 1, Output: "compile error: boom"}

	out, err := runCLI(t, deps, "run", "walk")
	if code := exitCode(t, err); code != 1 {
		t.Errorf("exit = %d, want 1\n%s", code, out)
	}
	// Failing steps surface their evidence tail.
	if !strings.Contains(out, "✗ build") || !strings.Contains(out, "compile error: boom") {
		t.Errorf("output:\n%s", out)
	}
}

func TestMarkAndStatusJSON(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	if _, err := runCLI(t, deps, "mark", "walk", "look", "pass", "--note", "user said LGTM"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, deps, "status", "walk", "--json")
	if code := exitCode(t, err); code != 2 { // others unrun
		t.Errorf("exit = %d", code)
	}
	var doc statusJSON
	if jerr := json.Unmarshal([]byte(out), &doc); jerr != nil {
		t.Fatalf("parse: %v\n%s", jerr, out)
	}
	if doc.Schema != 1 || doc.Checklist != "walk" || doc.Stale {
		t.Errorf("doc = %+v", doc)
	}
	if len(doc.Steps) != 3 {
		t.Fatalf("steps = %d", len(doc.Steps))
	}
	for _, s := range doc.Steps {
		switch s.ID {
		case "look":
			if s.Result != "pass" || s.By != "agent" || s.Note != "user said LGTM" {
				t.Errorf("look = %+v", s)
			}
		case "build":
			if s.Result != "unrun" || !s.Automatic {
				t.Errorf("build = %+v", s)
			}
		}
	}
}

func TestMarkRejectsBadVerdict(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	if _, err := runCLI(t, deps, "mark", "walk", "look", "maybe"); err == nil {
		t.Error("want error for bad verdict")
	}
}

func TestStatusAllPassExitsZero(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	for _, step := range []string{"build", "bar", "look"} {
		if _, err := runCLI(t, deps, "mark", "walk", step, "pass"); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCLI(t, deps, "status", "walk")
	if err != nil {
		t.Errorf("all-pass status: %v\n%s", err, out)
	}
}

func TestStaleBlocksWithoutForce(t *testing.T) {
	deps, fs, _, _ := testEnv(t)
	if _, err := runCLI(t, deps, "mark", "walk", "look", "pass"); err != nil {
		t.Fatal(err)
	}

	// Checklist edited under the run.
	wd, _ := os.Getwd()
	path := filepath.Join(wd, "checklists", "walk.toml")
	fs.files[path] = append(fs.files[path], []byte("\n# edited\n")...)

	if _, err := runCLI(t, deps, "mark", "walk", "build", "pass"); !errors.Is(err, qa.ErrStale) {
		t.Errorf("got %v, want ErrStale", err)
	}
	if _, err := runCLI(t, deps, "mark", "walk", "build", "pass", "--force"); err != nil {
		t.Errorf("forced mark: %v", err)
	}

	out, err := runCLI(t, deps, "status", "walk")
	exitCode(t, err) // still nonzero — fine
	if !strings.Contains(out, "STALE") {
		t.Errorf("forced write must keep the stale banner:\n%s", out)
	}
}

func TestResetGuardsFindings(t *testing.T) {
	deps, _, _, runner := testEnv(t)
	runner.results["go-build"] = qa.ExecResult{Exit: 1, Output: "boom"}
	if out, err := runCLI(t, deps, "run", "walk"); exitCode(t, err) != 1 {
		t.Fatalf("setup run: %v\n%s", err, out)
	}

	if _, err := runCLI(t, deps, "reset", "walk"); err == nil || !strings.Contains(err.Error(), "findings") {
		t.Errorf("reset with fail should need --force, got %v", err)
	}
	if _, err := runCLI(t, deps, "reset", "walk", "--force"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, deps, "status", "walk")
	if code := exitCode(t, err); code != 2 {
		t.Errorf("post-reset exit = %d\n%s", code, out)
	}
	if strings.Contains(out, "✗ build") {
		t.Errorf("reset kept a fail:\n%s", out)
	}
}

func TestLint(t *testing.T) {
	deps, fs, _, _ := testEnv(t)
	if out, err := runCLI(t, deps, "lint"); err != nil {
		t.Fatalf("clean lint: %v\n%s", err, out)
	}

	wd, _ := os.Getwd()
	fs.files[filepath.Join(wd, "checklists", "broken.toml")] = []byte(`
[checklist]
name = "broken"

[[step]]
id = "x"
expect = "e"
check = "orphan check"
`)
	out, err := runCLI(t, deps, "lint")
	if err == nil {
		t.Fatalf("want lint failure\n%s", out)
	}
	if !strings.Contains(out, "check without cmd") {
		t.Errorf("output:\n%s", out)
	}
}

func TestUnknownChecklistListsStems(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	_, err := runCLI(t, deps, "status", "ghost")
	if err == nil || !strings.Contains(err.Error(), "walk") {
		t.Errorf("miss should list available stems, got %v", err)
	}
}

func TestRunReturnsExitCode(t *testing.T) {
	deps, _, _, _ := testEnv(t)
	if code := Run(deps, []string{"run", "walk"}); code != 2 {
		t.Errorf("Run exit = %d, want 2", code)
	}
}
