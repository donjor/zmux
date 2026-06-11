package cli

import (
	"fmt"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/recipe"
	"github.com/spf13/cobra"
)

func newRecipeCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Manage zmux recipes",
	}
	cmd.AddCommand(
		newRecipeListCmd(app),
		newRecipeShowCmd(app),
		newRecipeLintCmd(app),
		newRecipeForkCmd(app),
		newRecipeEditCmd(app),
		newRecipeCreateCmd(app),
	)
	return cmd
}

func newRecipeListCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available recipes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			defs, err := loadRecipeDefinitions(app)
			if err != nil {
				return err
			}
			for _, def := range defs {
				fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-8s %-9s %-7s %s\n", def.Recipe.Name, def.Source, def.Recipe.Kind, def.Recipe.Context, def.Recipe.Description)
			}
			return nil
		},
	}
}

func newRecipeShowCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <recipe>",
		Short: "Show a recipe TOML file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defs, err := loadRecipeDefinitions(app)
			if err != nil {
				return err
			}
			def, ok := recipe.Find(defs, args[0])
			if !ok {
				return fmt.Errorf("recipe %q not found (available: %s)", args[0], recipe.JoinNames(defs))
			}
			fmt.Fprint(cmd.OutOrStdout(), string(def.Raw))
			if len(def.Raw) == 0 || def.Raw[len(def.Raw)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newRecipeLintCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "lint [recipe...]",
		Short: "Validate recipes",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(app.FS)
			if err != nil {
				cfg = config.DefaultConfig()
			}
			results := recipe.Lint(app.FS, recipe.ConfiguredDirs(app.FS, app.Profile, cfg), args)
			if len(results) == 0 {
				defs, _ := loadRecipeDefinitions(app)
				if len(args) > 0 {
					return fmt.Errorf("recipe %q not found (available: %s)", args[0], recipe.JoinNames(defs))
				}
				return fmt.Errorf("no recipes found")
			}
			for _, result := range results {
				if result.Err != nil {
					return fmt.Errorf("%s: %w", result.Path, result.Err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "ok %s\n", result.Name)
			}
			return nil
		},
	}
}

func newRecipeForkCmd(app *apppkg.App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "fork <bundled-recipe>",
		Short: "Copy a bundled recipe into your profile recipe directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var bundled *recipe.Definition
			for _, def := range recipe.Bundled() {
				if def.Recipe.Name == args[0] {
					copy := def
					bundled = &copy
					break
				}
			}
			if bundled == nil {
				return fmt.Errorf("bundled recipe %q not found", args[0])
			}
			path, err := recipe.Fork(app.FS, app.Profile, *bundled, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forked %s -> %s\n", args[0], path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing user recipe")
	return cmd
}

func newRecipeEditCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <recipe>",
		Short: "Edit a user recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defs, err := loadRecipeDefinitions(app)
			if err != nil {
				return err
			}
			def, ok := recipe.Find(defs, args[0])
			if !ok {
				return fmt.Errorf("recipe %q not found (create it with zmux recipe create %s)", args[0], args[0])
			}
			if def.Source == recipe.SourceBundled {
				return fmt.Errorf("recipe %q is bundled — run zmux recipe fork %s first", args[0], args[0])
			}
			return recipe.Edit(def.Path)
		},
	}
}

func newRecipeCreateCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a starter user recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := recipe.Slug(args[0])
			path, err := recipe.CreateStarter(app.FS, app.Profile, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", path)
			return nil
		},
	}
}

func loadRecipeDefinitions(app *apppkg.App) ([]recipe.Definition, error) {
	cfg, err := loadConfig(app.FS)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	dirs := recipe.ConfiguredDirs(app.FS, app.Profile, cfg)
	defs, err := recipe.Load(app.FS, dirs, cfg.Recipes.Disabled)
	if err != nil {
		return nil, err
	}
	return defs, nil
}
