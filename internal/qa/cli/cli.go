// Package cli is the command tree for the dedicated `qa` runner binary
// (cmd/qa, invoked via the repo's ./qa wrapper). It lives apart from the
// zmux command tree on purpose: QA is its own surface.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/qa"
	"github.com/donjor/zmux/internal/tui/qapicker"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/spf13/cobra"
)

// Deps are the runner's injected side-effect seams.
type Deps struct {
	FS     config.FS
	State  qa.StateFS
	Runner qa.CmdRunner
}

// Scorecard exit codes (pinned, plan 028): 0 all-pass, 1 any fail/error,
// 2 any pending/unrun — so an agent can gate on `qa run` / `qa status`.
const (
	exitFail    = 1
	exitPending = 2
)

// codedError carries an explicit exit code; empty msg means the command
// already printed its output.
type codedError struct {
	code int
	msg  string
}

func (e *codedError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("exit %d", e.code)
}

// Run executes the qa command tree and returns the process exit code.
func Run(deps Deps, args []string) int {
	root := NewRootCmd(deps)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		var coded *codedError
		if errors.As(err, &coded) {
			if coded.msg != "" {
				fmt.Fprintln(os.Stderr, coded.msg)
			}
			return coded.code
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// resultGlyph maps a step result to its scorecard mark.
func resultGlyph(r qa.Result) string {
	switch r {
	case qa.ResultPass:
		return "✓"
	case qa.ResultFail:
		return "✗"
	case qa.ResultError:
		return "!"
	case qa.ResultPending:
		return "⏳"
	default:
		return "·"
	}
}

// env is the resolved context every verb needs.
type env struct {
	deps     Deps
	repoRoot string
	store    *qa.Store
}

func setup(deps Deps) (*env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := qa.FindRepoRoot(deps.FS, cwd)
	if err != nil {
		return nil, err
	}
	return &env{
		deps:     deps,
		repoRoot: root,
		store:    qa.NewStore(deps.State, root),
	}, nil
}

// loadChecklist resolves and parses a checklist; lint issues are fatal
// for every verb except `qa lint` — a broken checklist never half-runs.
func (e *env) loadChecklist(name string) (*qa.Checklist, error) {
	path, err := qa.Resolve(e.deps.FS, e.repoRoot, name)
	if err != nil {
		return nil, err
	}
	cl, issues, err := qa.Load(e.deps.FS, path)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, fmt.Errorf("checklist %s has lint issues (qa lint):\n  %s",
			cl.Stem, strings.Join(issues, "\n  "))
	}
	return cl, nil
}

// scorecardExit maps a summary to the pinned exit-code contract.
func scorecardExit(s qa.Summary) error {
	switch {
	case s.Fail > 0 || s.Error > 0:
		return &codedError{code: exitFail}
	case s.Pending > 0 || s.Unrun > 0:
		return &codedError{code: exitPending}
	default:
		return nil
	}
}

func summaryLine(s qa.Summary) string {
	return fmt.Sprintf("✓%d ✗%d !%d ⏳%d ·%d", s.Pass, s.Fail, s.Error, s.Pending, s.Unrun)
}

// NewRootCmd builds the qa command tree.
func NewRootCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "qa [checklist]",
		Short: "QA walkthrough runner (human + agent)",
		Long: `QA walkthroughs: committed checklists (checklists/*.toml) describing how to verify
a change, with one shared scorecard per checklist (gitignored .qa/) that a
human (picker) and an agent (verbs) fill in together.

Step semantics fall out of two optional fields:
  cmd + check   automatic — run, regexp-match, mark
  cmd only      command runs for you, a human judges the expectation
  neither       instruction-only (press a key, look at the bar)

Bare 'qa' opens the picker (checklist select → steps); 'qa <checklist>'
jumps straight into one. Picker marks are by=human, verb marks by=agent.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runPicker(deps, name)
		},
	}
	cmd.AddCommand(
		newLsCmd(deps),
		newRunCmd(deps),
		newMarkCmd(deps),
		newStatusCmd(deps),
		newResetCmd(deps),
		newLintCmd(deps),
	)
	return cmd
}

// runPicker opens the interactive walkthrough — checklist select when no
// name is given (straight into the steps when only one checklist exists).
func runPicker(deps Deps, name string) error {
	e, err := setup(deps)
	if err != nil {
		return err
	}

	var model qapicker.Model
	if name == "" {
		refs, err := qa.Discover(deps.FS, e.repoRoot)
		if err != nil {
			return err
		}
		switch len(refs) {
		case 0:
			return fmt.Errorf("no checklists in %s/checklists/ — add checklists/<name>.toml", e.repoRoot)
		case 1:
			name = refs[0].Stem
		default:
			model = qapicker.NewBrowse(deps.FS, refs, e.store, e.repoRoot, deps.Runner, styles.DefaultStyles())
		}
	}
	if name != "" {
		cl, err := e.loadChecklist(name)
		if err != nil {
			return err
		}
		run, err := e.store.Load(cl.Stem)
		if err != nil {
			return err
		}
		model = qapicker.New(cl, run, e.store, e.repoRoot, deps.Runner, styles.DefaultStyles())
	}

	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("qa picker: %w", err)
	}
	return nil
}

func newLsCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List checklists with scorecard summaries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := setup(deps)
			if err != nil {
				return err
			}
			refs, err := qa.Discover(deps.FS, e.repoRoot)
			if err != nil {
				return err
			}
			if len(refs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no checklists in %s/checklists/ — add checklists/<name>.toml\n", e.repoRoot)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CHECKLIST\tSTEPS\tSCORE\t")
			for _, ref := range refs {
				cl, issues, err := qa.Load(deps.FS, ref.Path)
				if err != nil || len(issues) > 0 {
					fmt.Fprintf(w, "%s\t-\tbroken — qa lint %s\t\n", ref.Stem, ref.Stem)
					continue
				}
				run, err := e.store.Load(cl.Stem)
				if err != nil {
					return err
				}
				note := ""
				if run.Stale(cl) {
					note = "STALE"
				}
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", cl.Stem, len(cl.Steps), summaryLine(qa.Summarize(cl, run)), note)
			}
			return w.Flush()
		},
	}
}

func newRunCmd(deps Deps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "run <checklist> [step...]",
		Short: "Execute checklist steps and record results",
		Long: `Without step args, sweeps every automatic step (cmd + check) in checklist
order. Named steps also run human-judged commands — their result lands as
'pending' until someone marks a verdict.

Exit code: 0 all pass · 1 any fail/error · 2 any pending/unrun.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := setup(deps)
			if err != nil {
				return err
			}
			cl, err := e.loadChecklist(args[0])
			if err != nil {
				return err
			}
			steps, err := selectRunSteps(cl, args[1:])
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(steps) == 0 {
				fmt.Fprintln(out, "no automatic steps — run named steps or use the picker / qa mark")
				return nil
			}

			// Seed from the persisted scorecard so `needs` sees marks from
			// earlier invocations (and the human picker), not just this sweep.
			run, err := e.store.Load(cl.Stem)
			if err != nil {
				return err
			}
			for _, s := range steps {
				warnUnmetNeeds(out, run, s)
				if s.Cmd == "" {
					fmt.Fprintf(out, "· %s — no cmd; check by hand, then: qa mark %s %s pass|fail\n", s.ID, cl.Stem, s.ID)
					continue
				}
				// A settled human verdict outranks a sweep — re-running would
				// silently stomp judgment with a prejudge. --force overrides.
				if prev, ok := stepRecord(run, s.ID); ok && !force &&
					prev.By == "human" && (prev.Result == qa.ResultPass || prev.Result == qa.ResultFail) {
					fmt.Fprintf(out, "%s %s — human-%s, kept (--force re-runs)\n",
						resultGlyph(prev.Result), s.ID, prev.Result)
					continue
				}
				rec := qa.ExecuteStep(deps.Runner, cl, s, e.repoRoot)
				rec.By = "agent"
				run, err = e.store.Mark(cl, s.ID, rec, force)
				if err != nil {
					return err
				}
				printStepResult(out, s, rec)
			}

			sum := qa.Summarize(cl, run)
			fmt.Fprintf(out, "\n%s: %s\n", cl.Stem, summaryLine(sum))
			return scorecardExit(sum)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "write results even when the checklist changed under the run")
	return cmd
}

