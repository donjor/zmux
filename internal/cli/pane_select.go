package cli

import (
	"fmt"
	"strconv"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newPaneCloseCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <pane>",
		Short: "Close a pane by id, target, title, or index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolvePaneSelector(app, args[0])
			if err != nil {
				return err
			}
			return app.Runner.KillPane(target)
		},
	}
	return cmd
}

func newPaneFocusCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "focus <pane>",
		Short: "Focus a pane by id, target, title, or index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolvePaneSelector(app, args[0])
			if err != nil {
				return err
			}
			return app.Runner.SelectPane(target)
		},
	}
	return cmd
}

// findPaneByName looks up a single pane by its title within the
// current/target window. Returns ok=false if no match; errors if
// the title matches more than one pane.
func findPaneByName(app *apppkg.App, name, target string) (tmux.Pane, bool, error) {
	panes, err := app.Runner.ListWindowPanes(target)
	if err != nil {
		return tmux.Pane{}, false, err
	}
	matches, err := matchingPanes(app, panes, name)
	if err != nil {
		return tmux.Pane{}, false, err
	}
	if len(matches) == 0 {
		return tmux.Pane{}, false, nil
	}
	if len(matches) > 1 {
		return tmux.Pane{}, false, fmt.Errorf("pane %q is ambiguous (%d matches); use a pane id", name, len(matches))
	}
	return matches[0], true, nil
}

// resolvePaneSelector accepts any of: a tmux pane id (%N), a target
// spec containing ':' or '.', a pane title, or an integer index, and
// returns a concrete pane id usable for tmux operations.
func resolvePaneSelector(app *apppkg.App, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("pane selector is required")
	}
	if strings.HasPrefix(selector, "%") || strings.Contains(selector, ":") || strings.Contains(selector, ".") {
		return selector, nil
	}

	panes, err := app.Runner.ListWindowPanes("")
	if err != nil {
		return "", err
	}
	matches, err := matchingPanes(app, panes, selector)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("pane %q not found", selector)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("pane %q is ambiguous (%d matches); use a pane id", selector, len(matches))
	}
	return matches[0].ID, nil
}

func matchingPanes(app *apppkg.App, panes []tmux.Pane, selector string) ([]tmux.Pane, error) {
	var matches []tmux.Pane
	seen := map[string]bool{}
	for _, pane := range panes {
		matched := pane.Title == selector || strconv.Itoa(pane.Index) == selector
		if !matched {
			name, err := app.Runner.ShowPaneOption(pane.ID, optPaneName)
			if err != nil {
				return nil, err
			}
			matched = name == selector
		}
		if matched && !seen[pane.ID] {
			seen[pane.ID] = true
			matches = append(matches, pane)
		}
	}
	return matches, nil
}
