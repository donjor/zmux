package cli

import (
	"fmt"
	"os"

	"github.com/donjor/zmux/internal/keys"
	"github.com/spf13/cobra"
)

// newKeysCmd builds the hidden `keys` command group — maintainer tooling for the
// keybinding registry. `keys gen` regenerates docs/reference/keybindings.md from
// internal/keys; `--check` verifies it is up to date (used in CI).
//
// This operates on a repo working-tree file (not user config), so it uses os
// directly rather than the app's config.FS abstraction.
func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "keys",
		Short:  "Keybinding registry tools",
		Hidden: true,
	}

	var check bool
	var output string

	gen := &cobra.Command{
		Use:   "gen",
		Short: "Generate docs/reference/keybindings.md from the keys registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := keys.GenerateDoc()
			if err != nil {
				return err
			}
			if check {
				existing, err := os.ReadFile(output)
				if err != nil {
					return fmt.Errorf("read %s: %w", output, err)
				}
				if string(existing) != doc {
					return fmt.Errorf("%s is out of date — run `zmux keys gen` and commit the result", output)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date\n", output)
				return nil
			}
			if err := os.WriteFile(output, []byte(doc), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", output, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
			return nil
		},
	}
	gen.Flags().BoolVar(&check, "check", false, "verify the doc is up to date (exit non-zero on drift)")
	gen.Flags().StringVar(&output, "output", "docs/reference/keybindings.md", "output path for the generated doc")

	cmd.AddCommand(gen)
	return cmd
}
