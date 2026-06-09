// Package qa is the human+agent QA walkthrough framework (plan 028): a
// committed checklist (checklists/*.toml) describes a verification walkthrough; a
// mutable run records who verified what. Two frontends — the qapicker TUI
// (human) and the qa CLI verbs (agent) — share both.
package qa

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/donjor/zmux/internal/config"
)

// Step is one walkthrough entry. The semantics fall out of two optional
// fields (no kind enum):
//
//	cmd + check  → automatic: run, match, mark
//	cmd only     → command runs for you, a HUMAN judges the expectation
//	neither      → instruction-only (press a key, look at the bar)
//	check only   → invalid (lint error)
type Step struct {
	ID      string   `toml:"id"`
	Name    string   `toml:"name"`
	Cmd     string   `toml:"cmd"`
	Expect  string   `toml:"expect"`
	Check   string   `toml:"check"`   // Go regexp over combined output
	Needs   []string `toml:"needs"`   // soft ordering — warn, never block
	Timeout int      `toml:"timeout"` // seconds; 0 → default (60)
}

// Automatic reports whether the step can be machine-verified end to end.
func (s *Step) Automatic() bool { return s.Cmd != "" && s.Check != "" }

// Checklist is one parsed checklists/*.toml walkthrough.
type Checklist struct {
	Name  string            `toml:"name"`
	Doc   string            `toml:"doc"`
	Vars  map[string]string `toml:"vars"`
	Steps []Step            `toml:"-"`

	// Provenance (not in TOML).
	Path     string `toml:"-"` // source file
	Stem     string `toml:"-"` // file stem — the addressable name
	Checksum string `toml:"-"` // sha256 hex of the raw bytes
}

// checklistFile mirrors the TOML document shape.
type checklistFile struct {
	Checklist Checklist `toml:"checklist"`
	Steps     []Step    `toml:"step"`
}

// stepIDPattern keeps ids addressable from shells and state keys.
var stepIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// varPattern matches a {var} substitution site (after escape handling).
var varPattern = regexp.MustCompile(`\{([a-zA-Z0-9_-]+)\}`)

// Load reads and parses one checklist file. err is fatal (IO/TOML);
// issues are lint findings — callers other than `qa lint` treat any
// issue as fatal too, so a broken checklist never half-runs.
func Load(fs config.FS, path string) (*Checklist, []string, error) {
	raw, err := fs.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read checklist: %w", err)
	}
	var file checklistFile
	if err := toml.Unmarshal(raw, &file); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cl := file.Checklist
	cl.Steps = file.Steps
	cl.Path = path
	cl.Stem = strings.TrimSuffix(filepath.Base(path), ".toml")
	sum := sha256.Sum256(raw)
	cl.Checksum = hex.EncodeToString(sum[:])
	return &cl, Lint(&cl), nil
}

// Lint returns every schema violation in the checklist; empty = clean.
func Lint(cl *Checklist) []string {
	var issues []string
	add := func(format string, a ...any) { issues = append(issues, fmt.Sprintf(format, a...)) }

	if cl.Name == "" {
		add("checklist.name is required")
	}
	if len(cl.Steps) == 0 {
		add("checklist has no steps")
	}

	ids := make(map[string]bool, len(cl.Steps))
	for i := range cl.Steps {
		s := &cl.Steps[i]
		where := fmt.Sprintf("step %q", s.ID)
		if s.ID == "" {
			where = fmt.Sprintf("step #%d", i+1)
			add("%s: id is required", where)
		} else if !stepIDPattern.MatchString(s.ID) {
			add("%s: id must match %s", where, stepIDPattern)
		} else if ids[s.ID] {
			add("%s: duplicate id", where)
		}
		ids[s.ID] = true

		if s.Expect == "" {
			add("%s: expect is required — a step without an expectation can't be verified", where)
		}
		if s.Check != "" && s.Cmd == "" {
			add("%s: check without cmd — nothing to check against", where)
		}
		if s.Check != "" {
			if _, err := regexp.Compile(s.Check); err != nil {
				add("%s: check is not a valid Go regexp: %v", where, err)
			}
		}
		if s.Timeout < 0 {
			add("%s: timeout must be >= 0", where)
		}
		if s.Cmd != "" {
			if _, err := expand(s.Cmd, cl.Vars); err != nil {
				add("%s: %v", where, err)
			}
		}
	}

	for i := range cl.Steps {
		s := &cl.Steps[i]
		for _, need := range s.Needs {
			if need == s.ID {
				add("step %q: needs itself", s.ID)
			} else if !ids[need] {
				add("step %q: needs unknown step %q", s.ID, need)
			}
		}
	}
	return issues
}

// StepByID returns the step with the given id, or nil.
func (cl *Checklist) StepByID(id string) *Step {
	for i := range cl.Steps {
		if cl.Steps[i].ID == id {
			return &cl.Steps[i]
		}
	}
	return nil
}

// Command returns the step's cmd with {var} substitution applied. Escapes:
// `{{` and `}}` are literal braces. Unknown vars error (lint catches them
// up front; this guards direct callers).
func (cl *Checklist) Command(s *Step) (string, error) {
	return expand(s.Cmd, cl.Vars)
}

// expand substitutes {var} sites from vars, honoring {{ }} escapes.
func expand(in string, vars map[string]string) (string, error) {
	const lbrace, rbrace = "\x00", "\x01"
	work := strings.ReplaceAll(in, "{{", lbrace)
	work = strings.ReplaceAll(work, "}}", rbrace)

	var unknown []string
	work = varPattern.ReplaceAllStringFunc(work, func(site string) string {
		name := site[1 : len(site)-1]
		if val, ok := vars[name]; ok {
			return val
		}
		unknown = append(unknown, name)
		return site
	})
	if len(unknown) > 0 {
		return "", fmt.Errorf("unknown var(s) %s in %q", strings.Join(unknown, ", "), in)
	}
	work = strings.ReplaceAll(work, lbrace, "{")
	work = strings.ReplaceAll(work, rbrace, "}")
	return work, nil
}
