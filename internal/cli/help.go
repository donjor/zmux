package cli

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/help"
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

// printStyledHelp prints the full help — command reference plus keybindings —
// from the shared help.Sections() source, so this text surface and the prefix+?
// viewer never drift on what help exists.
func printStyledHelp(app *apppkg.App) {
	styles, _, _ := loadActiveStyles(app)

	titleStyle := styles.Accent.Bold(true)
	groupStyle := styles.Info.Bold(true)
	cmdStyle := styles.Success.Width(32)
	descStyle := styles.Normal

	var b strings.Builder

	b.WriteString(titleStyle.Render("zmux") + " " + styles.Muted.Render("— an opinionated tmux management wrapper") + "\n\n")

	for _, section := range help.Sections() {
		b.WriteString(groupStyle.Render(section.Title) + "\n")
		for _, e := range section.Entries {
			cmd := cmdStyle.Render("  " + e.Label)
			desc := descStyle.Render(e.Desc)
			b.WriteString(cmd + " " + desc + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(styles.Muted.Render("Config: ~/.zmux.toml  |  Docs: https://github.com/donjor/zmux") + "\n")

	// lipgloss v2 renders full-fidelity; the package Writer downsamples to the
	// terminal's color profile (v1 did this implicitly via the global renderer).
	_, _ = lipgloss.Fprint(os.Stdout, b.String())
}
