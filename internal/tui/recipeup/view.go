package recipeup

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/recipe"
)

func (m Model) render() string {
	header := m.headerView()
	bodyHeight := max(10, m.height-6)
	var body string
	switch m.screen {
	case screenInput:
		body = m.inputView(bodyHeight)
	case screenDryRun:
		body = m.planView(bodyHeight)
	case screenResult:
		body = m.resultView(bodyHeight)
	case screenEditor:
		body = m.editorView(bodyHeight)
	default:
		body = m.browserView(bodyHeight)
	}
	footer := m.footerView()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) headerView() string {
	left := titleStyle.Render("zmux recipes")
	context := "outside zmux"
	if m.app.Runner.IsInsideTmux() {
		context = "inside zmux"
	}
	right := mutedStyle.Render(context + "  recipe dirs: " + pathSummary(m))
	line := left
	padding := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	line += strings.Repeat(" ", padding) + right
	return lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(line)
}

func (m Model) browserView(height int) string {
	leftW := min(38, max(28, m.width/3))
	rightW := max(32, m.width-leftW-6)
	left := m.recipeListView(leftW, height)
	right := m.detailView(rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(leftW).Height(height).Render(left),
		"  ",
		panelStyle.Width(rightW).Height(height).Render(right),
	)
}

func (m Model) recipeListView(width int, height int) string {
	if len(m.defs) == 0 {
		return mutedStyle.Render("No recipes found")
	}
	lines := []string{titleStyle.Render("Recipes"), mutedStyle.Render("lint  name  kind"), ""}
	limit := max(1, height-4)
	start := 0
	if m.cursor >= limit {
		start = m.cursor - limit + 1
	}
	for i := start; i < len(m.defs) && len(lines) < height-1; i++ {
		def := m.defs[i]
		label := fmt.Sprintf("%s %s  %s",
			m.lintBadge(def.Recipe.Name),
			def.Recipe.Name,
			mutedStyle.Render(string(def.Recipe.Kind)),
		)
		if i == m.cursor {
			label = m.lintBadge(def.Recipe.Name) + " " + activeStyle.Render(def.Recipe.Name) + " " + mutedStyle.Render(string(def.Recipe.Kind))
		}
		lines = append(lines, clampLine(label, width-4))
	}
	return strings.Join(lines, "\n")
}

func (m Model) detailView(width int, height int) string {
	def, ok := m.selected()
	if !ok {
		return mutedStyle.Render("Select a recipe")
	}
	runCommand := recipe.RunCommand(m.app.Profile.Name, def.Recipe.Name, nil, false)
	if needsItems(def.Recipe) {
		runCommand += " <items...>"
	}
	defaults := defaultFormValues(def.Recipe, recipe.PlanOptions{}, nil)
	lintLine := okStyle.Render("lint ok")
	if result, ok := m.lintFor(def.Recipe.Name); ok && result.Err != nil {
		lintLine = errorStyle.Render("lint error: " + result.Err.Error())
	}
	lines := []string{
		titleStyle.Render(def.Recipe.Name),
		badge(string(def.Source), muted) + " " + badge(string(def.Recipe.Kind), accent) + " " + badge(def.Recipe.Context, yellow) + " " + lintLine,
		"",
		def.Recipe.Description,
		"",
		sectionStyle.Render("Command"),
		"  " + commandStyle.Render(runCommand),
		"  " + mutedStyle.Render(runCommand+" -y"),
		"",
		sectionStyle.Render("Defaults"),
		fmt.Sprintf("  CWD: %s", defaults.CWD),
		fmt.Sprintf("  Workspace: %s", defaults.Workspace),
		fmt.Sprintf("  Session: %s", defaults.Session),
		fmt.Sprintf("  Tabs: %s", defaults.TabMode),
	}
	if needsItems(def.Recipe) {
		lines = append(lines, "  "+warnStyle.Render(fmt.Sprintf("Inputs: %d+ item(s)", max(1, def.Recipe.Inputs.MinItems))))
	}
	lines = append(lines, "", sectionStyle.Render("Plan Shape"))
	lines = append(lines, fmt.Sprintf("  Workspace TOML: %s", def.Recipe.Workspace))
	if def.Recipe.Session != "" {
		lines = append(lines, fmt.Sprintf("  Session TOML: %s", def.Recipe.Session))
	}
	lines = append(lines, "", sectionStyle.Render("Inside zmux"))
	for _, spec := range commandSpecsForDetail(def.Recipe) {
		cmd := spec.command
		if cmd == "" {
			cmd = "interactive shell"
		}
		line := fmt.Sprintf("  %s  %s", spec.target, commandStyle.Render(cmd))
		lines = append(lines, clampLine(line, width-4))
	}
	if len(def.Raw) > 0 {
		lines = append(lines, "", sectionStyle.Render("TOML"))
		for _, line := range strings.Split(strings.TrimSpace(string(def.Raw)), "\n") {
			lines = append(lines, "  "+mutedStyle.Render(line))
			if len(lines) >= height-1 {
				break
			}
		}
	}
	return limitLines(strings.Join(lines, "\n"), height)
}

