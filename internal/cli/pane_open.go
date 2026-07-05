package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newPaneOpenCmd(app *apppkg.App) *cobra.Command {
	flags := &paneOpenFlags{}
	cmd := &cobra.Command{
		Use:     "open [name] [-- command...]",
		Aliases: []string{"split"},
		Short:   "Open a new pane and optionally run a command",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneOpen(app, cmd, flags, args)
		},
	}
	addPaneOpenFlags(cmd, flags, true)
	return cmd
}

func addPaneOpenFlags(cmd *cobra.Command, flags *paneOpenFlags, includeName bool) {
	cmd.Flags().StringVar(&flags.target, "target", "", "target pane/window (defaults to current tmux pane)")
	cmd.Flags().StringVarP(&flags.cwd, "cwd", "c", "", "working directory for the new pane (defaults to current directory)")
	if includeName {
		cmd.Flags().StringVarP(&flags.name, "name", "n", "", "pane name/title (defaults to positional name)")
	}
	cmd.Flags().StringVar(&flags.size, "size", "", "pane size, e.g. 40% or 80 cells")
	cmd.Flags().StringVarP(&flags.right, "right", "r", "", "split right; optional shorthand size, e.g. -r 40")
	cmd.Flags().StringVarP(&flags.left, "left", "l", "", "split left; optional shorthand size")
	cmd.Flags().StringVarP(&flags.down, "down", "d", "", "split below; optional shorthand size")
	cmd.Flags().StringVarP(&flags.up, "up", "u", "", "split above; optional shorthand size")
	cmd.Flags().BoolVar(&flags.labelTab, "label-tab", false, "preserve current tab name as a zmux label before opening the pane")
	cmd.Flags().BoolVar(&flags.noFocus, "no-focus", false, "create the pane without selecting it (agent/tool path)")
	for _, name := range []string{"right", "left", "down", "up"} {
		cmd.Flags().Lookup(name).NoOptDefVal = paneAutoSize
	}
}

func newPaneToggleCmd(app *apppkg.App) *cobra.Command {
	flags := &paneToggleFlags{}
	cmd := &cobra.Command{
		Use:   "toggle <name> [-- command...]",
		Short: "Toggle a named pane: close if present, open if absent",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneToggle(app, cmd, flags, args)
		},
	}
	addPaneOpenFlags(cmd, &flags.paneOpenFlags, false)
	cmd.Flags().BoolVar(&flags.focus, "focus", false, "focus an existing pane instead of closing it")
	cmd.Flags().BoolVar(&flags.replace, "replace", false, "close an existing pane and open a fresh one")
	return cmd
}

func runPaneOpen(app *apppkg.App, cmd *cobra.Command, flags *paneOpenFlags, args []string) error {
	name, command, err := splitPaneOpenArgs(cmd, args, flags.name != "")
	if err != nil {
		return err
	}
	if flags.name != "" {
		if name != "" && name != flags.name {
			return fmt.Errorf("pane name specified twice: %q and %q", name, flags.name)
		}
		name = flags.name
	}

	consumePaneDirectionSizeArg(cmd, flags, &command)
	direction, size, err := paneOpenDirectionAndSize(cmd, flags)
	if err != nil {
		return err
	}
	target := flags.target
	if target == "" {
		if !app.Runner.IsInsideTmux() {
			return fmt.Errorf("pane open requires tmux; run inside tmux or pass --target")
		}
		target = os.Getenv("TMUX_PANE")
	}

	cwd := flags.cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	cwd, err = normalizePaneCWD(cwd)
	if err != nil {
		return err
	}

	if flags.labelTab {
		ensureWindowLabelForPane(app, target)
	}

	paneID, err := app.Runner.SplitPane(tmux.SplitPaneOptions{
		Target:    target,
		Direction: direction,
		Size:      size,
		CWD:       cwd,
		Title:     name,
		Command:   command,
		Detached:  flags.noFocus,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), paneID)
	return nil
}

func runPaneToggle(app *apppkg.App, cmd *cobra.Command, flags *paneToggleFlags, args []string) error {
	if flags.focus && flags.replace {
		return fmt.Errorf("choose only one of --focus or --replace")
	}
	name, command, err := splitPaneOpenArgs(cmd, args, false)
	if err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("pane toggle requires a pane name")
	}
	consumePaneDirectionSizeArg(cmd, &flags.paneOpenFlags, &command)
	existing, found, err := findPaneByName(app, name, flags.target)
	if err != nil {
		return err
	}
	if found {
		if flags.focus {
			if err := app.Runner.SelectPane(existing.ID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), existing.ID)
			return nil
		}
		if err := app.Runner.KillPane(existing.ID); err != nil {
			return err
		}
		if !flags.replace {
			fmt.Fprintln(cmd.OutOrStdout(), existing.ID)
			return nil
		}
	}
	openFlags := flags.paneOpenFlags
	openFlags.name = name
	return runPaneOpen(app, cmd, &openFlags, command)
}

