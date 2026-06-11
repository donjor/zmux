package cli

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/keys"
	"github.com/spf13/cobra"
)

func newHelpCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "help",
		Short: "Show styled help with command groups",
		Long:  `Displays all zmux commands grouped by category with styled output.`,
		Run: func(cmd *cobra.Command, args []string) {
			printStyledHelp(app)

			if app.Runner.IsInsideTmux() {
				fmt.Print("\n  Press any key to close...")
				buf := make([]byte, 1)
				_, _ = os.Stdin.Read(buf)
			}
		},
	}
}

type helpEntry struct {
	cmd  string
	desc string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

func printStyledHelp(app *apppkg.App) {
	styles, _, _ := loadActiveStyles(app)

	titleStyle := styles.Accent.Bold(true)
	groupStyle := styles.Info.Bold(true)
	cmdStyle := styles.Success.Width(32)
	descStyle := styles.Normal

	groups := []helpGroup{
		{
			title: "Session Management",
			entries: []helpEntry{
				{"zmux", "Workspace picker (outside tmux) / dashboard (inside)"},
				{"zmux new <ws>", "Create workspace + main session, attach"},
				{"zmux new <ws> <session...>", "Create workspace + named sessions"},
				{"zmux new", "Create tmp-N session (no workspace)"},
				{"zmux open <ws> [session]", "Open workspace session (aliases: attach, a)"},
				{"zmux kill <name>", "Smart kill — workspace-first (alias: k)"},
				{"zmux ls", "List workspaces (workspace-primary)"},
				{"zmux ls <ws>", "List sessions within a workspace"},
				{"zmux ls -s", "Flat session list"},
				{"zmux tabs [session]", "List tabs (alias: t)"},
			},
		},
		{
			title: "Recipes",
			entries: []helpEntry{
				{"zmux run <recipe>", "Open the recipe form with defaults"},
				{"zmux run <recipe> -y", "Run recipe defaults without prompting"},
				{"zmux run <recipe> --dry-run", "Print the exact recipe plan"},
				{"zmux run --command <cmd>", "Force command mode on name collisions"},
				{"zmux recipe list", "List bundled and user recipes"},
				{"zmux recipe show <recipe>", "Show recipe TOML"},
				{"zmux recipe lint [recipe]", "Validate recipes"},
				{"zmux recipe fork <recipe>", "Copy a bundled recipe for editing"},
				{"zmux recipe edit <recipe>", "Edit a user recipe"},
			},
		},
		{
			title: "Workspace Management",
			entries: []helpEntry{
				{"zmux workspace list", "List workspaces (alias: ws)"},
				{"zmux workspace show <ws>", "Show workspace sessions"},
				{"zmux workspace kill <ws>", "Kill workspace + all sessions"},
				{"zmux session kill <session>", "Kill a session"},
				{"zmux session run <s> -n <t> -- <cmd>", "Detached session + command as first tab (workers)"},
				{"zmux tab move <tab> <dest>", "Move tab to another session"},
				{"zmux tab label [label]", "Set/clear stable tab label"},
				{"zmux tab state <state> [tab]", "Set/clear tab lifecycle glyph"},
				{"zmux tab pane <tab>", "Join a tab into the current tab as a pane"},
				{"zmux tab full [tab]", "Promote focused/named pane-tab to full tab"},
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
				{"zmux watch <tab> --idle <s>", "Wait until quiet for N seconds"},
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
				{"zmux snapshot [--no-png]", "Capture pane text/ANSI + PNG evidence bundle"},
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

	// Keybindings section — rendered from the internal/keys registry so the
	// help output, generated docs, and tmux conf never drift.
	b.WriteString(groupStyle.Render("Keybindings (prefix: Ctrl+Space)") + "\n")
	renderBindings := func(bindings []keys.Binding) {
		for _, kb := range bindings {
			cmd := cmdStyle.Render("  " + kb.DisplayKey())
			desc := descStyle.Render(kb.Help)
			b.WriteString(cmd + " " + desc + "\n")
		}
	}
	renderBindings(keys.PrefixBindings)
	renderBindings(keys.NoPrefixBindings)
	renderBindings(keys.InheritedBindings)
	b.WriteString("\n")

	b.WriteString(styles.Muted.Render("Config: ~/.zmux.toml  |  Docs: https://github.com/donjor/zmux") + "\n")

	// lipgloss v2 renders full-fidelity; the package Writer downsamples to the
	// terminal's color profile (v1 did this implicitly via the global renderer).
	lipgloss.Print(b.String())
}
