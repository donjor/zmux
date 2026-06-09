// Package qapicker is the human frontend of the QA walkthrough framework
// (plan 028): it drives a committed checklist step by step, runs step
// commands non-interactively, and persists verdicts (by=human) to the
// same scorecard the agent CLI verbs write.
package qapicker

import (
	"errors"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/qa"
	"github.com/donjor/zmux/internal/tui/styles"
)

type mode int

const (
	modeList mode = iota
	modeRunning
	modeVerdict
	modeResetConfirm
	modeChecklists // browse: pick a checklist before stepping
	modeNote       // verdict overlay: editing the step note
)

var keys = struct {
	Quit  key.Binding
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Pass  key.Binding
	Fail  key.Binding
	Note  key.Binding
	Reset key.Binding
	Back  key.Binding
}{
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "down")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run step")),
	Pass:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pass")),
	Fail:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fail")),
	Note:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "note")),
	Reset: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reset run")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

// stepDoneMsg carries an executed step's derived record back to Update.
type stepDoneMsg struct {
	id  string
	rec qa.StepRecord
}

// Model is the qa picker bubbletea model.
type Model struct {
	cl       *qa.Checklist
	store    *qa.Store
	repoRoot string
	runner   qa.CmdRunner
	styles   styles.Styles

	run    *qa.Run
	cursor int
	mode   mode
	errMsg string

	// Browse state (bare invocation): pick a checklist first; esc from
	// the step list returns here instead of quitting.
	fs       config.FS
	refs     []qa.Ref
	clCursor int

	// Verdict overlay state: the step just acted on and its (possibly
	// already persisted) record.
	verdictID  string
	verdictRec qa.StepRecord
	persisted  bool // record already on the scorecard (cmd ran)

	// Note editing (n in the verdict overlay). noteDirty marks a staged
	// note that hasn't reached the scorecard yet, so esc-keep on a
	// persisted record can re-mark instead of silently dropping it.
	noteInput textinput.Model
	noteDirty bool

	width, height int

	Quitting bool
}

// New builds the picker straight into one checklist. run may be nil (no
// scorecard yet).
func New(cl *qa.Checklist, run *qa.Run, store *qa.Store, repoRoot string, runner qa.CmdRunner, st styles.Styles) Model {
	return Model{cl: cl, run: run, store: store, repoRoot: repoRoot, runner: runner, styles: st}
}

// NewBrowse builds the picker in checklist-select mode.
func NewBrowse(fs config.FS, refs []qa.Ref, store *qa.Store, repoRoot string, runner qa.CmdRunner, st styles.Styles) Model {
	return Model{
		fs: fs, refs: refs, mode: modeChecklists,
		store: store, repoRoot: repoRoot, runner: runner, styles: st,
	}
}

// openChecklist loads a checklist and its run, entering the step list.
func (m Model) openChecklist(ref qa.Ref) Model {
	cl, issues, err := qa.Load(m.fs, ref.Path)
	if err != nil {
		m.errMsg = err.Error()
		return m
	}
	if len(issues) > 0 {
		m.errMsg = ref.Stem + " has lint issues (qa lint " + ref.Stem + ")"
		return m
	}
	run, err := m.store.Load(cl.Stem)
	if err != nil {
		m.errMsg = err.Error()
		return m
	}
	m.cl, m.run = cl, run
	m.cursor = 0
	m.errMsg = ""
	m.mode = modeList
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// step returns the checklist step under the cursor.
func (m Model) step() *qa.Step {
	if m.cursor < 0 || m.cursor >= len(m.cl.Steps) {
		return nil
	}
	return &m.cl.Steps[m.cursor]
}

// record returns the scorecard record for a step id (zero when unrun).
func (m Model) record(id string) qa.StepRecord {
	if m.run == nil {
		return qa.StepRecord{}
	}
	return m.run.Steps[id]
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case stepDoneMsg:
		return m.handleStepDone(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeVerdict:
		return m.handleVerdictKey(msg)
	case modeNote:
		return m.handleNoteKey(msg)
	case modeResetConfirm:
		return m.handleResetKey(msg)
	case modeChecklists:
		return m.handleChecklistKey(msg)
	case modeRunning:
		// Only quit escapes a running command.
		if key.Matches(msg, keys.Quit) {
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// List mode.
	switch {
	case key.Matches(msg, keys.Quit):
		m.Quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		// Browsing? Back to the checklist select instead of out.
		if len(m.refs) > 0 {
			m.mode = modeChecklists
			m.errMsg = ""
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, keys.Down):
		if m.cursor < len(m.cl.Steps)-1 {
			m.cursor++
		}
	case key.Matches(msg, keys.Reset):
		m.mode = modeResetConfirm
	case key.Matches(msg, keys.Enter):
		return m.actOnStep()
	}
	return m, nil
}

func (m Model) handleChecklistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Back):
		m.Quitting = true
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		if m.clCursor > 0 {
			m.clCursor--
		}
	case key.Matches(msg, keys.Down):
		if m.clCursor < len(m.refs)-1 {
			m.clCursor++
		}
	case key.Matches(msg, keys.Enter):
		if m.clCursor < len(m.refs) {
			return m.openChecklist(m.refs[m.clCursor]), nil
		}
	}
	return m, nil
}