// stepRecord looks up a step's persisted record; run may be nil (no
// scorecard yet).
func stepRecord(run *qa.Run, id string) (qa.StepRecord, bool) {
	if run == nil {
		return qa.StepRecord{}, false
	}
	rec, ok := run.Steps[id]
	return rec, ok
}

// selectRunSteps picks the steps a run sweeps: all automatic steps by
// default, or exactly the named ones.
func selectRunSteps(cl *qa.Checklist, names []string) ([]*qa.Step, error) {
	if len(names) == 0 {
		var autos []*qa.Step
		for i := range cl.Steps {
			if cl.Steps[i].Automatic() {
				autos = append(autos, &cl.Steps[i])
			}
		}
		return autos, nil
	}
	steps := make([]*qa.Step, 0, len(names))
	for _, name := range names {
		s := cl.StepByID(name)
		if s == nil {
			return nil, fmt.Errorf("no step %q in checklist %s", name, cl.Stem)
		}
		steps = append(steps, s)
	}
	return steps, nil
}

// warnUnmetNeeds flags soft prerequisites that haven't passed — warn,
// never block (ratified).
func warnUnmetNeeds(out io.Writer, run *qa.Run, s *qa.Step) {
	for _, need := range s.Needs {
		var rec qa.StepRecord
		if run != nil {
			rec = run.Steps[need]
		}
		if rec.Result != qa.ResultPass {
			fmt.Fprintf(out, "⚠ %s needs %s (currently %s)\n", s.ID, need, displayResult(rec.Result))
		}
	}
}