type commandSpec struct {
	target  string
	command string
}

func commandSpecsForDetail(r recipe.Recipe) []commandSpec {
	var specs []commandSpec
	if len(r.Sessions) > 0 {
		for _, sess := range r.Sessions {
			sessionName := sess.Name
			if sessionName == "" {
				sessionName = r.Session
			}
			if sessionName == "" {
				sessionName = "{{ workspace }}"
			}
			for _, tab := range sess.Tabs {
				specs = append(specs, commandSpec{target: sessionName + ":" + tab.Name, command: tab.Command})
			}
		}
		return specs
	}
	sessionName := r.Session
	if sessionName == "" {
		sessionName = "{{ workspace }}"
	}
	for _, tab := range r.Tabs {
		specs = append(specs, commandSpec{target: sessionName + ":" + tab.Name, command: tab.Command})
	}
	return specs
}

func (m Model) inputView(height int) string {
	body := titleStyle.Render("Recipe inputs") + "\n\n"
	if m.form != nil {
		body += m.form.View()
	}
	return panelStyle.Width(m.width - 4).Height(height).Render(body)
}

func (m Model) planView(height int) string {
	body := titleStyle.Render("Dry-run plan") + mutedStyle.Render("  enter/y: run  esc: back") + "\n\n"
	body += m.renderPlan()
	return panelStyle.Width(m.width - 4).Height(height).Render(limitLines(body, height-2))
}

func (m Model) resultView(height int) string {
	lines := []string{titleStyle.Render("Result"), ""}
	if m.err != nil {
		lines = append(lines, errorStyle.Render(m.err.Error()))
	} else {
		lines = append(lines, okStyle.Render("Recipe completed"))
	}
	lines = append(lines, "", "Log")
	for _, line := range tail(m.logs, height-7) {
		lines = append(lines, "  "+line)
	}
	return panelStyle.Width(m.width - 4).Height(height).Render(strings.Join(lines, "\n"))
}

func (m Model) editorView(height int) string {
	title := titleStyle.Render("TOML editor")
	if m.editorName != "" {
		title += mutedStyle.Render("  " + m.editorName)
	}
	body := title + mutedStyle.Render("  ctrl+s: save  esc: back") + "\n"
	if m.err != nil {
		body += errorStyle.Render(m.err.Error()) + "\n"
	} else {
		body += mutedStyle.Render(m.editorPath) + "\n"
	}
	body += "\n" + m.editor.View()
	return panelStyle.Width(m.width - 4).Height(height).Render(limitLines(body, height-2))
}

func (m Model) footerView() string {
	keys := "enter:configure/run  f:fork  e:edit  n:new  l:lint  r:reload  ?:help  q:quit"
	if m.screen == screenEditor {
		keys = "ctrl+s:save  esc:back  ctrl+c:quit"
	}
	if count := m.lintErrorCount(); count > 0 {
		keys = fmt.Sprintf("lint: %d error(s)  |  %s", count, keys)
	}
	if m.showHelp {
		keys += "  |  Recipes reconcile missing pieces and skip existing sessions/tabs."
	}
	if m.err != nil && m.screen != screenResult {
		keys = errorStyle.Render(m.err.Error()) + "\n" + mutedStyle.Render(keys)
	} else {
		keys = mutedStyle.Render(keys)
	}
	return lipgloss.NewStyle().Width(m.width).Padding(0, 1).Render(keys)
}

