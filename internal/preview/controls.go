package preview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── ChoiceControl ──

// ChoiceControl picks one value from a fixed option list. Left/right
// arrow or h/l cycles.
type ChoiceControl struct {
	id      ControlID
	label   string
	options []string
	cursor  int
}

// NewChoice builds a ChoiceControl. Default selection is the first option.
func NewChoice(id ControlID, label string, options []string, initial string) *ChoiceControl {
	c := &ChoiceControl{id: id, label: label, options: options}
	for i, o := range options {
		if o == initial {
			c.cursor = i
			break
		}
	}
	return c
}

func (c *ChoiceControl) ID() ControlID { return c.id }
func (c *ChoiceControl) Label() string { return c.label }
func (c *ChoiceControl) Value() any {
	if len(c.options) == 0 {
		return ""
	}
	return c.options[c.cursor]
}

func (c *ChoiceControl) Handle(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "left", "h":
		c.cursor = (c.cursor - 1 + len(c.options)) % len(c.options)
		return true
	case "right", "l", "space":
		c.cursor = (c.cursor + 1) % len(c.options)
		return true
	}
	return false
}

func (c *ChoiceControl) View(focused bool) string {
	current := ""
	if len(c.options) > 0 {
		current = c.options[c.cursor]
	}
	prefix := MuteStyle.Render("  ")
	label := MuteStyle.Render(fmt.Sprintf("%-12s", c.label+":"))
	valueStyle := lipgloss.NewStyle().Foreground(Teal).Bold(true)
	if focused {
		prefix = lipgloss.NewStyle().Foreground(Gold).Bold(true).Render("▸ ")
		label = lipgloss.NewStyle().Foreground(FG).Bold(true).Render(fmt.Sprintf("%-12s", c.label+":"))
		valueStyle = lipgloss.NewStyle().Foreground(Gold).Bold(true)
	}
	position := ""
	if len(c.options) > 1 {
		position = MuteStyle.Render(fmt.Sprintf("  %d/%d", c.cursor+1, len(c.options)))
	}
	return fmt.Sprintf("%s%s %s%s", prefix, label, valueStyle.Render(current), position)
}

// ── IntControl ──

// IntControl picks an integer in [min,max]. Left/right or h/l adjusts.
type IntControl struct {
	id       ControlID
	label    string
	min, max int
	val      int
	suffix   string // optional trailing label (e.g. "sessions")
}

func NewInt(id ControlID, label string, min, max, initial int, suffix string) *IntControl {
	return &IntControl{id: id, label: label, min: min, max: max, val: clamp(initial, min, max), suffix: suffix}
}

func (c *IntControl) ID() ControlID { return c.id }
func (c *IntControl) Label() string { return c.label }
func (c *IntControl) Value() any    { return c.val }

func (c *IntControl) SetMax(max int) {
	c.max = max
	if c.val > max {
		c.val = max
	}
}

func (c *IntControl) Handle(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "left", "h", "-":
		if c.val > c.min {
			c.val--
		}
		return true
	case "right", "l", "+", "space":
		if c.val < c.max {
			c.val++
		}
		return true
	}
	return false
}

func (c *IntControl) View(focused bool) string {
	prefix := MuteStyle.Render("  ")
	label := MuteStyle.Render(fmt.Sprintf("%-12s", c.label+":"))
	if focused {
		prefix = lipgloss.NewStyle().Foreground(Gold).Bold(true).Render("▸ ")
		label = lipgloss.NewStyle().Foreground(FG).Bold(true).Render(fmt.Sprintf("%-12s", c.label+":"))
	}
	suffix := c.suffix
	if suffix != "" {
		suffix = " " + suffix
	}
	val := fmt.Sprintf("%d%s", c.val, suffix)
	rangeText := MuteStyle.Render(fmt.Sprintf("(%d–%d)", c.min, c.max))
	bar := intBar(c.val-c.min, c.max-c.min, 10)
	if focused {
		val = controlPill(val, true)
	} else {
		val = lipgloss.NewStyle().Foreground(Teal).Bold(true).Render(val)
	}
	return fmt.Sprintf("%s%s %s  %s  %s", prefix, label, val, bar, rangeText)
}

// ── ToggleControl ──

// ToggleControl flips a bool. Space / enter / left / right toggles.
type ToggleControl struct {
	id    ControlID
	label string
	val   bool
}

func NewToggle(id ControlID, label string, initial bool) *ToggleControl {
	return &ToggleControl{id: id, label: label, val: initial}
}

func (c *ToggleControl) ID() ControlID { return c.id }
func (c *ToggleControl) Label() string { return c.label }
func (c *ToggleControl) Value() any    { return c.val }

func (c *ToggleControl) Handle(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "space", "enter", "left", "right", "h", "l":
		c.val = !c.val
		return true
	}
	return false
}

func (c *ToggleControl) View(focused bool) string {
	prefix := MuteStyle.Render("  ")
	label := MuteStyle.Render(fmt.Sprintf("%-12s", c.label+":"))
	if focused {
		prefix = lipgloss.NewStyle().Foreground(Gold).Bold(true).Render("▸ ")
		label = lipgloss.NewStyle().Foreground(FG).Bold(true).Render(fmt.Sprintf("%-12s", c.label+":"))
	}
	stateStyle := lipgloss.NewStyle().Foreground(Dim)
	glyph := "○"
	text := "off"
	if c.val {
		stateStyle = lipgloss.NewStyle().Foreground(Green).Bold(true)
		glyph = "●"
		text = "on"
	}
	if focused {
		stateStyle = lipgloss.NewStyle().Foreground(Gold).Bold(true)
	}
	return fmt.Sprintf("%s%s %s", prefix, label, stateStyle.Render(glyph+" "+text))
}

// ── helpers ──

func controlPill(s string, focused bool) string {
	style := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	if focused {
		return style.Foreground(BGDark).Background(Gold).Render(s)
	}
	return style.Foreground(BGDark).Background(Teal).Render(s)
}

func intBar(v, max int, width int) string {
	if max <= 0 {
		max = 1
	}
	filled := v * width / max
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return lipgloss.NewStyle().Foreground(Blue).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(Dim).Render(strings.Repeat("░", width-filled))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
