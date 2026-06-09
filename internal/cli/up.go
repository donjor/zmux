package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/recipe"
	"github.com/donjor/zmux/internal/tui/recipeup"
	"github.com/spf13/cobra"
)

func newUpCmd(app *apppkg.App) *cobra.Command {
	var yes bool
	var dryRun bool
	var detach bool
	var workspaceName string
	var snapshot bool

	cmd := &cobra.Command{
		Use:   "up [recipe] [items...]",
		Short: "Legacy alias for the recipe runner",
		Long: `Legacy recipe runner. Prefer zmux run <recipe>.

  zmux run dev                    Open the recipe form
  zmux run dev -y                 Run defaults without prompting
  zmux run dev --dry-run          Print the plan`,
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if snapshot {
					out, err := recipeup.Snapshot(app)
					if err != nil {
						return err
					}
					fmt.Fprint(cmd.OutOrStdout(), out)
					return nil
				}
				return recipeup.Run(app)
			}
			name := args[0]
			items := args[1:]
			defs, err := loadRecipeDefinitions(app)
			if err != nil {
				return err
			}
			def, ok := recipe.Find(defs, name)
			if !ok {
				return fmt.Errorf("recipe %q not found (available: %s)", name, recipe.JoinNames(defs))
			}
			plan, err := planRecipe(app, def.Recipe, recipe.PlanOptions{
				Items:     items,
				Workspace: workspaceName,
				Detach:    detach,
			})
			if err != nil {
				return err
			}
			rendered := recipe.RenderPlan(plan)
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			if dryRun {
				return nil
			}
			if !yes {
				ok, err := confirmRecipeRun()
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}
			return recipe.Execute(app.Runner, app.WorkspaceStore, plan)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "run without interactive confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and exit")
	cmd.Flags().BoolVar(&detach, "detach", false, "create/reconcile without attaching after execution")
	cmd.Flags().StringVar(&workspaceName, "workspace", "", "override planned workspace name")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "print deterministic recipe runner snapshot")
	return cmd
}

func planRecipe(app *apppkg.App, r recipe.Recipe, opts recipe.PlanOptions) (recipe.Plan, error) {
	dir, err := os.Getwd()
	if err != nil {
		dir = os.Getenv("HOME")
	}
	if opts.CWD == "" {
		opts.CWD = dir
	}
	opts.Bin = app.Profile.Name
	opts.InsideZmux = app.Runner.IsInsideTmux()
	state := recipe.State{
		Sessions:   map[string]recipe.SessionState{},
		Workspaces: map[string]recipe.WorkspaceState{},
	}
	if app.Runner.ServerRunning() {
		state, err = recipe.BuildState(app.Runner, app.WorkspaceStore)
		if err != nil {
			return recipe.Plan{}, err
		}
	}
	return recipe.PlanRecipe(r, opts, state)
}

func confirmRecipeRun() (bool, error) {
	fmt.Fprint(os.Stderr, "Run this recipe plan? [y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}
