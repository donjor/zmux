package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
)

const paneAutoSize = "auto"

type paneOpenFlags struct {
	target   string
	cwd      string
	name     string
	size     string
	right    string
	left     string
	down     string
	up       string
	labelTab bool
}

type paneListFlags struct {
	target  string
	session bool
	all     bool
	quiet   bool
	json    bool
}

type paneResizeFlags struct {
	size   string
	width  string
	height string
}

type paneCurrentFlags struct {
	json bool
}

type paneToggleFlags struct {
	paneOpenFlags
	focus   bool
	replace bool
}

var paneCmd = &cobra.Command{
	Use:   "pane",
	Short: "Manage tmux panes with zmux-native commands",
}

func newPaneOpenCmd() *cobra.Command {
	flags := &paneOpenFlags{}
	cmd := &cobra.Command{
		Use:     "open [name] [-- command...]",
		Aliases: []string{"split"},
		Short:   "Open a new pane and optionally run a command",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneOpen(cmd, flags, args)
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
	for _, name := range []string{"right", "left", "down", "up"} {
		cmd.Flags().Lookup(name).NoOptDefVal = paneAutoSize
	}
}

func newPaneListCmd() *cobra.Command {
	flags := &paneListFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List panes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneList(cmd, flags)
		},
	}
	addPaneListFlags(cmd, flags)
	return cmd
}

func newTopLevelPaneListCmd(use string) *cobra.Command {
	flags := &paneListFlags{}
	cmd := &cobra.Command{
		Use:   use,
		Short: "List panes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneList(cmd, flags)
		},
	}
	addPaneListFlags(cmd, flags)
	return cmd
}

func addPaneListFlags(cmd *cobra.Command, flags *paneListFlags) {
	cmd.Flags().StringVar(&flags.target, "target", "", "target session/window/pane")
	cmd.Flags().BoolVar(&flags.session, "session", false, "list all panes in the current or target session")
	cmd.Flags().BoolVar(&flags.all, "all", false, "list panes across all sessions")
	cmd.Flags().BoolVarP(&flags.quiet, "quiet", "q", false, "print only pane ids")
	cmd.Flags().BoolVar(&flags.json, "json", false, "print pane data as JSON")
}

func newPaneCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <pane>",
		Short: "Close a pane by id, target, title, or index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolvePaneSelector(args[0])
			if err != nil {
				return err
			}
			return app.Runner.KillPane(target)
		},
	}
	return cmd
}

func newPaneFocusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "focus <pane>",
		Short: "Focus a pane by id, target, title, or index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolvePaneSelector(args[0])
			if err != nil {
				return err
			}
			return app.Runner.SelectPane(target)
		},
	}
	return cmd
}

func newPaneCurrentCmd() *cobra.Command {
	flags := &paneCurrentFlags{}
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the current pane id",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneCurrent(cmd, flags)
		},
	}
	cmd.Flags().BoolVar(&flags.json, "json", false, "print current pane data as JSON")
	return cmd
}

func newPaneToggleCmd() *cobra.Command {
	flags := &paneToggleFlags{}
	cmd := &cobra.Command{
		Use:   "toggle <name> [-- command...]",
		Short: "Toggle a named pane: close if present, open if absent",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneToggle(cmd, flags, args)
		},
	}
	addPaneOpenFlags(cmd, &flags.paneOpenFlags, false)
	cmd.Flags().BoolVar(&flags.focus, "focus", false, "focus an existing pane instead of closing it")
	cmd.Flags().BoolVar(&flags.replace, "replace", false, "close an existing pane and open a fresh one")
	return cmd
}

func newPaneResizeCmd() *cobra.Command {
	flags := &paneResizeFlags{}
	cmd := &cobra.Command{
		Use:   "resize <pane>",
		Short: "Resize a pane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneResize(flags, args[0])
		},
	}
	cmd.Flags().StringVar(&flags.size, "size", "", "set pane width, e.g. 40% or 80 cells")
	cmd.Flags().StringVar(&flags.width, "width", "", "set pane width")
	cmd.Flags().StringVar(&flags.height, "height", "", "set pane height")
	return cmd
}

func runPaneOpen(cmd *cobra.Command, flags *paneOpenFlags, args []string) error {
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
		ensureWindowLabelForPane(target)
	}

	paneID, err := app.Runner.SplitPane(tmux.SplitPaneOptions{
		Target:    target,
		Direction: direction,
		Size:      size,
		CWD:       cwd,
		Title:     name,
		Command:   command,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), paneID)
	return nil
}

