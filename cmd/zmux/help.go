package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
	styles, _, _ := loadActiveStyles()

	titleStyle := styles.Accent.Bold(true)
	groupStyle := styles.Info.Bold(true)
	cmdStyle := styles.Success.Width(32)
	descStyle := styles.Normal

	groups := []helpGroup{
		{
			title: "Session Management",
			entries: []helpEntry{
				{"zmux", "Workspace picker (outside tmux) / dashboard (inside)"},
				{"zmux <ws>", "Attach workspace's last-active session"},
				{"zmux <ws> <session>", "Attach specific session in workspace"},
				{"zmux new <ws>", "Create workspace + main session, attach"},
				{"zmux new <ws> <session...>", "Create workspace + named sessions"},
				{"zmux new <ws> -t <tmpl>", "Create workspace from template"},
				{"zmux new", "Create tmp-N session (no workspace)"},
				{"zmux open <ws> [session]", "Open workspace session (alias: o, a)"},
				{"zmux kill <name>", "Smart kill — workspace-first (alias: k)"},
				{"zmux ls", "List workspaces (workspace-primary)"},
				{"zmux ls <ws>", "List sessions within a workspace"},
				{"zmux ls -s", "Flat session list"},
				{"zmux tabs [session]", "List tabs (alias: t)"},
			},
		},
		{
			title: "Workspace Management",
			entries: []helpEntry{
				{"zmux workspace list", "List workspaces (alias: ws)"},
				{"zmux workspace show <ws>", "Show workspace sessions"},
				{"zmux workspace kill <ws>", "Kill workspace + all sessions"},
				{"zmux session kill <session>", "Kill a session"},
				{"zmux tab move <tab> <dest>", "Move tab to another session"},
				{"zmux tab label [label]", "Set/clear stable tab label"},
				{"zmux tab kill <tab>", "Kill a tab"},
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
				{"zmux pane open <name> -r 40 -- <cmd>", "Open pane, print pane id"},
				{"zmux pane open --label-tab ...", "Preserve tab label before sidecar split"},
				{"zmux pane toggle <name> -r 40 -- <cmd>", "Toggle named pane"},
				{"zmux pane current [--json]", "Print current pane id/details"},
				{"zmux pane list / zmux panes", "List panes in current window"},
				{"zmux pane focus/close <pane>", "Focus or close pane by id/title/index"},
				{"zmux pane resize <pane> --size 40%", "Resize pane width"},
				{"zmux terminal current --json", "Resolve visible terminal screenshot target"},
				{"zmux terminal capabilities", "Diagnose tmux truecolor/RGB support"},
				{"zmux terminal refresh", "Reattach client to refresh RGB features"},
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
				{"zmux refresh", "Apply config + refresh current client"},
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
		{"prefix + w", "Workspace session picker"},
		{"prefix + [ / ]", "Previous / next session in workspace"},
		{"prefix + c", "New tab"},
		{"prefix + n / N", "Next / previous tab"},
		{"prefix + < / >", "Move tab left / right"},
		{"prefix + x", "Close tab (with confirm)"},
		{"prefix + R", "Respawn stopped/dead pane"},
		{"prefix + .", "Label tab (blank clears)"},
		{"prefix + ,", "Rename session"},
		{"prefix + r", "Reload config (zmux apply)"},
		{"prefix + v", "Enter vi copy mode"},
		{"prefix + ←/→/↑/↓", "Focus pane (tmux default)"},
		{"prefix + C-Arrow", "Resize pane by one cell (tmux default)"},
		{"prefix + M-Arrow", "Resize pane by five cells (tmux default)"},
		{"prefix + q / z", "Show pane ids / zoom pane (tmux default)"},
		{"prefix + o / ;", "Next / previous pane (tmux default)"},
		{"prefix + % / \"", "Split pane right / below (tmux default)"},
		{"Alt+1-9", "Switch to tab (no prefix)"},
		{"Shift+Alt+1-9", "Switch to session N in workspace"},
		{"Alt+Shift+Arrow", "Focus pane (no prefix)"},
		{"Alt+`", "Tab switcher (no prefix)"},
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
