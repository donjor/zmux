package cli

import (
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
	var doctor bool
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
  --doctor    check rc files and the current shell's loaded hook version
  --yes       skip the confirmation prompt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if doctor {
				doctorPlan, doctorBin, err := setupDoctorPlan(app, binOverride)
				if err != nil {
					return err
				}
				return runSetupDoctor(app, doctorPlan, doctorBin)
			}

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

			if !assumeYes && !confirm(fmt.Sprintf("%s %s?", verb, describeSetupFiles(plan))) {
				fmt.Println("Skipped.")
				return nil
			}

			results, err := plan.Apply(app.FS, setup.ApplyOptions{Backup: true})
			if err != nil {
				return err
			}
			for _, result := range results {
				fmt.Printf("%s: %s\n", result.Note, result.Edit.File)
			}
			printLoadedShellHint(app)
			return nil
		},
	}
	shellCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without writing")
	shellCmd.Flags().BoolVar(&remove, "remove", false, "remove the zmux-managed block")
	shellCmd.Flags().BoolVar(&doctor, "doctor", false, "check rc files and current shell hook version")
	shellCmd.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	shellCmd.Flags().StringVar(&binOverride, "bin", "", "binary name to write into the shell block (default: current profile)")

	// `zmux setup doctor` is the same command as top-level `zmux doctor`.
	cmd.AddCommand(shellCmd, newDoctorCmd(app))
	return cmd
}

func newDoctorCmd(app *apppkg.App) *cobra.Command {
	var binOverride string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check zmux shell integration freshness",
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, bin, err := setupDoctorPlan(app, binOverride)
			if err != nil {
				return err
			}
			return runSetupDoctor(app, plan, bin)
		},
	}
	cmd.Flags().StringVar(&binOverride, "bin", "", "binary name expected in the shell block when comparing setup plans")
	return cmd
}

func setupDoctorPlan(app *apppkg.App, binOverride string) (setup.Plan, string, error) {
	home, err := app.FS.UserHomeDir()
	if err != nil {
		return setup.Plan{}, "", fmt.Errorf("resolve home dir: %w", err)
	}
	shell := setup.DetectShell(os.Getenv("SHELL"))
	bin := "zmux"
	if strings.TrimSpace(binOverride) != "" {
		bin = strings.TrimSpace(binOverride)
	}
	plan, ok := setup.PlanShellIntegration(setup.ShellInput{
		Shell:       shell,
		Home:        home,
		Bin:         bin,
		BashProfile: resolveBashProfileFile(app, home),
	})
	if !ok {
		return setup.Plan{}, "", fmt.Errorf("unsupported shell %q — add zmux to your rc file manually", os.Getenv("SHELL"))
	}
	return plan, bin, nil
}

func runSetupDoctor(app *apppkg.App, plan setup.Plan, bin string) error {
	ok := true
	cmdName := setupCommandName(app)
	fmt.Printf("zmux shell integration doctor\n")
	fmt.Printf("expected shell integration version: %s\n", setup.ShellIntegrationVersion)

	for _, edit := range plan.Edits {
		fileOK, reason := diagnoseSetupFile(app, edit)
		if fileOK {
			fmt.Printf("ok: %s — %s\n", edit.File, reason)
		} else {
			ok = false
			fmt.Printf("stale: %s — %s\n", edit.File, reason)
		}
	}

	if inTmuxShell() {
		loaded := strings.TrimSpace(os.Getenv("ZMUX_SHELL_INTEGRATION_VERSION"))
		if loaded == setup.ShellIntegrationVersion {
			fmt.Printf("ok: current shell loaded integration version %s\n", loaded)
		} else {
			ok = false
			if loaded == "" {
				loaded = "missing"
			}
			fmt.Printf("stale: current shell loaded integration version %s (expected %s)\n", loaded, setup.ShellIntegrationVersion)
			fmt.Println("hint: existing shells keep already-loaded hook functions; open a fresh zmux tab/shell after setup")
		}
		fmt.Printf("current shell: ZMUX_BIN=%q TMUX_PANE=%q ZMUX_SHELL_ROOT=%q\n", os.Getenv("ZMUX_BIN"), os.Getenv("TMUX_PANE"), os.Getenv("ZMUX_SHELL_ROOT"))
	} else {
		fmt.Println("ok: not inside tmux; runtime hook freshness not checked")
	}

	if !ok {
		fmt.Printf("fix: run `%s setup shell --yes --bin %s`, then open a fresh shell/tab\n", cmdName, setupDoctorBin(bin))
		return fmt.Errorf("shell integration doctor found issues")
	}
	return nil
}

func diagnoseSetupFile(app *apppkg.App, edit setup.Edit) (bool, string) {
	data, err := app.FS.ReadFile(edit.File)
	if err != nil {
		return false, "file missing"
	}
	block, ok := setup.ManagedBlock(string(data))
	if !ok {
		return false, "zmux-managed block missing"
	}
	if strings.Contains(edit.Label, "bash login bridge") {
		if strings.Contains(block, "__zmux_setup_lifecycle") && strings.Contains(block, "ZMUX_BASHRC_BRIDGED") {
			return true, "login bridge present"
		}
		return false, "login bridge is missing current lifecycle bridge"
	}
	if !strings.Contains(block, "ZMUX_SHELL_INTEGRATION_VERSION") || !strings.Contains(block, shellSingleQuoted(setup.ShellIntegrationVersion)) {
		return false, "managed block missing current version marker"
	}
	if strings.Contains(block, "ble-attach") {
		return false, "managed block contains retired forced ble-attach path"
	}
	if block != strings.TrimRight(edit.Block, "\n") {
		return false, "managed block differs from expected setup plan"
	}
	return true, "managed block current"
}

func printLoadedShellHint(app *apppkg.App) {
	if !inTmuxShell() {
		return
	}
	loaded := strings.TrimSpace(os.Getenv("ZMUX_SHELL_INTEGRATION_VERSION"))
	if loaded == setup.ShellIntegrationVersion {
		return
	}
	if loaded == "" {
		loaded = "missing"
	}
	fmt.Printf("note: current shell loaded integration version %s; open a fresh shell/tab to use version %s\n", loaded, setup.ShellIntegrationVersion)
	fmt.Printf("note: verify with `%s doctor`\n", setupCommandName(app))
}

func inTmuxShell() bool {
	return strings.TrimSpace(os.Getenv("TMUX")) != "" || strings.TrimSpace(os.Getenv("TMUX_PANE")) != ""
}

func setupCommandName(app *apppkg.App) string {
	if strings.TrimSpace(app.Profile.Name) != "" {
		return strings.TrimSpace(app.Profile.Name)
	}
	return "zmux"
}

func setupDoctorBin(bin string) string {
	if strings.TrimSpace(bin) != "" {
		return strings.TrimSpace(bin)
	}
	return "zmux"
}

func shellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
