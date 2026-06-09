package qa

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testChecklist() *Checklist {
	return &Checklist{
		Name:     "x",
		Stem:     "x",
		Path:     "/repo/qa/x.toml",
		Checksum: "aaa",
		Steps:    []Step{{ID: "build", Expect: "ok"}, {ID: "look", Expect: "ok"}},
	}
}

func testStore(fs StateFS) *Store {
	st := NewStore(fs, "/repo")
	tick := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	st.now = func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}
	return st
}

func TestLoadAbsent(t *testing.T) {
	st := testStore(newMemStateFS())
	run, err := st.Load("x")
	if err != nil || run != nil {
		t.Errorf("Load on absent = %v, %v; want nil, nil", run, err)
	}
}

func TestLoadVersionMismatch(t *testing.T) {
	fs := newMemStateFS()
	st := testStore(fs)
	path := st.runPath("x")
	fs.files[path] = []byte(`{"state_version": 99}`)
	if _, err := st.Load("x"); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("got %v", err)
	}
}

func TestMark(t *testing.T) {
	cl := testChecklist()
	fs := newMemStateFS()
	st := testStore(fs)

	run, err := st.Mark(cl, "build", StepRecord{Result: ResultPass, By: "agent"}, false)
	if err != nil {
		t.Fatalf("Mark: %v", err)
	}
	if run.Checksum != "aaa" || run.Checklist != "x" || run.RepoRoot != "/repo" {
		t.Errorf("run binding = %+v", run)
	}
	rec := run.Steps["build"]
	if rec.Result != ResultPass || rec.By != "agent" || rec.At.IsZero() {
		t.Errorf("rec = %+v", rec)
	}
	if !run.Updated.Equal(rec.At) {
		t.Errorf("Updated = %v, rec.At = %v", run.Updated, rec.At)
	}

	// Second mark merges — first step survives the reload.
	run, err = st.Mark(cl, "look", StepRecord{Result: ResultFail, By: "human", Note: "pill missing"}, false)
	if err != nil {
		t.Fatalf("second Mark: %v", err)
	}
	if run.Steps["build"].Result != ResultPass {
		t.Error("reload-merge lost the first step")
	}
	if run.Steps["look"].Note != "pill missing" {
		t.Errorf("look = %+v", run.Steps["look"])
	}
}

func TestMarkUnknownStep(t *testing.T) {
	st := testStore(newMemStateFS())
	if _, err := st.Mark(testChecklist(), "ghost", StepRecord{Result: ResultPass}, false); err == nil {
		t.Error("unknown step: want error")
	}
}

func TestMarkStale(t *testing.T) {
	cl := testChecklist()
	fs := newMemStateFS()
	st := testStore(fs)
	if _, err := st.Mark(cl, "build", StepRecord{Result: ResultPass}, false); err != nil {
		t.Fatal(err)
	}

	// Checklist bytes changed underneath the run.
	edited := testChecklist()
	edited.Checksum = "bbb"

	if _, err := st.Mark(edited, "look", StepRecord{Result: ResultPass}, false); !errors.Is(err, ErrStale) {
		t.Errorf("stale unforced: got %v, want ErrStale", err)
	}

	// Forced write lands — and deliberately keeps the OLD checksum so the
	// staleness banner persists until reset.
	run, err := st.Mark(edited, "look", StepRecord{Result: ResultPass}, true)
	if err != nil {
		t.Fatalf("forced Mark: %v", err)
	}
	if run.Steps["look"].Result != ResultPass {
		t.Error("forced write missing")
	}
	if run.Checksum != "aaa" {
		t.Errorf("forced write rebound checksum to %q; staleness must persist", run.Checksum)
	}
	if !run.Stale(edited) {
		t.Error("run should still report stale after a forced write")
	}
}

func TestReset(t *testing.T) {
	cl := testChecklist()
	fs := newMemStateFS()
	st := testStore(fs)
	if _, err := st.Mark(cl, "build", StepRecord{Result: ResultFail}, false); err != nil {
		t.Fatal(err)
	}

	edited := testChecklist()
	edited.Checksum = "bbb"
	run, err := st.Reset(edited)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 0 {
		t.Errorf("Reset kept steps: %v", run.Steps)
	}
	if run.Checksum != "bbb" || run.Stale(edited) {
		t.Errorf("Reset did not rebind checksum: %q", run.Checksum)
	}

	// Persisted, not just returned.
	loaded, err := st.Load("x")
	if err != nil || loaded == nil || len(loaded.Steps) != 0 {
		t.Errorf("reloaded run = %+v, %v", loaded, err)
	}
}

func TestSummarize(t *testing.T) {
	cl := &Checklist{Steps: []Step{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}, {ID: "new"},
	}}
	run := &Run{Steps: map[string]StepRecord{
		"a": {Result: ResultPass},
		"b": {Result: ResultFail},
		"c": {Result: ResultError},
		"d": {Result: ResultPending},
		// e untouched, "new" added to the checklist after the run started.
	}}
	got := Summarize(cl, run)
	want := Summary{Pass: 1, Fail: 1, Error: 1, Pending: 1, Unrun: 2}
	if got != want {
		t.Errorf("Summarize = %+v, want %+v", got, want)
	}

	if got := Summarize(cl, nil); got.Unrun != 6 {
		t.Errorf("nil run: %+v", got)
	}
}

func TestRunPathIsRepoLocal(t *testing.T) {
	st := NewStore(newMemStateFS(), "/home/u/app")
	if got := st.runPath("walk"); got != "/home/u/app/.qa/walk.json" {
		t.Errorf("runPath = %q", got)
	}
}

func TestRealStateFSWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	var fs RealStateFS

	if err := fs.WriteFileAtomic(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFileAtomic(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "v2" {
		t.Errorf("read back %q, %v", got, err)
	}

	// No temp litter left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".qa-") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}
