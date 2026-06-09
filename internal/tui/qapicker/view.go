package qapicker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/qa"
)

// outputTailLines bounds the captured-output box in the verdict overlay.
const outputTailLines = 12

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

func (m Model) glyphStyle(r qa.Result) lipgloss.Style {
	switch r {
	case qa.ResultPass:
		return m.styles.Success
	case qa.ResultFail, qa.ResultError:
		return m.styles.Error
	case qa.ResultPending:
		return m.styles.Special
	default:
		return m.styles.Dim
	}
}

func (m Model) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m Model) view() string {
	if m.Quitting {
		return ""
	}
	var b strings.Builder

	// Header.
	if m.mode == modeChecklists {
		b.WriteString(m.styles.Title.Render("qa") + m.styles.Muted.Render(" — pick a checklist") + "\n")
	} else {
		b.WriteString(m.styles.Title.Render("qa") + m.styles.Muted.Render(" — "+m.cl.Stem))
		if m.cl.Name != "" {
			b.WriteString(m.styles.Muted.Render("  " + m.cl.Name))
		}
		b.WriteString("\n")
		if m.run.Stale(m.cl) {
			b.WriteString(m.styles.Error.Render("⚠ checklist changed since this run started — R resets the scorecard") + "\n")
		}
	}
	if m.errMsg != "" {
		b.WriteString(m.styles.Error.Render("✗ "+m.errMsg) + "\n")
	}
	b.WriteString("\n")

	switch m.mode {
	case modeVerdict:
		b.WriteString(m.viewVerdict())
	case modeNote:
		b.WriteString(m.viewNote())
	case modeResetConfirm:
		b.WriteString(m.viewResetConfirm())
	case modeChecklists:
		b.WriteString(m.viewChecklists())
	default:
		b.WriteString(m.viewList())
	}

	// Live scorecard footer (only once a checklist is open).
	b.WriteString("\n")
	if m.mode != modeChecklists {
		sum := qa.Summarize(m.cl, m.run)
		score := fmt.Sprintf("✓%d ✗%d !%d ⏳%d ·%d", sum.Pass, sum.Fail, sum.Error, sum.Pending, sum.Unrun)
		b.WriteString(m.styles.Normal.Render(score) + "  ")
	}
	b.WriteString(m.help() + "\n")
	return b.String()
}