func ensureWindowLabelForPane(app *apppkg.App, target string) {
	out, err := app.Runner.DisplayMessage(target, fmt.Sprintf("zmux\t#{%s}\t#{window_name}", tablabel.Option))
	if err != nil {
		return
	}
	fields := strings.SplitN(strings.TrimRight(out, "\r\n"), "\t", 3)
	if len(fields) != 3 || fields[0] != "zmux" {
		return
	}
	label := strings.TrimSpace(fields[1])
	windowName := strings.TrimSpace(fields[2])
	if label != "" || windowName == "" {
		return
	}
	_ = app.Runner.SetWindowOption(target, tablabel.Option, windowName)
	_ = app.Runner.SetWindowOption(target, tablabel.SourceOption, tablabel.SourcePane)
}

// splitPaneOpenArgs separates the positional pane name (if any) from
// the trailing command. With --name set, all positional args become
// the command. Without --name, the first arg is the name.
func splitPaneOpenArgs(_ *cobra.Command, args []string, nameFromFlag bool) (string, []string, error) {
	if nameFromFlag {
		return "", args, nil
	}
	if len(args) == 0 {
		return "", nil, nil
	}
	if len(args) == 1 {
		return args[0], nil, nil
	}
	return args[0], args[1:], nil
}

// consumePaneDirectionSizeArg lets `-r 40` consume an inline-style
// numeric size from the command tail when the user wrote `-r` with
// the bare auto-size sentinel.
func consumePaneDirectionSizeArg(cmd *cobra.Command, flags *paneOpenFlags, command *[]string) {
	if len(*command) == 0 || !isPaneSizeToken((*command)[0]) {
		return
	}
	for _, name := range []string{"right", "left", "down", "up"} {
		if cmd.Flags().Changed(name) {
			switch name {
			case "right":
				if flags.right == paneAutoSize {
					flags.right = (*command)[0]
					*command = (*command)[1:]
				}
			case "left":
				if flags.left == paneAutoSize {
					flags.left = (*command)[0]
					*command = (*command)[1:]
				}
			case "down":
				if flags.down == paneAutoSize {
					flags.down = (*command)[0]
					*command = (*command)[1:]
				}
			case "up":
				if flags.up == paneAutoSize {
					flags.up = (*command)[0]
					*command = (*command)[1:]
				}
			}
			return
		}
	}
}

func isPaneSizeToken(value string) bool {
	trimmed := strings.TrimSuffix(value, "%")
	if trimmed == "" {
		return false
	}
	_, err := strconv.Atoi(trimmed)
	return err == nil
}

func paneOpenDirectionAndSize(cmd *cobra.Command, flags *paneOpenFlags) (tmux.SplitDirection, string, error) {
	type directionFlag struct {
		name      string
		value     string
		direction tmux.SplitDirection
	}
	all := []directionFlag{
		{name: "right", value: flags.right, direction: tmux.SplitRight},
		{name: "left", value: flags.left, direction: tmux.SplitLeft},
		{name: "down", value: flags.down, direction: tmux.SplitDown},
		{name: "up", value: flags.up, direction: tmux.SplitUp},
	}
	direction := tmux.SplitRight
	inlineSize := ""
	selected := 0
	for _, flag := range all {
		if cmd.Flags().Changed(flag.name) {
			selected++
			direction = flag.direction
			inlineSize = flag.value
		}
	}
	if selected > 1 {
		return "", "", fmt.Errorf("choose only one split direction")
	}
	if inlineSize != "" && inlineSize != paneAutoSize && flags.size != "" {
		return "", "", fmt.Errorf("direction size and --size are mutually exclusive")
	}
	size := flags.size
	if inlineSize != "" && inlineSize != paneAutoSize {
		size = normalizePaneDirectionSize(inlineSize)
	}
	return direction, size, nil
}

func normalizePaneDirectionSize(size string) string {
	if strings.HasSuffix(size, "%") {
		return size
	}
	if n, err := strconv.Atoi(size); err == nil && n > 0 && n <= 100 {
		return fmt.Sprintf("%d%%", n)
	}
	return size
}

func normalizePaneCWD(cwd string) (string, error) {
	if cwd == "" {
		return "", nil
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("--cwd %s: %w", cwd, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--cwd %s is not a directory", cwd)
	}
	return abs, nil
}