func ensureWindowLabelForPane(target string) {
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

func runPaneCurrent(cmd *cobra.Command, flags *paneCurrentFlags) error {
	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return fmt.Errorf("pane current requires tmux")
	}
	if !flags.json {
		fmt.Fprintln(cmd.OutOrStdout(), paneID)
		return nil
	}
	panes, err := app.Runner.ListWindowPanes("")
	if err != nil {
		return err
	}
	for _, pane := range panes {
		if pane.ID == paneID {
			encoded, err := json.MarshalIndent(pane, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		}
	}
	encoded, err := json.MarshalIndent(tmux.Pane{ID: paneID}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
	return nil
}

func runPaneToggle(cmd *cobra.Command, flags *paneToggleFlags, args []string) error {
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
	existing, found, err := findPaneByName(name, flags.target)
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
	return runPaneOpen(cmd, &openFlags, command)
}

func runPaneList(cmd *cobra.Command, flags *paneListFlags) error {
	panes, err := loadPanesForList(flags)
	if err != nil {
		return err
	}
	if flags.json {
		encoded, err := json.MarshalIndent(panes, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	if flags.quiet {
		for _, pane := range panes {
			fmt.Fprintln(cmd.OutOrStdout(), pane.ID)
		}
		return nil
	}

	callerPane := os.Getenv("TMUX_PANE")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCALLER\tACTIVE\tSESSION\tWIN\tIDX\tTITLE\tCMD\tSIZE\tCWD")
	for _, pane := range panes {
		caller := ""
		if pane.ID == callerPane {
			caller = "you"
		}
		active := ""
		if pane.Active {
			active = "*"
		}
		title := pane.Title
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%dx%d\t%s\n",
			pane.ID, caller, active, pane.Session, pane.WindowIndex, pane.Index, title, pane.Command, pane.Width, pane.Height, pane.Dir)
	}
	return w.Flush()
}

func loadPanesForList(flags *paneListFlags) ([]tmux.Pane, error) {
	if flags.all && flags.session {
		return nil, fmt.Errorf("choose only one of --all or --session")
	}
	if flags.all && flags.target != "" {
		return nil, fmt.Errorf("--all cannot be combined with --target")
	}
	if flags.all {
		return app.Runner.ListAllPanes()
	}
	if flags.session {
		return app.Runner.ListPanes(flags.target)
	}
	return app.Runner.ListWindowPanes(flags.target)
}

func runPaneResize(flags *paneResizeFlags, selector string) error {
	target, err := resolvePaneSelector(selector)
	if err != nil {
		return err
	}
	axis := "width"
	size := flags.size
	selected := 0
	if flags.size != "" {
		selected++
	}
	if flags.width != "" {
		selected++
		axis = "width"
		size = flags.width
	}
	if flags.height != "" {
		selected++
		axis = "height"
		size = flags.height
	}
	if selected == 0 {
		return fmt.Errorf("pane resize requires --size, --width, or --height")
	}
	if selected > 1 {
		return fmt.Errorf("choose only one of --size, --width, or --height")
	}
	return app.Runner.ResizePane(target, axis, size)
}

func findPaneByName(name, target string) (tmux.Pane, bool, error) {
	panes, err := app.Runner.ListWindowPanes(target)
	if err != nil {
		return tmux.Pane{}, false, err
	}
	var matches []tmux.Pane
	for _, pane := range panes {
		if pane.Title == name {
			matches = append(matches, pane)
		}
	}
	if len(matches) == 0 {
		return tmux.Pane{}, false, nil
	}
	if len(matches) > 1 {
		return tmux.Pane{}, false, fmt.Errorf("pane %q is ambiguous (%d matches); use a pane id", name, len(matches))
	}
	return matches[0], true, nil
}

func resolvePaneSelector(selector string) (string, error) {
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
	var matches []tmux.Pane
	for _, pane := range panes {
		if pane.Title == selector || strconv.Itoa(pane.Index) == selector {
			matches = append(matches, pane)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("pane %q not found", selector)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("pane %q is ambiguous (%d matches); use a pane id", selector, len(matches))
	}
	return matches[0].ID, nil
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

func init() {
	paneCmd.AddCommand(newPaneOpenCmd())
	paneCmd.AddCommand(newPaneListCmd())
	paneCmd.AddCommand(newPaneCurrentCmd())
	paneCmd.AddCommand(newPaneToggleCmd())
	paneCmd.AddCommand(newPaneCloseCmd())
	paneCmd.AddCommand(newPaneFocusCmd())
	paneCmd.AddCommand(newPaneResizeCmd())
	rootCmd.AddCommand(paneCmd)
	rootCmd.AddCommand(newTopLevelPaneListCmd("panes"))
	rootCmd.AddCommand(newTopLevelPaneListCmd("list-panes"))
}