// viewChecklists renders the browse screen: every committed checklist
// with its live scorecard summary.
func (m Model) viewChecklists() string {
	var b strings.Builder
	for i, ref := range m.refs {
		cursor := "  "
		nameStyle := m.styles.Normal
		if i == m.clCursor {
			cursor = m.styles.Accent.Render("> ")
			nameStyle = m.styles.Selected
		}
		line := cursor + nameStyle.Render(ref.Stem)
		if cl, issues, err := qa.Load(m.fs, ref.Path); err != nil || len(issues) > 0 {
			line += m.styles.Error.Render("  broken — qa lint " + ref.Stem)
		} else {
			run, _ := m.store.Load(cl.Stem)
			sum := qa.Summarize(cl, run)
			line += m.styles.Dim.Render(fmt.Sprintf("  %d steps  ✓%d ✗%d !%d ⏳%d ·%d",
				len(cl.Steps), sum.Pass, sum.Fail, sum.Error, sum.Pending, sum.Unrun))
			if run.Stale(cl) {
				line += m.styles.Error.Render("  STALE")
			}
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m Model) viewList() string {
	var b strings.Builder
	for i := range m.cl.Steps {
		s := &m.cl.Steps[i]
		rec := m.record(s.ID)

		cursor := "  "
		nameStyle := m.styles.Normal
		if i == m.cursor {
			cursor = m.styles.Accent.Render("> ")
			nameStyle = m.styles.Selected
		}
		if m.mode == modeRunning && i == m.cursor {
			cursor = m.styles.Special.Render("… ")
		}

		glyph := m.glyphStyle(rec.Result).Render(resultGlyph(rec.Result))
		label := s.Name
		if label == "" {
			label = s.ID
		}
		line := cursor + glyph + " " + nameStyle.Render(label)
		if rec.By != "" {
			line += m.styles.Dim.Render("  [" + rec.By + "]")
		}
		if rec.Note != "" {
			line += m.styles.Dim.Render(" — " + rec.Note)
		}
		b.WriteString(line + "\n")
	}

	// EXPECT box + command hint for the selected step.
	if s := m.step(); s != nil {
		b.WriteString("\n" + m.expectBox(s))
		if s.Cmd != "" {
			cmd, err := m.cl.Command(s)
			if err == nil {
				b.WriteString("\n" + m.styles.Dim.Render("$ "+cmd))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// expectBox renders the step's expectation in a bordered box — the
// thing the human is judging against.
func (m Model) expectBox(s *qa.Step) string {
	w := m.width - 4
	if w < 20 || w > 76 {
		w = 76
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Accent.GetForeground()).
		Padding(0, 1).
		Width(w)
	return box.Render(m.styles.Subtitle.Render("EXPECT  ") + s.Expect)
}

func (m Model) viewVerdict() string {
	var b strings.Builder
	s := m.cl.StepByID(m.verdictID)
	if s == nil {
		return ""
	}
	label := s.Name
	if label == "" {
		label = s.ID
	}
	b.WriteString(m.styles.Selected.Render(label) + "\n\n")
	b.WriteString(m.expectBox(s) + "\n")

	if ev := m.verdictRec.Evidence; ev != nil {
		b.WriteString(m.styles.Dim.Render(fmt.Sprintf("$ %s  (exit %d, %dms)", ev.Command, ev.Exit, ev.DurationMS)) + "\n")
		out := strings.TrimRight(ev.Output, "\n")
		if out != "" {
			lines := strings.Split(out, "\n")
			if len(lines) > outputTailLines {
				b.WriteString(m.styles.Dim.Render(fmt.Sprintf("  … %d earlier lines", len(lines)-outputTailLines)) + "\n")
				lines = lines[len(lines)-outputTailLines:]
			}
			for _, line := range lines {
				b.WriteString("  " + line + "\n")
			}
		}
	}
	if m.verdictRec.Note != "" {
		// Runner notes explain errors; human notes are plain commentary.
		noteStyle := m.styles.Dim
		if m.verdictRec.Result == qa.ResultError {
			noteStyle = m.styles.Error
		}
		b.WriteString(noteStyle.Render("» "+m.verdictRec.Note) + "\n")
	}

	// Prejudge: what execution already put on the scorecard.
	b.WriteString("\n")
	if m.persisted {
		glyph := m.glyphStyle(m.verdictRec.Result).Render(resultGlyph(m.verdictRec.Result))
		fmt.Fprintf(&b, "recorded as %s %s — override?  ", glyph, displayResult(m.verdictRec.Result))
	} else {
		b.WriteString("verdict?  ")
	}
	b.WriteString(m.styles.Help.Render("p:pass  f:fail  n:note  esc:keep") + "\n")
	return b.String()
}

// viewNote renders the verdict overlay's note editor.
func (m Model) viewNote() string {
	var b strings.Builder
	s := m.cl.StepByID(m.verdictID)
	if s == nil {
		return ""
	}
	label := s.Name
	if label == "" {
		label = s.ID
	}
	b.WriteString(m.styles.Selected.Render(label) + "\n\n")
	b.WriteString(m.expectBox(s) + "\n\n")
	b.WriteString(m.styles.Subtitle.Render("NOTE  ") + m.noteInput.View() + "\n")
	return b.String()
}

func (m Model) viewResetConfirm() string {
	return m.styles.Error.Render("discard this scorecard and start fresh?") +
		"  " + m.styles.Help.Render("y:reset  n:cancel") + "\n"
}

func (m Model) help() string {
	switch m.mode {
	case modeVerdict:
		return m.styles.Help.Render("p:pass  f:fail  n:note  esc:keep")
	case modeNote:
		return m.styles.Help.Render("enter:stage note  esc:cancel")
	case modeResetConfirm:
		return m.styles.Help.Render("y:reset  n:cancel")
	case modeRunning:
		return m.styles.Help.Render("running…")
	case modeChecklists:
		return m.styles.Help.Render("enter:open  q:quit")
	default:
		if len(m.refs) > 0 {
			return m.styles.Help.Render("enter:run  p/f after run  R:reset  esc:back  q:quit")
		}
		return m.styles.Help.Render("enter:run  p/f after run  R:reset  q:quit")
	}
}

func displayResult(r qa.Result) string {
	if r == qa.ResultUnrun {
		return "unrun"
	}
	return string(r)
}
