package guard

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// corpusRow mirrors a line of testdata/zmux-guard-corpus.jsonl.
type corpusRow struct {
	Command  string `json:"command"`
	Cwd      string `json:"cwd"`
	Kind     string `json:"kind"`
	Decision string `json:"decision"`
	Target   string `json:"target"`
	Note     string `json:"note"`
}

func loadCorpus(t *testing.T) []corpusRow {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "zmux-guard-corpus.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	defer f.Close()
	var rows []corpusRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r corpusRow
		if err := json.Unmarshal(line, &r); err != nil {
			t.Fatalf("parse corpus line %q: %v", string(line), err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan corpus: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("corpus is empty")
	}
	return rows
}

// TestClassifyAgainstCorpus is the drift gate: every shared-corpus row must
// classify to its recorded kind + decision. The same corpus backs the Claude
// hook and pi-extension tests.
func TestClassifyAgainstCorpus(t *testing.T) {
	for _, r := range loadCorpus(t) {
		got := Classify(r.Command, Options{RepoCwd: r.Cwd == "repo"})
		if string(got.Kind) != r.Kind {
			t.Errorf("kind mismatch for %q (cwd=%s): got %q want %q [%s]", r.Command, r.Cwd, got.Kind, r.Kind, r.Note)
		}
		if string(got.Decision) != r.Decision {
			t.Errorf("decision mismatch for %q (cwd=%s): got %q want %q [%s]", r.Command, r.Cwd, got.Decision, r.Decision, r.Note)
		}
	}
}

// TestBlockedRowsCarryTarget guards that anything we block also tells the agent
// where to go instead (except background, which maps to the generic runtime).
func TestBlockedRowsCarryTarget(t *testing.T) {
	for _, r := range loadCorpus(t) {
		if r.Decision != "block" {
			continue
		}
		got := Classify(r.Command, Options{RepoCwd: r.Cwd == "repo"})
		if got.Target == "" {
			t.Errorf("blocked command %q has no suggestion target", r.Command)
		}
	}
}
