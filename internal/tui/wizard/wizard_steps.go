package wizard

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// Per-step key handlers. Each function owns the keymap for exactly one
// step and returns the (possibly transitioned) model plus an optional
// command. Dispatching from handleKey happens in wizard.go.

func (m WizardModel) handleWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepDepCheck
		return m, m.detectDeps
	}
	return m, nil
}

func (m WizardModel) handleDepCheck(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepDetectTargets
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleDetectTargets(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepTheme
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleThemeStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.themeCursor > 0 {
			m.themeCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.themeCursor < len(m.themes)-1 {
			m.themeCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.themeCursor < len(m.themes) {
			m.chosenTheme = m.themes[m.themeCursor].Name
		}
		m.step = stepBarPreset
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleBarPreset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.presetCursor > 0 {
			m.presetCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.presetCursor < len(m.presets)-1 {
			m.presetCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.presetCursor < len(m.presets) {
			m.chosenPreset = m.presets[m.presetCursor].String()
		}
		m.step = stepSyncTarget
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleSyncTarget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.syncCursor > 0 {
			m.syncCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.syncCursor < len(m.syncTargets)-1 {
			m.syncCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.syncCursor < len(m.syncTargets) {
			m.chosenSync = m.syncTargets[m.syncCursor]
		}
		m.step = stepSummary
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepWriting
		return m, m.writeConfig
	}
	return m, nil
}

// handleSuccess handles the final step — enter/q exits, c/y copies the
// restart command to the clipboard.
func (m WizardModel) handleSuccess(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) || msg.String() == "q" {
		return m, tea.Quit
	}
	if (msg.String() == "c" || msg.String() == "y") && !m.Copied {
		cmd := restartCmd(m.profile)
		if err := copyToClipboard(cmd); err == nil {
			m.Copied = true
		}
		return m, nil
	}
	return m, nil
}
