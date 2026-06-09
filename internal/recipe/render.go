package recipe

import (
	"fmt"
	"strings"
)

func RunCommand(bin string, recipeName string, items []string, detach bool) string {
	return RunCommandWithOptions(bin, recipeName, PlanOptions{Items: items, Detach: detach}, PlanOptions{})
}

func RunCommandWithOptions(bin string, recipeName string, opts PlanOptions, defaults PlanOptions) string {
	if bin == "" {
		bin = "zmux"
	}
	parts := []string{bin, "run"}
	if opts.Detach {
		parts = append(parts, "--detach")
	}
	if opts.CWD != "" && defaults.CWD != "" && opts.CWD != defaults.CWD {
		parts = append(parts, "--cwd", ShellQuote(opts.CWD))
	}
	if opts.Workspace != "" && defaults.Workspace != "" && opts.Workspace != defaults.Workspace {
		parts = append(parts, "--workspace", ShellQuote(opts.Workspace))
	}
	if opts.Session != "" && defaults.Session != "" && opts.Session != defaults.Session {
		parts = append(parts, "--recipe-session", ShellQuote(opts.Session))
	}
	if opts.TabMode != "" && defaults.TabMode != "" && opts.TabMode != defaults.TabMode {
		parts = append(parts, "--tab-mode", ShellQuote(opts.TabMode))
	}
	parts = append(parts, recipeName)
	if hasFlagLikeItem(opts.Items) {
		parts = append(parts, "--")
	}
	for _, item := range opts.Items {
		parts = append(parts, ShellQuote(item))
	}
	return strings.Join(parts, " ")
}

func RenderPlan(p Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Recipe: %s", p.RecipeName)
	if p.RecipeDescription != "" {
		fmt.Fprintf(&b, " — %s", p.RecipeDescription)
	}
	b.WriteString("\n")
	if p.InsideZmux {
		b.WriteString("Run from: inside zmux\n")
	} else {
		b.WriteString("Run from: outside zmux\n")
	}
	if p.RunCommand != "" {
		fmt.Fprintf(&b, "Command: %s\n", p.RunCommand)
	}
	fmt.Fprintf(&b, "Workspace: %s\n", p.Workspace)
	fmt.Fprintf(&b, "Root: %s\n", p.CWD)
	if p.SessionDefault != "" {
		fmt.Fprintf(&b, "Session: %s\n", p.SessionDefault)
	}
	if p.TabMode != "" {
		fmt.Fprintf(&b, "Tabs: %s\n", p.TabMode)
	}
	if p.Detach {
		b.WriteString("Mode: detach\n")
	}
	if len(p.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, warning := range p.Warnings {
			fmt.Fprintf(&b, "  ! %s  %s\n", warning.Target, warning.Message)
		}
	}
	b.WriteString("\n")
	b.WriteString("Inside zmux commands:\n")
	for _, line := range insideCommandLines(p) {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	b.WriteString("\n")
	b.WriteString("Reconcile:\n")
	for _, sess := range p.Sessions {
		status := "create"
		if sess.Exists {
			status = "existing"
		}
		fmt.Fprintf(&b, "%s session %s\n", status, sess.Name)
		for _, tab := range sess.Tabs {
			tabStatus := "create"
			if tab.Exists {
				tabStatus = "existing"
			}
			fmt.Fprintf(&b, "  %s tab %s", tabStatus, tab.Name)
			if tab.Command != "" {
				fmt.Fprintf(&b, "  -> %s", tab.Command)
			}
			b.WriteString("\n")
		}
	}
	if p.FocusSession != "" {
		target := p.FocusSession
		if p.FocusTab != "" {
			target += ":" + p.FocusTab
		}
		if p.Detach {
			fmt.Fprintf(&b, "\nFocus: %s (skipped by --detach)\n", target)
		} else if p.InsideZmux {
			fmt.Fprintf(&b, "\nFocus: %s (switch-client)\n", target)
		} else {
			fmt.Fprintf(&b, "\nFocus: %s (attach)\n", target)
		}
	}
	b.WriteString("\nActions:\n")
	for _, action := range p.Actions {
		prefix := "+"
		if action.Skipped {
			prefix = "="
		}
		fmt.Fprintf(&b, "  %s %s %s", prefix, action.Kind, action.Target)
		if action.Detail != "" {
			fmt.Fprintf(&b, "  %s", action.Detail)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func insideCommandLines(p Plan) []string {
	var lines []string
	for _, sess := range p.Sessions {
		for _, tab := range sess.Tabs {
			cmd := tab.Command
			if cmd == "" {
				cmd = "interactive shell"
			}
			status := "will run"
			if p.TabMode == TabModeReady {
				status = "ready at prompt"
			}
			if p.TabMode == TabModeEmpty {
				status = "empty tab"
			}
			if tab.Exists {
				status = "skipped; tab exists"
			}
			lines = append(lines, fmt.Sprintf("%s:%s  %s  (%s)", sess.Name, tab.Name, cmd, status))
		}
	}
	if len(lines) == 0 {
		return []string{"none"}
	}
	return lines
}

func hasFlagLikeItem(items []string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, "-") {
			return true
		}
	}
	return false
}