// actOnStep runs the current step's command (async), or jumps straight
// to the verdict prompt for instruction-only steps.
func (m Model) actOnStep() (tea.Model, tea.Cmd) {
	s := m.step()
	if s == nil {
		return m, nil
	}
	m.errMsg = ""
	if s.Cmd == "" {
		m.mode = modeVerdict
		m.verdictID = s.ID
		m.verdictRec = m.record(s.ID)
		m.persisted = false
		m.noteDirty = false
		return m, nil
	}
	m.mode = modeRunning
	id := s.ID
	cl, runner, root := m.cl, m.runner, m.repoRoot
	return m, func() tea.Msg {
		return stepDoneMsg{id: id, rec: qa.ExecuteStep(runner, cl, cl.StepByID(id), root)}
	}
}

// handleStepDone persists the derived record immediately (the contract:
// auto → pass/fail, human-judged → pending, broken → error), then opens
// the verdict overlay so the human can override the prejudge.
//
// The prejudge is attributed "auto", not "human" — the regexp matched, no
// one judged anything. An explicit p/f in the overlay upgrades it to
// by=human (and only human verdicts block agent re-sweeps in `qa run`).
func (m Model) handleStepDone(msg stepDoneMsg) (tea.Model, tea.Cmd) {
	msg.rec.By = "auto"
	run, err := m.store.Mark(m.cl, msg.id, msg.rec, false)
	if err != nil {
		m.mode = modeList
		m.errMsg = err.Error()
		if errors.Is(err, qa.ErrStale) {
			m.errMsg = "checklist changed under this run — R to reset (findings are discarded)"
		}
		return m, nil
	}
	m.run = run
	m.mode = modeVerdict
	m.verdictID = msg.id
	m.verdictRec = msg.rec
	m.persisted = true
	m.noteDirty = false
	return m, nil
}

func (m Model) handleVerdictKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Pass):
		return m.mark(qa.ResultPass, "human")
	case key.Matches(msg, keys.Fail):
		return m.mark(qa.ResultFail, "human")
	case key.Matches(msg, keys.Note):
		ti := textinput.New()
		ti.Placeholder = "note for this step"
		ti.CharLimit = 240
		ti.SetWidth(noteInputWidth(m.width))
		ti.SetValue(m.verdictRec.Note)
		ti.Focus()
		m.noteInput = ti
		m.mode = modeNote
		return m, textinput.Blink
	case key.Matches(msg, keys.Back), key.Matches(msg, keys.Enter):
		// Keep whatever the execution persisted (pending/pass/fail/error);
		// an unrun instruction-only step stays unrun. A staged note on a
		// persisted record is saved, not dropped — keeping its existing
		// attribution (a note alone is not a verdict).
		if m.persisted && m.noteDirty {
			return m.mark(m.verdictRec.Result, m.verdictRec.By)
		}
		m.mode = modeList
		return m, nil
	case key.Matches(msg, keys.Quit):
		m.Quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleNoteKey edits the staged note: enter stages it, esc cancels the
// edit. Persistence happens with the verdict (or esc-keep) in modeVerdict.
func (m Model) handleNoteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Enter):
		if v := m.noteInput.Value(); v != m.verdictRec.Note {
			m.verdictRec.Note = v
			m.noteDirty = true
		}
		m.mode = modeVerdict
		return m, nil
	case key.Matches(msg, keys.Back):
		m.mode = modeVerdict
		return m, nil
	case msg.String() == "ctrl+c": // plain q types into the note
		m.Quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.noteInput, cmd = m.noteInput.Update(msg)
	return m, cmd
}

// noteInputWidth mirrors the expect box width so the overlay lines up.
func noteInputWidth(w int) int {
	w -= 4
	if w < 20 || w > 76 {
		return 76
	}
	return w
}

// mark records a verdict, carrying execution evidence along. by is
// "human" for explicit p/f; the esc-keep note-save path preserves the
// record's existing attribution.
func (m Model) mark(result qa.Result, by string) (tea.Model, tea.Cmd) {
	rec := m.verdictRec
	rec.Result = result
	rec.By = by
	run, err := m.store.Mark(m.cl, m.verdictID, rec, false)
	if err != nil {
		m.mode = modeList
		m.errMsg = err.Error()
		if errors.Is(err, qa.ErrStale) {
			m.errMsg = "checklist changed under this run — R to reset (findings are discarded)"
		}
		return m, nil
	}
	m.run = run
	m.mode = modeList
	m.errMsg = ""
	m.noteDirty = false
	return m, nil
}

func (m Model) handleResetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		run, err := m.store.Reset(m.cl)
		if err != nil {
			m.errMsg = err.Error()
		} else {
			m.run = run
			m.errMsg = ""
		}
		m.mode = modeList
	case "n", "N", "esc", "q":
		m.mode = modeList
	}
	return m, nil
}
