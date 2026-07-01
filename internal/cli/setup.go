package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/setup"
	"github.com/spf13/cobra"
)

// newSetupCmd builds the `setup` command group — Go-native shell integration
// that replaces install.sh's bash. `setup shell` adds (or removes) a managed
// auto-start + command-lifecycle block in your shell rc, backed up and idempotent.
func newSetupCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure zmux shell integration",
	}

	var dryRun bool
	var remove bool
	var assumeYes bool
	var binOverride string

	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Add or remove zmux shell integration in your shell rc",
		Long: `Adds a managed block to your shell rc (.bashrc/.zshrc/config.fish) that
launches the zmux session picker in interactive shells when not already inside
tmux and records command lifecycle events inside zmux-managed panes. The block
is delimited by zmux-managed markers so it can be cleanly updated or removed,
and the prior rc file is backed up to <rc>.bak.

  --dry-run   show what would change without writing
  --remove    remove the managed block
  --yes       skip the confirmation prompt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := app.FS.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home dir: %w", err)
			}
			shell := setup.DetectShell(os.Getenv("SHELL"))
			bin := app.Profile.Name
			if strings.TrimSpace(binOverride) != "" {
				bin = strings.TrimSpace(binOverride)
			}
			plan, ok := setup.PlanShellIntegration(setup.ShellInput{
				Shell:       shell,
				Home:        home,
				Bin:         bin,
				BashProfile: resolveBashProfileFile(app, home),
				Remove:      remove,
			})
			if !ok {
				return fmt.Errorf("unsupported shell %q — add zmux to your rc file manually", os.Getenv("SHELL"))
			}

			verb := "Add zmux shell integration to"
			if remove {
				verb = "Remove zmux shell integration from"
			}

			if dryRun {
				results, err := plan.Apply(app.FS, setup.ApplyOptions{DryRun: true})
				if err != nil {
					return err
				}
				for _, result := range results {
					fmt.Printf("dry-run: %s (%s)\n", strings.ToLower(result.Note), result.Edit.File)
				}
				return nil
			}

			if !assumeYes {
				fmt.Printf("%s %s? [y/N] ", verb, describeSetupFiles(plan))
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				if r := strings.TrimSpace(strings.ToLower(line)); r != "y" && r != "yes" {
					fmt.Println("Skipped.")
					return nil
				}
			}

			results, err := plan.Apply(app.FS, setup.ApplyOptions{Backup: true})
			if err != nil {
				return err
			}
			for _, result := range results {
				fmt.Printf("%s: %s\n", result.Note, result.Edit.File)
			}
			return nil
		},
	}
	shellCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without writing")
	shellCmd.Flags().BoolVar(&remove, "remove", false, "remove the zmux-managed block")
	shellCmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	shellCmd.Flags().StringVar(&binOverride, "bin", "", "binary name to write into the shell block (default: current profile)")

	cmd.AddCommand(shellCmd)
	return cmd
}

func resolveBashProfileFile(app *apppkg.App, home string) string {
	for _, path := range []string{
		home + "/.bash_profile",
		home + "/.bash_login",
		home + "/.profile",
	} {
		if _, err := app.FS.Stat(path); err == nil {
			return path
		}
	}
	return setup.DefaultBashProfile(home)
}

func describeSetupFiles(plan setup.Plan) string {
	files := make([]string, 0, len(plan.Edits))
	for _, edit := range plan.Edits {
		files = append(files, edit.File)
	}
	return strings.Join(files, ", ")
}
