package wizard

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/views"
)

// Per-step view renderers. Each function renders exactly one step; the
// top-level View() in wizard.go picks which one to call based on
// m.step and wraps the result with the progress header and help bar.

func (m WizardModel) viewWelcome() string {
	var b strings.Builder

	title := m.styles.Title.Render("zmux init")
	ver := m.styles.Muted.Render(fmt.Sprintf(" v%s", m.version))
	b.WriteString("  " + title + ver + "\n\n")

	b.WriteString(m.styles.Normal.Render("  Welcome to zmux! This wizard will help you set up:") + "\n\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Config file (~/.zmux.toml)") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("tmux configuration (~/.tmux.conf)") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Theme selection") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Status bar preset") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Sync target configuration") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("User directories (~/.zmux/themes/, ~/.zmux/templates/)") + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to begin.") + "\n")

	return b.String()
}

func (m WizardModel) viewDepCheck() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Dependency Check") + "\n\n")

	depStyles := views.DepCheckStyles{
		Success: m.styles.Success,
		Error:   m.styles.Error,
		Normal:  m.styles.Normal,
		Muted:   m.styles.Muted,
	}
	b.WriteString(views.RenderDepCheck(m.deps.TmuxVersion, m.deps.ClipboardTool, depStyles))

	if m.deps.TmuxVersion == "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render("  tmux is required. Please install tmux >= 3.2") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to continue.") + "\n")

	return b.String()
}

func (m WizardModel) viewDetectTargets() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Detected Sync Targets") + "\n\n")

	if m.deps.HasGhostty {
		b.WriteString(m.styles.Success.Render("  [ok]"))
		b.WriteString(m.styles.Normal.Render("  Ghostty config found") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  [--]"))
		b.WriteString(m.styles.Muted.Render("  Ghostty config not found") + "\n")
	}

	if m.deps.HasNvim {
		b.WriteString(m.styles.Success.Render("  [ok]"))
		b.WriteString(m.styles.Normal.Render("  Neovim found") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  [--]"))
		b.WriteString(m.styles.Muted.Render("  Neovim not found") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to continue.") + "\n")

	return b.String()
}

func (m WizardModel) viewTheme() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Theme") + "\n\n")

	if len(m.themes) == 0 {
		b.WriteString(m.styles.Muted.Render("  No themes available.") + "\n")
		return b.String()
	}

	// Calculate visible window.
	availableHeight := m.height - 12
	if availableHeight < 5 {
		availableHeight = 10
	}

	start := 0
	if m.themeCursor >= availableHeight {
		start = m.themeCursor - availableHeight + 1
	}
	end := start + availableHeight
	if end > len(m.themes) {
		end = len(m.themes)
	}

	for i := start; i < end; i++ {
		cursor := "  "
		if i == m.themeCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.themeCursor {
			nameStyle = m.styles.Selected
		}
		name := nameStyle.Render(m.themes[i].Name)

		var sourceTag string
		switch m.themes[i].Source {
		case theme.SourceBundled:
			sourceTag = m.styles.Info.Render(" [bundled]")
		case theme.SourceUser:
			sourceTag = m.styles.Success.Render(" [user]")
		}

		b.WriteString("  " + cursor + name + sourceTag + "\n")
	}

	if start > 0 {
		b.WriteString(m.styles.Dim.Render("    ... more above") + "\n")
	}
	if end < len(m.themes) {
		b.WriteString(m.styles.Dim.Render("    ... more below") + "\n")
	}

	// Show swatch for selected theme.
	if m.themeCursor < len(m.themes) {
		t, err := m.resolver.Resolve(m.themes[m.themeCursor].Name)
		if err == nil {
			palette := t.SemanticPalette()
			width := m.width
			if width <= 0 {
				width = 80
			}
			swatch := views.RenderSwatch(&palette, width)
			if swatch != "" {
				b.WriteString("\n" + swatch + "\n")
			}
		}
	}

	return b.String()
}

func (m WizardModel) viewBarPreset() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Status Bar Preset") + "\n\n")

	// Get palette for previews.
	var palette *theme.Palette
	if m.chosenTheme != "" {
		t, err := m.resolver.Resolve(m.chosenTheme)
		if err == nil {
			p := t.SemanticPalette()
			palette = &p
		}
	}

	for i, p := range m.presets {
		cursor := "  "
		if i == m.presetCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.presetCursor {
			nameStyle = m.styles.Selected
		}
		name := nameStyle.Render(p.String())
		b.WriteString("  " + cursor + name + "\n")

		// ANSI preview.
		if palette != nil {
			preview := bar.RenderPreview(p, palette)
			b.WriteString("    " + preview + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m WizardModel) viewSyncTarget() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Sync Target") + "\n\n")
	b.WriteString(m.styles.Muted.Render("  Sync pulls your theme from another app.") + "\n\n")

	for i, target := range m.syncTargets {
		cursor := "  "
		if i == m.syncCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.syncCursor {
			nameStyle = m.styles.Selected
		}

		desc := ""
		switch target {
		case "none":
			desc = m.styles.Muted.Render(" (manual theme management)")
		case "ghostty":
			desc = m.styles.Muted.Render(" (sync from Ghostty terminal)")
		case "nvim":
			desc = m.styles.Muted.Render(" (sync from Neovim colorscheme)")
		}

		b.WriteString("  " + cursor + nameStyle.Render(target) + desc + "\n")
	}

	return b.String()
}

func (m WizardModel) viewSummary() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Configuration Summary") + "\n\n")

	b.WriteString(m.styles.Accent.Render("  Theme:      ") + m.styles.Normal.Render(m.chosenTheme) + "\n")
	b.WriteString(m.styles.Accent.Render("  Bar preset: ") + m.styles.Normal.Render(m.chosenPreset) + "\n")
	b.WriteString(m.styles.Accent.Render("  Prefix:     ") + m.styles.Normal.Render("Ctrl+Space") + "\n")
	b.WriteString(m.styles.Accent.Render("  Sync:       ") + m.styles.Normal.Render(m.chosenSync) + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Normal.Render("  This will create:") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux.toml") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.tmux.conf") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux/themes/") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux/templates/") + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Normal.Render("  Press Enter to write configuration.") + "\n")

	return b.String()
}

func (m WizardModel) viewWriting() string {
	return "  " + m.styles.Accent.Render("Writing configuration...") + "\n"
}

func (m WizardModel) viewSuccess() string {
	var b strings.Builder

	if m.Error != nil {
		b.WriteString("  " + m.styles.Error.Render("Error writing config:") + "\n")
		b.WriteString("  " + m.styles.Normal.Render(m.Error.Error()) + "\n")
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render("  Press Enter or q to exit.") + "\n")
		return b.String()
	}

	b.WriteString("  " + m.styles.Success.Render("Configuration written successfully!") + "\n\n")

	cmd := restartCmd(m.profile)
	b.WriteString(m.styles.Normal.Render("  Run this to apply:") + "\n\n")
	b.WriteString("    " + m.styles.Accent.Render(cmd) + "\n\n")

	if m.Copied {
		b.WriteString("  " + m.styles.Success.Render("Copied to clipboard!") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  c/y:copy  enter:exit") + "\n")
	}

	return b.String()
}

func (m WizardModel) viewHelp() string {
	parts := []string{"enter:next"}

	if m.canNavigateBack() {
		parts = append(parts, "shift+tab:back")
	}
	parts = append(parts, "ctrl+c:cancel")

	if m.step == stepTheme || m.step == stepBarPreset || m.step == stepSyncTarget {
		parts = append([]string{"j/k:navigate"}, parts...)
	}

	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}
