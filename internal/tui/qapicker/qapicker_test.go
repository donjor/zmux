package qapicker

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/qa"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tkey"
)

// memStateFS is an in-memory qa.StateFS.
type memStateFS struct {
	files map[string][]byte
}

func (m *memStateFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memStateFS) WriteFileAtomic(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memStateFS) MkdirAll(_ string, _ os.FileMode) error { return nil }

// mockRunner returns canned results keyed by command.
type mockRunner struct {
	results map[string]qa.ExecResult
}

func (m *mockRunner) Run(command, _ string, _ time.Duration) (qa.ExecResult, error) {
	return m.results[command], nil
}

func testChecklist() *qa.Checklist {
	return &qa.Checklist{
		Name:     "Walk",
		Stem:     "walk",
		Path:     "/repo/qa/walk.toml",
		Checksum: "aaa",
		Steps: []qa.Step{
			{ID: "build", Name: "Build", Cmd: "go-build", Expect: "builds", Check: "ok"},
			{ID: "bar", Name: "Bar", Cmd: "show-bar", Expect: "pill visible"},
			{ID: "look", Name: "Look", Expect: "dock looks right"},
		},
	}
}

func newTestModel(t *testing.T) (Model, *memStateFS, *mockRunner) {
	t.Helper()
	state := &memStateFS{files: map[string][]byte{}}
	store := qa.NewStore(state, "/state")
	runner := &mockRunner{results: map[string]qa.ExecResult{
		"go-build": {Exit: 0, Output: "build ok"},
		"show-bar": {Exit: 0, Output: "bar shown"},
	}}
	m := New(testChecklist(), nil, store, "/repo", runner, styles.DefaultStyles())
	m.width, m.height = 80, 24
	return m, state, runner
}

func send(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tkey.Enter()
	case "esc":
		msg = tkey.Esc()
	case "down":
		msg = tkey.Down()
	default:
		msg = tkey.Type(k)
	}
	out, _ := m.Update(msg)
	return out.(Model)
}

// runStep drives enter through the async exec cmd and delivers its msg.
func runStep(t *testing.T, m Model) Model {
	t.Helper()
	out, cmd := m.Update(tkey.Enter())
	m = out.(Model)
	if m.mode != modeRunning {
		t.Fatalf("mode = %d, want running", m.mode)
	}
	if cmd == nil {
		t.Fatal("no exec cmd returned")
	}
	out, _ = m.Update(cmd())
	return out.(Model)
}

// persistedRecord reads a step record back from the state file.
func persistedRecord(t *testing.T, state *memStateFS, id string) (qa.StepRecord, bool) {
	t.Helper()
	for _, raw := range state.files {
		var run qa.Run
		if err := json.Unmarshal(raw, &run); err != nil {
			t.Fatal(err)
		}
		rec, ok := run.Steps[id]
		return rec, ok
	}
	return qa.StepRecord{}, false
}

func TestAutoStepPersistsImmediatelyAndPrejudges(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m) // cursor on "build" (automatic)

	if m.mode != modeVerdict || !m.persisted {
		t.Fatalf("mode = %d persisted = %v", m.mode, m.persisted)
	}
	// The prejudge is the regexp's work, not a human judgment.
	rec, ok := persistedRecord(t, state, "build")
	if !ok || rec.Result != qa.ResultPass || rec.By != "auto" {
		t.Errorf("persisted = %+v ok=%v", rec, ok)
	}

	// Esc keeps the prejudged pass.
	m = send(m, "esc")
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
	if rec, _ := persistedRecord(t, state, "build"); rec.Result != qa.ResultPass {
		t.Errorf("esc changed the record: %+v", rec)
	}
}

func TestAutoStepVerdictOverride(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m)
	m = send(m, "f") // human overrides the prejudged pass
	if m.mode != modeList {
		t.Errorf("mode = %d, want list after verdict", m.mode)
	}

	rec, _ := persistedRecord(t, state, "build")
	if rec.Result != qa.ResultFail || rec.By != "human" {
		t.Errorf("rec = %+v", rec)
	}
	if rec.Evidence == nil {
		t.Error("override must keep execution evidence")
	}
}

func TestHumanJudgedStepLandsPending(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = send(m, "down") // → "bar" (cmd only)
	m = runStep(t, m)

	rec, _ := persistedRecord(t, state, "bar")
	if rec.Result != qa.ResultPending {
		t.Errorf("rec = %+v", rec)
	}
	m = send(m, "p")
	if rec, _ := persistedRecord(t, state, "bar"); rec.Result != qa.ResultPass {
		t.Errorf("verdict rec = %+v", rec)
	}
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
}

func TestInstructionOnlyStepPromptsWithoutRunning(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = send(m, "down")
	m = send(m, "down") // → "look" (instruction-only)
	m = send(m, "enter")

	if m.mode != modeVerdict || m.persisted {
		t.Fatalf("mode = %d persisted = %v", m.mode, m.persisted)
	}
	// Esc abandons — nothing persisted.
	m = send(m, "esc")
	if _, ok := persistedRecord(t, state, "look"); ok {
		t.Error("esc on unrun instruction step must not persist")
	}

	m = send(m, "enter")
	m = send(m, "p")
	rec, _ := persistedRecord(t, state, "look")
	if rec.Result != qa.ResultPass || rec.By != "human" {
		t.Errorf("rec = %+v", rec)
	}
}

