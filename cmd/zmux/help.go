package main

import (
	"fmt"
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
	},
}

func init() {
	rootCmd.AddCommand(helpCmd)
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
	cmdStyle := lipgloss.NewStyle().Width(28).Foreground(lipgloss.Color("2"))
	descStyle := styles.Normal

	groups := []helpGroup{
		{
			title: "Session Management",
			entries: []helpEntry{
				{"zmux", "Launch session picker / dashboard"},
				{"zmux --dashboard", "Render dashboard directly (for popup)"},
			},
		},
		{
			title: "Theming",
			entries: []helpEntry{
				{"zmux theme", "Launch interactive theme picker"},
				{"zmux theme set <name>", "Set theme directly"},
				{"zmux theme list", "List available themes"},
				{"zmux theme sync", "Pull theme from default sync target"},
				{"zmux theme pull <target>", "Pull theme from ghostty/nvim"},
			},
		},
		{
			title: "Status Bar",
			entries: []helpEntry{
				{"zmux bar", "List bar presets with ANSI previews"},
				{"zmux bar <preset>", "Set bar preset (default/minimal/powerline/blocks)"},
				{"zmux bar show", "Show current bar preset"},
			},
		},
		{
			title: "Configuration",
			entries: []helpEntry{
				{"zmux init", "Run interactive setup wizard"},
				{"zmux apply", "Apply theme + bar to running tmux"},
				{"zmux status", "Show current configuration summary"},
			},
		},
		{
			title: "Miscellaneous",
			entries: []helpEntry{
				{"zmux version", "Print version information"},
				{"zmux completion <shell>", "Generate shell completions (bash/zsh/fish)"},
				{"zmux help", "Show this help"},
			},
		},
	}

	var b strings.Builder

	// Title.
	b.WriteString(titleStyle.Render("zmux") + " " + styles.Muted.Render("- an opinionated tmux management wrapper") + "\n\n")

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
	b.WriteString(groupStyle.Render("tmux Keybindings (with prefix)") + "\n")
	bindings := []helpEntry{
		{"prefix + d", "Open zmux dashboard popup"},
		{"prefix + ?", "Open zmux help popup"},
		{"prefix + v", "Enter copy mode (vi keys)"},
		{"prefix + c", "New window"},
		{"prefix + n", "Next window"},
		{"prefix + r", "Reload tmux config"},
		{"Alt+1-5", "Switch to window 1-5 (no prefix)"},
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
