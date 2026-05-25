package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/procfs"
	"github.com/donjor/zmux/internal/snapshot"
	"github.com/donjor/zmux/internal/terminal"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/wm"
	"github.com/spf13/cobra"
)

type snapshotFlags struct {
	panes []string
	lines int
	noPNG bool
	out   string
	json  bool
}

func newSnapshotCmd(app *apppkg.App) *cobra.Command {
	// Production desktop-window deps; the screenshot path mirrors `zmux terminal current`.
	return newSnapshotCmdWith(app, wm.NewHyprlandAdapter(), procfs.LinuxInspector{}, snapshot.GrimShooter{})
}

func newSnapshotCmdWith(app *apppkg.App, adapter wm.Adapter, process procfs.Inspector, shooter snapshot.Shooter) *cobra.Command {
	flags := &snapshotFlags{lines: 120}
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture terminal evidence: per-pane text/ANSI + optional PNG screenshot",
		Long: `Bundle terminal/TUI evidence for the current zmux session.

For each pane it captures plain text and ANSI-coloured text plus metadata, and
(by default) a strict PNG screenshot of the current desktop terminal. Artifacts
land in ~/.zmux/snapshots/<timestamp>/ (override with --out) as pane.txt /
pane.ansi / pane.meta.json plus snapshot.json, manifest.json, and README.md.

With no --pane flags it captures every pane in the current window. The PNG only
ever covers the current terminal; if you target panes elsewhere with --pane it is
skipped (a screenshot would show a different window).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSnapshot(app, cmd, flags, adapter, process, shooter)
		},
	}
	cmd.Flags().StringArrayVar(&flags.panes, "pane", nil, "pane id to capture (repeatable; default: all panes in current window)")
	cmd.Flags().IntVar(&flags.lines, "lines", 120, "history lines to capture per pane (1-2000)")
	cmd.Flags().BoolVar(&flags.noPNG, "no-png", false, "skip the PNG screenshot of the current terminal")
	cmd.Flags().StringVar(&flags.out, "out", "", "output directory (default ~/.zmux/snapshots/<timestamp>)")
	cmd.Flags().BoolVar(&flags.json, "json", false, "print the snapshot result as JSON")
	return cmd
}

func runSnapshot(app *apppkg.App, cmd *cobra.Command, flags *snapshotFlags, adapter wm.Adapter, process procfs.Inspector, shooter snapshot.Shooter) error {
	currentPane := os.Getenv("TMUX_PANE")

	var seed []string
	targets, explicit, err := resolvePaneTargets(app.Runner, flags.panes)
	if err != nil {
		// Degrade to an empty capture (the snapshot records the reason) rather
		// than hard-failing an evidence command.
		seed = append(seed, fmt.Sprintf("pane discovery failed: %v", err))
		targets, explicit = nil, len(flags.panes) > 0
	}

	// Guard against false evidence: the PNG only covers the current terminal, so
	// it is meaningful only when every captured pane lives in the current window.
	png := !flags.noPNG
	if png && explicit {
		if ok, reason := allPanesInCurrentWindow(app.Runner, targets); !ok {
			png = false
			seed = append(seed, "PNG skipped: "+reason)
		}
	}

	dir := resolveOutDir(app.Profile.SnapshotsDir, flags.out)

	s := snapshot.Snapshotter{
		Runner:   app.Runner,
		FS:       app.FS,
		Resolver: terminal.Resolver{Runner: app.Runner, Adapter: adapter, Process: process, CurrentPaneID: currentPane},
		Shooter:  shooter,
	}
	result, err := s.Capture(context.Background(), snapshot.Options{
		Dir:      dir,
		Panes:    targets,
		Lines:    flags.lines,
		PNG:      png,
		Warnings: seed,
	})
	if err != nil {
		return err
	}

	if flags.json {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	renderSnapshot(cmd, result)
	return nil
}

// resolveOutDir returns the explicit --out dir, or <snapshotsDir>/<stamp> under
// the active profile's snapshots dir (~/.zmux/snapshots, or ~/.zzmux/snapshots).
func resolveOutDir(snapshotsDir, out string) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	return filepath.Join(snapshotsDir, snapshot.Stamp(time.Now()))
}

// resolvePaneTargets builds capture targets. Explicit --pane ids keep their id
// as the name source; with none given it captures all panes in the current
// window. Names are de-duplicated so artifacts never collide.
func resolvePaneTargets(runner tmux.Runner, paneIDs []string) (targets []snapshot.PaneTarget, explicit bool, err error) {
	dedupe := newNameDeduper()
	if len(paneIDs) > 0 {
		byID := paneCommandsByID(runner)
		for _, id := range paneIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			targets = append(targets, snapshot.PaneTarget{Name: dedupe(paneName(byID[id], id)), PaneID: id})
		}
		return targets, true, nil
	}
	panes, err := runner.ListWindowPanes("")
	if err != nil {
		return nil, false, fmt.Errorf("list current window panes: %w", err)
	}
	for _, p := range panes {
		targets = append(targets, snapshot.PaneTarget{Name: dedupe(paneName(p.Command, p.ID)), PaneID: p.ID})
	}
	return targets, false, nil
}

func paneCommandsByID(runner tmux.Runner) map[string]string {
	byID := map[string]string{}
	panes, err := runner.ListWindowPanes("")
	if err != nil {
		return byID
	}
	for _, p := range panes {
		byID[p.ID] = p.Command
	}
	return byID
}

// paneName prefers the pane's running command (short and stable, e.g. nvim,
// bash, server) for the artifact name, falling back to the pane id.
func paneName(command, id string) string {
	if c := strings.TrimSpace(command); c != "" {
		return c
	}
	return id
}

func newNameDeduper() func(string) string {
	seen := map[string]int{}
	return func(name string) string {
		n := seen[name]
		seen[name]++
		if n == 0 {
			return name
		}
		return fmt.Sprintf("%s-%d", name, n+1)
	}
}

// allPanesInCurrentWindow reports whether every target pane is in the current
// tmux window — the precondition for a PNG of the current terminal to be honest
// evidence for those panes.
func allPanesInCurrentWindow(runner tmux.Runner, targets []snapshot.PaneTarget) (bool, string) {
	panes, err := runner.ListWindowPanes("")
	if err != nil {
		return false, "cannot verify the requested panes are in the current window"
	}
	inWindow := make(map[string]bool, len(panes))
	for _, p := range panes {
		inWindow[p.ID] = true
	}
	for _, t := range targets {
		if !inWindow[t.PaneID] {
			return false, fmt.Sprintf("--pane %s is not in the current window; a screenshot would show a different terminal", t.PaneID)
		}
	}
	return true, ""
}

func renderSnapshot(cmd *cobra.Command, r snapshot.Result) {
	out := cmd.OutOrStdout()
	status := "✗"
	if r.OK {
		status = "✓"
	}
	fmt.Fprintf(out, "%s snapshot: %s\n", status, r.Dir)
	fmt.Fprintf(out, "modalities: %s\n", joinOrNone(r.Modalities))
	for _, p := range r.Panes {
		size := ""
		if p.Width > 0 && p.Height > 0 {
			size = " " + strconv.Itoa(p.Width) + "x" + strconv.Itoa(p.Height)
		}
		fmt.Fprintf(out, "  %s (%s%s)\n", p.Name, p.PaneID, size)
	}
	if r.ScreenshotPath != "" {
		fmt.Fprintf(out, "  png: %s\n", r.ScreenshotPath)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(out, "  ! %s\n", w)
	}
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	return strings.Join(ss, ", ")
}