func pathSummary(m Model) string {
	if len(m.dirs) == 0 {
		return "none"
	}
	return strings.Join(m.dirs, ", ")
}

func (m Model) renderPlan() string {
	p := m.plan
	lines := []string{
		titleStyle.Render(p.RecipeName),
		mutedStyle.Render(p.RecipeDescription),
		"",
		sectionStyle.Render("Outside zmux"),
		"  " + commandStyle.Render(p.RunCommand),
		"",
		sectionStyle.Render("Defaults"),
		fmt.Sprintf("  Workspace: %s", p.Workspace),
		fmt.Sprintf("  Session: %s", p.SessionDefault),
		fmt.Sprintf("  Root: %s", p.CWD),
		fmt.Sprintf("  Tabs: %s", p.TabMode),
		"",
		sectionStyle.Render("Inside zmux commands"),
	}
	if len(p.Warnings) > 0 {
		lines = append(lines, sectionStyle.Render("Warnings"))
		for _, warning := range p.Warnings {
			lines = append(lines, "  "+warnStyle.Render(warning.Target+"  "+warning.Message))
		}
		lines = append(lines, "")
	}
	for _, sess := range p.Sessions {
		for _, tab := range sess.Tabs {
			cmd := tab.Command
			if cmd == "" {
				cmd = "interactive shell"
			}
			status := okStyle.Render("will run")
			if p.TabMode == recipe.TabModeReady {
				status = warnStyle.Render("ready")
			}
			if p.TabMode == recipe.TabModeEmpty {
				status = mutedStyle.Render("empty")
			}
			if tab.Exists {
				status = warnStyle.Render("skipped: tab exists")
			}
			lines = append(lines, fmt.Sprintf("  %s:%s  %s  %s", sess.Name, tab.Name, commandStyle.Render(cmd), status))
		}
	}
	lines = append(lines, "", sectionStyle.Render("Reconcile"))
	lines = append(lines, fmt.Sprintf("  Workspace: %s", p.Workspace))
	lines = append(lines, fmt.Sprintf("  Root: %s", p.CWD))
	for _, sess := range p.Sessions {
		lines = append(lines, fmt.Sprintf("  %s session %s", createStatus("create", sess.Exists), sess.Name))
		for _, tab := range sess.Tabs {
			lines = append(lines, fmt.Sprintf("    %s tab %s", createStatus("create", tab.Exists), tab.Name))
		}
	}
	if p.FocusSession != "" {
		target := p.FocusSession
		if p.FocusTab != "" {
			target += ":" + p.FocusTab
		}
		mode := "attach"
		if p.InsideZmux {
			mode = "switch-client"
		}
		if p.Detach {
			lines = append(lines, "", fmt.Sprintf("  Focus: %s (%s skipped by detach)", target, mode))
		} else {
			lines = append(lines, "", fmt.Sprintf("  Focus: %s (%s)", target, mode))
		}
	}
	lines = append(lines, "", sectionStyle.Render("Actions"))
	for _, action := range p.Actions {
		prefix := okStyle.Render("+")
		if action.Skipped {
			prefix = warnStyle.Render("=")
		}
		line := fmt.Sprintf("  %s %s %s", prefix, action.Kind, action.Target)
		if action.Detail != "" {
			line += "  " + mutedStyle.Render(action.Detail)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func createStatus(word string, exists bool) string {
	if exists {
		return warnStyle.Render("existing")
	}
	return okStyle.Render(word)
}

func (m Model) lintBadge(name string) string {
	if result, ok := m.lintFor(name); ok && result.Err != nil {
		return errorStyle.Render("err")
	}
	return okStyle.Render("ok ")
}

func badge(label string, c color.Color) string {
	return chipStyle.Foreground(c).Render(label)
}

func needsItems(r recipe.Recipe) bool {
	return r.Inputs.Items || r.Inputs.MinItems > 0 || r.ForEach == "items"
}

func clampLine(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	plain := []rune(s)
	if len(plain) > width-1 {
		plain = plain[:width-1]
	}
	return string(plain) + "…"
}

func limitLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n")
}

func tail(lines []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
