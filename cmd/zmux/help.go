package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/tui"
)

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show styled help with command groups",
	Long:  `Displays all zmux commands grouped by category with styled output.`,
	Run: func(cmd *cobra.Command, args []string) {
		printStyledHelp()

		if app.Runner.IsInsideTmux() {
			fmt.Print("\n  Press any key to close...")
			buf := make([]byte, 1)
			os.Stdin.Read(buf)
		}
	},
}

func init() {
	rootCmd.AddCommand(helpCmd)

	// Override cobra's default -h/--help to show our styled help.
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printStyledHelp()
	})
}

type helpEntry struct {
	cmd  string
	desc string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

func printStyledHelp() {
	styles := tui.DefaultStyles()

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	cmdStyle := lipgloss.NewStyle().Width(32).Foreground(lipgloss.Color("2"))
	descStyle := styles.Normal

	groups := []helpGroup{
		{
			title: "Session Management",
			entries: []helpEntry{
				{"zmux", "Session picker / dashboard"},
				{"zmux <name>", "Attach or create session"},
				{"zmux new [name]", "Create + attach (alias: n)"},
				{"zmux new -t <tmpl> [name]", "Create from template"},
				{"zmux attach <name>", "Attach to session (alias: a)"},
				{"zmux attach --mirror <name>", "Shared view (agent+user)"},
				{"zmux attach --hijack <name>", "Steal session"},
				{"zmux kill <name>", "Kill session (alias: k)"},
				{"zmux ls", "List sessions"},
				{"zmux tabs [session]", "List tabs (alias: t)"},
			},
		},
		{
			title: "Terminal Commands",
			entries: []helpEntry{
				{"zmux run '<cmd>' -n <tab>", "Run + wait for completion"},
				{"zmux run '<cmd>' -n <tab> -d", "Run detached (servers)"},
				{"zmux run '<cmd>' -n <tab> -f", "Run + follow output"},
				{"zmux watch <tab>", "Capture tab output"},
				{"zmux watch <tab> --until <pat>", "Wait for pattern match"},
				{"zmux watch <tab> -f", "Follow output (tail -f)"},
				{"zmux send <tab> <keys>", "Send keystrokes to tab"},
				{"zmux type <tab> '<text>'", "Type text + Enter"},
			},
		},
		{
			title: "Theming",
			entries: []helpEntry{
				{"zmux theme", "Interactive theme picker"},
				{"zmux theme set <name>", "Set theme directly"},
				{"zmux theme list", "List available themes"},
				{"zmux theme sync", "Sync from default target"},
				{"zmux theme pull <target>", "Pull from ghostty/nvim"},
			},
		},
		{
			title: "Configuration",
			entries: []helpEntry{
				{"zmux bar", "Bar presets with previews"},
				{"zmux bar <preset>", "Set preset"},
				{"zmux init", "Setup wizard"},
				{"zmux apply", "Regenerate + apply config"},
				{"zmux status", "Show config summary"},
			},
		},
		{
			title: "Other",
			entries: []helpEntry{
				{"zmux version", "Version info"},
				{"zmux completion <shell>", "Completions (bash/zsh/fish)"},
				{"zmux help", "This help"},
			},
		},
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("zmux") + " " + styles.Muted.Render("— an opinionated tmux management wrapper") + "\n\n")

	for _, group := range groups {
		b.WriteString(groupStyle.Render(group.title) + "\n")

		for _, entry := range group.entries {
			cmd := cmdStyle.Render("  " + entry.cmd)
			desc := descStyle.Render(entry.desc)
			b.WriteString(cmd + " " + desc + "\n")
		}
		b.WriteString("\n")
	}

	// Keybindings section.
	b.WriteString(groupStyle.Render("Keybindings (prefix: Ctrl+Space)") + "\n")
	bindings := []helpEntry{
		{"prefix + Space", "Dashboard"},
		{"prefix + p", "Command palette"},
		{"prefix + d", "Detach"},
		{"prefix + ?", "Help popup"},
		{"prefix + c", "New tab"},
		{"prefix + n", "Next tab"},
		{"prefix + r", "Reload config (zmux apply)"},
		{"Alt+1-5", "Switch to tab (no prefix)"},
	}
	for _, entry := range bindings {
		cmd := cmdStyle.Render("  " + entry.cmd)
		desc := descStyle.Render(entry.desc)
		b.WriteString(cmd + " " + desc + "\n")
	}
	b.WriteString("\n")

	b.WriteString(styles.Muted.Render("Config: ~/.zmux.toml  |  Docs: https://github.com/donjor/zmux") + "\n")

	fmt.Print(b.String())
}
