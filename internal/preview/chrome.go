package preview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderHero draws the app-level design-lab heading.
func renderHero(width int) string {
	// Width is content width for lipgloss; leave room for border + padding so
	// the hero never wraps badges into a visually odd second line.
	innerW := maxPreview(24, width-10)
	line := TitleStyle.Render("zmux uiproto") +
		MuteStyle.Render("  visual design lab  ·  ") +
		lipgloss.NewStyle().Foreground(Blue).Bold(true).Render("● live") +
		MuteStyle.Render(" · ") +
		lipgloss.NewStyle().Foreground(Gold).Bold(true).Render("prototype")
	if lipgloss.Width(line) > innerW {
		line = TitleStyle.Render("zmux uiproto") + MuteStyle.Render("  visual design lab")
	}
	sub := SubtitleStyle.Render("iterate status bars, pane chrome, dashboard rows, and picker surfaces outside production paths")
	return HeroStyle.Width(innerW).Render(line + "\n" + sub)
}

// renderPageTabs draws the top page switcher strip.
func renderPageTabs(pages []Page, active int) string {
	var parts []string
	for i, p := range pages {
		label := p.Title()
		if i == active {
			parts = append(parts, TabActiveStyle.Render("▸ "+label))
		} else {
			parts = append(parts, TabInactiveStyle.Render("  "+label))
		}
	}
	return strings.Join(parts, " ")
}

func renderDivider(width int) string {
	if width < 4 {
		return ""
	}
	return DimStyle.Render(strings.Repeat("━", width))
}

// renderControls draws a vertical list of controls, one per line.
func renderControls(ctrls []Control, focus int) string {
	if len(ctrls) == 0 {
		return MuteStyle.Render("(no controls)")
	}
	var lines []string
	for i, c := range ctrls {
		lines = append(lines, c.View(i == focus))
	}
	return strings.Join(lines, "\n")
}

func renderControlPanel(ctrls []Control, focus, width int) string {
	header := TitleStyle.Render("controls") + "\n" + SubtitleStyle.Render("↑↓ select · ←→ adjust · space toggle")
	body := renderControls(ctrls, focus)
	return PanelStyle.Width(width).Render(header + "\n\n" + body)
}

func renderPreviewPanel(title string, content string, width, height int, focused bool) string {
	style := PanelStyle
	if focused {
		style = PanelActiveStyle
	}
	usableH := height - 4
	if usableH < 6 {
		usableH = 6
	}
	header := TitleStyle.Render(title) + "  " + SubtitleStyle.Render("preview canvas")
	body := clampLines(content, usableH)
	return style.Width(width).Render(header + "\n\n" + body)
}

func renderFooter() string {
	hints := []string{
		"tab page",
		"shift+tab back",
		"↑↓ controls",
		"←→ adjust",
		"space toggle",
		"q quit",
	}
	return HelpStyle.Render("  " + strings.Join(hints, "  ·  "))
}

func clampLines(s string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	trimmed := append([]string{}, lines[:maxLines-1]...)
	trimmed = append(trimmed, MuteStyle.Render(fmt.Sprintf("… %d more lines", len(lines)-maxLines+1)))
	return strings.Join(trimmed, "\n")
}

func maxPreview(a, b int) int {
	if a > b {
		return a
	}
	return b
}