func displayResult(r qa.Result) string {
	if r == qa.ResultUnrun {
		return "unrun"
	}
	return string(r)
}

func printStepResult(out io.Writer, s *qa.Step, rec qa.StepRecord) {
	dur := ""
	if rec.Evidence != nil {
		dur = fmt.Sprintf(" (%s)", (time.Duration(rec.Evidence.DurationMS) * time.Millisecond).Round(10*time.Millisecond))
	}
	fmt.Fprintf(out, "%s %s — %s%s\n", resultGlyph(rec.Result), s.ID, displayResult(rec.Result), dur)
	if rec.Note != "" {
		fmt.Fprintf(out, "  %s\n", rec.Note)
	}
	// Evidence helps exactly when the step didn't pass.
	if rec.Result != qa.ResultPass && rec.Result != qa.ResultPending && rec.Evidence != nil && rec.Evidence.Output != "" {
		for _, line := range strings.Split(strings.TrimRight(rec.Evidence.Output, "\n"), "\n") {
			fmt.Fprintf(out, "  │ %s\n", line)
		}
	}
}

func newMarkCmd(deps Deps) *cobra.Command {
	var note string
	var force bool

	cmd := &cobra.Command{
		Use:   "mark <checklist> <step> <pass|fail>",
		Short: "Record a verdict on a step (CLI marks are by=agent)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			verdict := qa.Result(args[2])
			if verdict != qa.ResultPass && verdict != qa.ResultFail {
				return fmt.Errorf("verdict must be pass or fail, got %q", args[2])
			}
			e, err := setup(deps)
			if err != nil {
				return err
			}
			cl, err := e.loadChecklist(args[0])
			if err != nil {
				return err
			}
			rec := qa.StepRecord{Result: verdict, By: "agent", Note: note}
			run, err := e.store.Mark(cl, args[1], rec, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s/%s — %s\n%s\n",
				resultGlyph(verdict), cl.Stem, args[1], verdict, summaryLine(qa.Summarize(cl, run)))
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "context for the verdict (spoken human verdicts go here too)")
	cmd.Flags().BoolVar(&force, "force", false, "write even when the checklist changed under the run")
	return cmd
}

// statusJSON is the pinned v1 schema for `qa status --json`.
type statusJSON struct {
	Schema    int          `json:"schema"`
	Checklist string       `json:"checklist"`
	Name      string       `json:"name"`
	Path      string       `json:"path"`
	RepoRoot  string       `json:"repo_root"`
	Stale     bool         `json:"stale"`
	Started   *time.Time   `json:"started,omitempty"`
	Updated   *time.Time   `json:"updated_at,omitempty"`
	Summary   summaryJSON  `json:"summary"`
	Steps     []stepStatus `json:"steps"`
}

type summaryJSON struct {
	Pass    int `json:"pass"`
	Fail    int `json:"fail"`
	Error   int `json:"error"`
	Pending int `json:"pending"`
	Unrun   int `json:"unrun"`
}

type stepStatus struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	Expect    string       `json:"expect"`
	Automatic bool         `json:"automatic"`
	Result    string       `json:"result"` // unrun|pending|pass|fail|error
	By        string       `json:"by,omitempty"`
	Note      string       `json:"note,omitempty"`
	At        *time.Time   `json:"at,omitempty"`
	Evidence  *qa.Evidence `json:"evidence,omitempty"`
}