func TestBrokenCommandLandsError(t *testing.T) {
	m, state, runner := newTestModel(t)
	runner.results["show-bar"] = qa.ExecResult{Exit: 2, Output: "boom"}
	m = send(m, "down")
	m = runStep(t, m)

	rec, _ := persistedRecord(t, state, "bar")
	if rec.Result != qa.ResultError {
		t.Errorf("rec = %+v", rec)
	}
	if m.mode != modeVerdict {
		t.Errorf("mode = %d", m.mode)
	}
}

func TestStaleRunBlocksMark(t *testing.T) {
	m, _, _ := newTestModel(t)
	// Seed a run bound to different checklist bytes.
	if _, err := m.store.Mark(m.cl, "build", qa.StepRecord{Result: qa.ResultPass}, false); err != nil {
		t.Fatal(err)
	}
	m.cl.Checksum = "bbb"
	run, err := m.store.Load("walk")
	if err != nil {
		t.Fatal(err)
	}
	m.run = run

	m = runStep(t, m)
	if m.mode != modeList || !strings.Contains(m.errMsg, "R to reset") {
		t.Errorf("mode = %d errMsg = %q", m.mode, m.errMsg)
	}
}

func TestResetConfirm(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m)
	m = send(m, "esc")

	m = send(m, "R")
	if m.mode != modeResetConfirm {
		t.Fatalf("mode = %d", m.mode)
	}
	m = send(m, "n")
	if rec, ok := persistedRecord(t, state, "build"); !ok || rec.Result != qa.ResultPass {
		t.Errorf("cancel reset wiped the run: %+v", rec)
	}

	m = send(m, "R")
	m = send(m, "y")
	if _, ok := persistedRecord(t, state, "build"); ok {
		t.Error("reset kept the old record")
	}
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
}

func TestViewShowsMarksExpectAndFooter(t *testing.T) {
	m, _, _ := newTestModel(t)
	m = runStep(t, m)
	m = send(m, "esc")

	view := m.view()
	if !strings.Contains(view, "✓") || !strings.Contains(view, "Build") {
		t.Errorf("missing pass mark:\n%s", view)
	}
	if !strings.Contains(view, "[auto]") {
		t.Errorf("missing by attribution:\n%s", view)
	}
	if !strings.Contains(view, "EXPECT") {
		t.Errorf("missing expect box:\n%s", view)
	}
	if !strings.Contains(view, "✓1 ✗0 !0 ⏳0 ·2") {
		t.Errorf("missing live footer:\n%s", view)
	}
}

func TestVerdictViewShowsOutputAndPrejudge(t *testing.T) {
	m, _, _ := newTestModel(t)
	m = runStep(t, m)

	view := m.view()
	if !strings.Contains(view, "build ok") {
		t.Errorf("missing captured output:\n%s", view)
	}
	if !strings.Contains(view, "recorded as") || !strings.Contains(view, "pass") {
		t.Errorf("missing prejudge:\n%s", view)
	}
}

func TestNoteSavedWithVerdict(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m) // "build" → verdict overlay

	m = send(m, "n")
	if m.mode != modeNote {
		t.Fatalf("mode = %d, want note editor", m.mode)
	}
	m = send(m, "looks off")
	m = send(m, "enter") // stage
	if m.mode != modeVerdict || !m.noteDirty {
		t.Fatalf("mode = %d dirty = %v", m.mode, m.noteDirty)
	}

	m = send(m, "f")
	rec, _ := persistedRecord(t, state, "build")
	if rec.Result != qa.ResultFail || rec.Note != "looks off" || rec.By != "human" {
		t.Errorf("rec = %+v", rec)
	}
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
}

func TestNoteEscKeepPersistsNote(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m) // prejudged pass already persisted

	m = send(m, "n")
	m = send(m, "pill alignment slightly off")
	m = send(m, "enter")
	m = send(m, "esc") // keep the pass — note must not be dropped

	rec, _ := persistedRecord(t, state, "build")
	if rec.Result != qa.ResultPass || rec.Note != "pill alignment slightly off" {
		t.Errorf("rec = %+v", rec)
	}
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
}

func TestNoteEditCancelled(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = runStep(t, m)

	m = send(m, "n")
	m = send(m, "scratch this")
	m = send(m, "esc") // cancel the edit
	if m.mode != modeVerdict || m.noteDirty {
		t.Fatalf("mode = %d dirty = %v", m.mode, m.noteDirty)
	}
	m = send(m, "esc") // keep
	rec, _ := persistedRecord(t, state, "build")
	if rec.Note != "" {
		t.Errorf("cancelled note leaked: %+v", rec)
	}
}

func TestNoteOnInstructionStepAbandonedByEsc(t *testing.T) {
	m, state, _ := newTestModel(t)
	m = send(m, "down")
	m = send(m, "down") // → "look" (instruction-only)
	m = send(m, "enter")

	m = send(m, "n")
	m = send(m, "half-formed thought")
	m = send(m, "enter")
	m = send(m, "esc") // abandon the unrun step — nothing persists
	if m.mode != modeList {
		t.Errorf("mode = %d", m.mode)
	}
	if _, ok := persistedRecord(t, state, "look"); ok {
		t.Error("abandoned instruction step must not persist")
	}
}

func TestQuit(t *testing.T) {
	m, _, _ := newTestModel(t)
	out, cmd := m.Update(tkey.Type("q"))
	if !out.(Model).Quitting || cmd == nil {
		t.Error("q must quit")
	}
}
