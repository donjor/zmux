package qa

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// stateVersion guards the JSON shape; bumps invalidate old runs loudly.
const stateVersion = 1

// Result is a step's verification outcome. Zero value = unrun.
type Result string

const (
	ResultUnrun   Result = ""        // never touched
	ResultPending Result = "pending" // cmd ran for a human-judged step; awaiting verdict
	ResultPass    Result = "pass"
	ResultFail    Result = "fail"
	ResultError   Result = "error" // the command itself broke — never forced into pass/fail
)

// Evidence is the structured record of a command execution.
type Evidence struct {
	Command    string `json:"command"`
	Exit       int    `json:"exit"`
	DurationMS int64  `json:"duration_ms"`
	Output     string `json:"output"` // combined stdout+stderr tail (≤ EvidenceCap)
	Matched    bool   `json:"matched,omitempty"`
}

// StepRecord is one step's scorecard entry.
type StepRecord struct {
	Result   Result    `json:"result"`
	By       string    `json:"by,omitempty"` // "agent" | "human"
	Note     string    `json:"note,omitempty"`
	At       time.Time `json:"at"`
	Evidence *Evidence `json:"evidence,omitempty"`
}

// Run is the persisted scorecard: one active run per checklist (v1).
type Run struct {
	StateVersion  int                   `json:"state_version"`
	Checklist     string                `json:"checklist"`
	ChecklistPath string                `json:"checklist_path"`
	RepoRoot      string                `json:"repo_root"`
	Checksum      string                `json:"checksum"`
	Started       time.Time             `json:"started"`
	Updated       time.Time             `json:"updated_at"`
	Steps         map[string]StepRecord `json:"steps"`
}

// ErrStale marks a run whose checklist changed underneath it: writes are
// blocked (mixing results across checklist versions lies on the scorecard)
// until the caller forces or resets.
var ErrStale = errors.New("checklist changed since this run started — reset (qa reset) or pass --force to keep mixing results")

// Store reads and writes runs. Every write is reload-merge-write through
// the atomic StateFS, so concurrent human/agent marks interleave per step
// (last writer wins per step, never per file).
type Store struct {
	fs       StateFS
	repoRoot string
	dir      string // <repo>/.qa — gitignored, next to the checklists it scores
	now      func() time.Time
}

// NewStore builds a store for one repo. Scorecards live in the repo's
// gitignored .qa/ dir — inspectable, per-worktree by construction.
func NewStore(fs StateFS, repoRoot string) *Store {
	return &Store{fs: fs, repoRoot: repoRoot, dir: filepath.Join(repoRoot, ".qa"), now: time.Now}
}

// runPath is the scorecard location for a checklist.
func (st *Store) runPath(stem string) string {
	return filepath.Join(st.dir, stem+".json")
}

// Load returns the active run, or nil when none exists yet.
func (st *Store) Load(stem string) (*Run, error) {
	raw, err := st.fs.ReadFile(st.runPath(stem))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read run state: %w", err)
	}
	var run Run
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, fmt.Errorf("parse run state: %w", err)
	}
	if run.StateVersion != stateVersion {
		return nil, fmt.Errorf("run state version %d unsupported (want %d) — qa reset", run.StateVersion, stateVersion)
	}
	return &run, nil
}

// save persists a run atomically.
func (st *Store) save(stem string, run *Run) error {
	path := st.runPath(stem)
	if err := st.fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	raw, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return st.fs.WriteFileAtomic(path, raw, 0o644)
}

// newRun initializes an empty scorecard bound to the checklist version.
func (st *Store) newRun(cl *Checklist) *Run {
	return &Run{
		StateVersion:  stateVersion,
		Checklist:     cl.Stem,
		ChecklistPath: cl.Path,
		RepoRoot:      st.repoRoot,
		Checksum:      cl.Checksum,
		Started:       st.now(),
		Steps:         map[string]StepRecord{},
	}
}

// Stale reports whether the run predates the current checklist bytes.
func (run *Run) Stale(cl *Checklist) bool {
	return run != nil && run.Checksum != cl.Checksum
}

// Mark records a step result via reload-merge-write. A stale run blocks
// the write with ErrStale unless force; forced writes deliberately leave
// the recorded checksum stale so status keeps flagging the mix.
func (st *Store) Mark(cl *Checklist, stepID string, rec StepRecord, force bool) (*Run, error) {
	if cl.StepByID(stepID) == nil {
		return nil, fmt.Errorf("no step %q in checklist %s", stepID, cl.Stem)
	}
	run, err := st.Load(cl.Stem)
	if err != nil {
		return nil, err
	}
	if run == nil {
		run = st.newRun(cl)
	} else if run.Stale(cl) && !force {
		return nil, ErrStale
	}
	rec.At = st.now()
	run.Steps[stepID] = rec
	run.Updated = rec.At
	if err := st.save(cl.Stem, run); err != nil {
		return nil, err
	}
	return run, nil
}

// Reset discards the scorecard and starts a fresh run bound to the
// current checklist bytes. Callers gate destructive resets (`--force`,
// picker confirm) — the store just obeys.
func (st *Store) Reset(cl *Checklist) (*Run, error) {
	run := st.newRun(cl)
	if err := st.save(cl.Stem, run); err != nil {
		return nil, err
	}
	return run, nil
}

// Summary tallies a run against its checklist (unrun derives from the
// checklist, not the state file — steps added later count as unrun).
type Summary struct {
	Pass, Fail, Error, Pending, Unrun int
}

// Summarize counts results for every checklist step.
func Summarize(cl *Checklist, run *Run) Summary {
	var s Summary
	for i := range cl.Steps {
		var rec StepRecord
		if run != nil {
			rec = run.Steps[cl.Steps[i].ID]
		}
		switch rec.Result {
		case ResultPass:
			s.Pass++
		case ResultFail:
			s.Fail++
		case ResultError:
			s.Error++
		case ResultPending:
			s.Pending++
		default:
			s.Unrun++
		}
	}
	return s
}