func newStatusCmd(deps Deps) *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "status <checklist>",
		Short: "Show the scorecard for a checklist",
		Long:  "Exit code: 0 all pass · 1 any fail/error · 2 any pending/unrun.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := setup(deps)
			if err != nil {
				return err
			}
			cl, err := e.loadChecklist(args[0])
			if err != nil {
				return err
			}
			run, err := e.store.Load(cl.Stem)
			if err != nil {
				return err
			}
			sum := qa.Summarize(cl, run)
			out := cmd.OutOrStdout()

			if jsonOut {
				doc := statusJSON{
					Schema:    1,
					Checklist: cl.Stem,
					Name:      cl.Name,
					Path:      cl.Path,
					RepoRoot:  e.repoRoot,
					Stale:     run.Stale(cl),
					Summary:   summaryJSON{sum.Pass, sum.Fail, sum.Error, sum.Pending, sum.Unrun},
				}
				if run != nil {
					doc.Started, doc.Updated = &run.Started, &run.Updated
				}
				for i := range cl.Steps {
					s := &cl.Steps[i]
					st := stepStatus{
						ID: s.ID, Name: s.Name, Expect: s.Expect,
						Automatic: s.Automatic(), Result: "unrun",
					}
					if run != nil {
						if rec, ok := run.Steps[s.ID]; ok {
							st.Result = displayResult(rec.Result)
							st.By, st.Note, st.Evidence = rec.By, rec.Note, rec.Evidence
							at := rec.At
							st.At = &at
						}
					}
					doc.Steps = append(doc.Steps, st)
				}
				enc, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(enc))
				return scorecardExit(sum)
			}

			fmt.Fprintf(out, "%s — %s\n", cl.Stem, cl.Name)
			if run.Stale(cl) {
				fmt.Fprintln(out, "⚠ STALE: checklist changed since this run started — qa reset, or --force to keep mixing")
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for i := range cl.Steps {
				s := &cl.Steps[i]
				var rec qa.StepRecord
				if run != nil {
					rec = run.Steps[s.ID]
				}
				by := rec.By
				if rec.Note != "" {
					by += " — " + rec.Note
				}
				fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", resultGlyph(rec.Result), s.ID, s.Name, displayResult(rec.Result), by)
			}
			w.Flush()
			fmt.Fprintf(out, "\n%s\n", summaryLine(sum))
			return scorecardExit(sum)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable scorecard (schema v1)")
	return cmd
}

func newResetCmd(deps Deps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reset <checklist>",
		Short: "Discard the scorecard and start a fresh run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := setup(deps)
			if err != nil {
				return err
			}
			cl, err := e.loadChecklist(args[0])
			if err != nil {
				return err
			}
			run, err := e.store.Load(cl.Stem)
			if err != nil {
				return err
			}
			if run != nil && !force {
				// Fails and notes are findings — don't silently wipe them.
				for id, rec := range run.Steps {
					if rec.Result == qa.ResultFail || rec.Result == qa.ResultError || rec.Note != "" {
						return fmt.Errorf("scorecard has findings (e.g. %s: %s) — pass --force to discard them", id, displayResult(rec.Result))
					}
				}
			}
			if _, err := e.store.Reset(cl); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reset %s — fresh run bound to current checklist\n", cl.Stem)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "discard a scorecard that has fails or notes")
	return cmd
}

func newLintCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "lint [checklist]",
		Short: "Validate checklist files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := setup(deps)
			if err != nil {
				return err
			}
			refs, err := qa.Discover(deps.FS, e.repoRoot)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				path, err := qa.Resolve(deps.FS, e.repoRoot, args[0])
				if err != nil {
					return err
				}
				refs = []qa.Ref{{Path: path, Stem: strings.TrimSuffix(filepath.Base(path), ".toml")}}
			}

			out := cmd.OutOrStdout()
			var steps int
			issues := qa.LintRefs(refs)
			for _, ref := range refs {
				cl, clIssues, err := qa.Load(deps.FS, ref.Path)
				if err != nil {
					issues = append(issues, fmt.Sprintf("%s: %v", ref.Path, err))
					continue
				}
				for _, is := range clIssues {
					issues = append(issues, fmt.Sprintf("%s: %s", cl.Stem, is))
				}
				steps += len(cl.Steps)
			}

			if len(issues) > 0 {
				for _, is := range issues {
					fmt.Fprintln(out, is)
				}
				return &codedError{code: 1, msg: fmt.Sprintf("%d issue(s)", len(issues))}
			}
			fmt.Fprintf(out, "ok: %d checklist(s), %d step(s)\n", len(refs), steps)
			return nil
		},
	}
}
