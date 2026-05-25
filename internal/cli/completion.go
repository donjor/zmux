package cli

import (
	"os"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/spf13/cobra"
)

func newCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for zmux.

To load completions:

Bash:
  $ source <(zmux completion bash)
  # To load completions for each session, execute once:
  $ zmux completion bash > /etc/bash_completion.d/zmux

Zsh:
  $ source <(zmux completion zsh)
  # To load completions for each session, execute once:
  $ zmux completion zsh > "${fpath[1]}/_zmux"

Fish:
  $ zmux completion fish | source
  # To load completions for each session, execute once:
  $ zmux completion fish > ~/.config/fish/completions/zmux.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			}
			return nil
		},
	}
}

func registerThemeCompletions(themeSetCmd *cobra.Command) {
	themeSetCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return listThemeNames(), cobra.ShellCompDirectiveNoFileComp
	}
}

func registerBarCompletions(barCmd *cobra.Command) {
	barCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return presetNames(), cobra.ShellCompDirectiveNoFileComp
	}
}

func registerSyncCompletions(themePullCmd *cobra.Command) {
	themePullCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{"ghostty", "nvim"}, cobra.ShellCompDirectiveNoFileComp
	}
}

// listThemeNames returns all available theme names for completion.
func listThemeNames() []string {
	fs := &config.RealFS{}
	home, err := fs.UserHomeDir()
	if err != nil {
		return bundledThemeNames()
	}

	resolver := theme.NewResolver(
		fs,
		home+"/.zmux/themes",
		home+"/.zmux/themes/iterm2",
	)

	themes := resolver.List()
	names := make([]string, len(themes))
	for i, ti := range themes {
		names[i] = ti.Name
	}
	return names
}

// bundledThemeNames returns just the bundled theme names as a fallback.
func bundledThemeNames() []string {
	fs := &config.RealFS{}
	resolver := theme.NewResolver(fs, "", "")
	themes := resolver.List()
	names := make([]string, len(themes))
	for i, ti := range themes {
		names[i] = ti.Name
	}
	return names
}
