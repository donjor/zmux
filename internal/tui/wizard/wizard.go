// Package tui — init wizard.
//
// The wizard is a linear 9-step bubbletea flow: welcome → dep check →
// detected environment → theme → bar preset → sync target → summary →
// writing (async) → success. Shared state lives on WizardModel.
//
// This file owns the container: the model struct, consts, keymap,
// Init/Update/View dispatchers, and typed accessors for test code.
// Per-step key handlers live in wizard_steps.go; per-step view
// renderers live in wizard_views.go; the two async commands
// (detectDeps + writeConfig) and helper functions live in
// wizard_data.go.

package wizard

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/styles"
)

// wizardStep identifies the current step in the init wizard.
type wizardStep int

const (
	stepWelcome wizardStep = iota
	stepDepCheck
	stepDetectTargets
	stepTheme
	stepBarPreset
	stepSyncTarget
	stepSummary
	stepWriting
	stepSuccess
)

// wizardKeys defines keybindings for the init wizard.
var wizardKeys = struct {
	Quit    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Up      key.Binding
	Down    key.Binding
	TabBack key.Binding
}{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "next"),
	),
	Back: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "back"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	TabBack: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "back"),
	),
}

// WizardDeps holds detected dependency information surfaced on the
// dep-check + detected-targets screens.
type WizardDeps struct {
	TmuxVersion   string
	ClipboardTool string
	HasGhostty    bool
	HasNvim       bool
	GhosttyPath   string
}

// WizardModel is the bubbletea model for the zmux init wizard.
type WizardModel struct {
	step    wizardStep
	styles  styles.Styles
	width   int
	height  int
	version string
	fs      config.FS
	profile config.Profile

	// Dependency detection
	deps WizardDeps

	// Choices
	themes      []theme.ThemeInfo
	themeCursor int
	chosenTheme string

	presets      []bar.Preset
	presetCursor int
	chosenPreset string

	syncTargets []string
	syncCursor  int
	chosenSync  string

	// Result
	Cancelled bool
	Done      bool
	Error     error
	Copied    bool // whether the user copied the restart command

	// For rendering previews
	resolver *theme.Resolver
}

// NewWizardModel creates a new init wizard model. The default theme is
// ayu-dark if present in the resolver's list, otherwise the first
// available theme.
func NewWizardModel(fs config.FS, resolver *theme.Resolver, ver string, styles styles.Styles, profile config.Profile) WizardModel {
	themes := resolver.List()

	// Default to ayu-dark if available.
	themeCursor := 0
	for i, ti := range themes {
		if ti.Name == "ayu-dark" {
			themeCursor = i
			break
		}
	}

	presets := bar.AllPresets()

	return WizardModel{
		step:         stepWelcome,
		styles:       styles,
		version:      ver,
		fs:           fs,
		profile:      profile,
		resolver:     resolver,
		themes:       themes,
		themeCursor:  themeCursor,
		chosenTheme:  "ayu-dark",
		presets:      presets,
		presetCursor: 0,
		chosenPreset: "default",
		syncTargets:  []string{"none"},
		syncCursor:   0,
		chosenSync:   "none",
	}
}

// Init is a no-op — detection runs on enter from the welcome screen,
// not on model creation, so tests can walk the flow deterministically.
func (m WizardModel) Init() tea.Cmd { return nil }

// Update handles messages and user input.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case detectDepsMsg:
		m.deps = msg.deps
		// Build sync targets based on detection.
		m.syncTargets = []string{"none"}
		if m.deps.HasGhostty {
			m.syncTargets = append(m.syncTargets, "ghostty")
		}
		if m.deps.HasNvim {
			m.syncTargets = append(m.syncTargets, "nvim")
		}
		return m, nil

	case configWrittenMsg:
		m.step = stepSuccess
		m.Done = true
		return m, nil

	case configWriteErrMsg:
		m.Error = msg.err
		m.step = stepSuccess
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey dispatches a key event to the appropriate per-step
// handler. Ctrl+C always cancels; back-nav steps one backwards on
// screens where it's allowed.
func (m WizardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Cancel at any point.
	if key.Matches(msg, wizardKeys.Quit) {
		m.Cancelled = true
		return m, tea.Quit
	}

	// Back navigation — welcome is the first step; writing + success
	// are non-reversible.
	if (key.Matches(msg, wizardKeys.Back) || key.Matches(msg, wizardKeys.TabBack)) &&
		m.canNavigateBack() {
		m.step--
		return m, nil
	}

	switch m.step {
	case stepWelcome:
		return m.handleWelcome(msg)
	case stepDepCheck:
		return m.handleDepCheck(msg)
	case stepDetectTargets:
		return m.handleDetectTargets(msg)
	case stepTheme:
		return m.handleThemeStep(msg)
	case stepBarPreset:
		return m.handleBarPreset(msg)
	case stepSyncTarget:
		return m.handleSyncTarget(msg)
	case stepSummary:
		return m.handleSummary(msg)
	case stepSuccess:
		return m.handleSuccess(msg)
	}

	return m, nil
}

// canNavigateBack returns true if shift+tab / 'b' should step
// backwards. Welcome is the first step so can't go back; writing +
// success are non-reversible.
func (m WizardModel) canNavigateBack() bool {
	return m.step > stepWelcome && m.step < stepWriting
}

// View renders the wizard UI by delegating to the per-step view
// function and wrapping with progress header + help bar.
func (m WizardModel) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m WizardModel) view() string {
	if m.Cancelled {
		return ""
	}

	// Step labels for the "[N/M] <Label>" progress header. Ordered to
	// match the wizardStep iota.
	stepNames := []string{
		"Welcome", "Dependencies", "Environment", "Theme",
		"Status Bar", "Sync Target", "Summary", "Writing...", "Done",
	}
	currentName := "Unknown"
	if int(m.step) < len(stepNames) {
		currentName = stepNames[m.step]
	}

	progress := m.styles.Dim.Render(fmt.Sprintf("  [%d/%d] %s", m.step+1, len(stepNames), currentName))

	var body string
	switch m.step {
	case stepWelcome:
		body = m.viewWelcome()
	case stepDepCheck:
		body = m.viewDepCheck()
	case stepDetectTargets:
		body = m.viewDetectTargets()
	case stepTheme:
		body = m.viewTheme()
	case stepBarPreset:
		body = m.viewBarPreset()
	case stepSyncTarget:
		body = m.viewSyncTarget()
	case stepSummary:
		body = m.viewSummary()
	case stepWriting:
		body = m.viewWriting()
	case stepSuccess:
		body = m.viewSuccess()
	}

	return progress + "\n\n" + body + "\n" + m.viewHelp()
}

// ── Typed accessors for tests ──

// Step returns the current wizard step.
func (m WizardModel) Step() wizardStep { return m.step }

// ChosenTheme returns the theme selected by the user.
func (m WizardModel) ChosenTheme() string { return m.chosenTheme }

// ChosenPreset returns the bar preset selected by the user.
func (m WizardModel) ChosenPreset() string { return m.chosenPreset }

// ChosenSync returns the sync target selected by the user.
func (m WizardModel) ChosenSync() string { return m.chosenSync }
