// Package snapshot bundles terminal/TUI evidence: per-pane text and ANSI
// captures, pane metadata, and an optional strict PNG screenshot of the current
// desktop terminal. It is the Go-native, provider-agnostic port of the prior
// pi-parley vision_snapshot tool (see docs/reference/terminal-snapshot-correlation-proposal.md).
//
// All side effects go through interfaces: tmux.Runner for captures, config.FS
// for file writes, a TargetResolver for screenshot geometry (satisfied by
// terminal.Resolver), and a Shooter for the screenshot tool itself.
package snapshot

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/terminal"
	"github.com/donjor/zmux/internal/tmux"
)

const (
	// SchemaVersion identifies the snapshot.json/manifest.json contract.
	SchemaVersion = "zmux-snapshot/v1"
	// Type is the stable discriminator written into the bundle artifacts.
	Type = "zmux_snapshot"

	defaultLines = 120
	minLines     = 1
	maxLines     = 2000
)

// PaneTarget is a pane to capture, with a stable artifact name.
type PaneTarget struct {
	Name   string
	PaneID string
}

// PaneArtifact records the files written for a captured pane.
type PaneArtifact struct {
	Name     string `json:"name"`
	PaneID   string `json:"paneId"`
	AnsiPath string `json:"ansiPath,omitempty"`
	TextPath string `json:"textPath,omitempty"`
	MetaPath string `json:"metaPath"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// Result is the machine-readable outcome, written verbatim to snapshot.json.
type Result struct {
	SchemaVersion  string         `json:"schemaVersion"`
	Type           string         `json:"type"`
	OK             bool           `json:"ok"`
	Dir            string         `json:"dir"`
	CreatedAt      string         `json:"createdAt"`
	Modalities     []string       `json:"modalities"`
	Panes          []PaneArtifact `json:"panes"`
	ScreenshotPath string         `json:"screenshotPath,omitempty"`
	Warnings       []string       `json:"warnings"`
	ViewCommands   []string       `json:"viewCommands"`
}

// Options configures a single capture. Dir is the full output directory; the
// caller (CLI) owns path/timestamp policy. Panes must be resolved by the caller.
type Options struct {
	Dir      string
	Panes    []PaneTarget
	Lines    int
	PNG      bool
	Warnings []string // seed warnings (e.g. caller-detected PNG/pane mismatch)
}

// TargetResolver resolves the screenshot target for the current tmux client.
// terminal.Resolver satisfies it.
type TargetResolver interface {
	Resolve(ctx context.Context) (terminal.Result, error)
}

// Shooter captures a desktop rectangle to a PNG file.
type Shooter interface {
	// Shoot writes a PNG of the given WM geometry ("x,y WxH") to outPath.
	Shoot(geometry, outPath string) error
}

// Snapshotter orchestrates a capture. All fields are injected; Now defaults to
// time.Now when nil.
type Snapshotter struct {
	Runner   tmux.Runner
	FS       config.FS
	Resolver TargetResolver
	Shooter  Shooter
	Now      func() time.Time
}

func (s Snapshotter) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Stamp renders a filesystem-safe timestamp for a snapshot directory name,
// e.g. 2026-05-25T08-30-00-000Z. Exported so callers building the default
// output path use the same convention.
func Stamp(t time.Time) string {
	iso := t.UTC().Format("2006-01-02T15:04:05.000Z")
	return strings.NewReplacer(":", "-", ".", "-").Replace(iso)
}

func clampLines(lines int) int {
	if lines <= 0 {
		return defaultLines
	}
	if lines < minLines {
		return minLines
	}
	if lines > maxLines {
		return maxLines
	}
	return lines
}

// Capture runs the snapshot: per-pane text/ANSI/meta, optional PNG, and the
// snapshot.json + manifest.json + README.md bundle. It returns an error only on
// a hard failure (e.g. cannot create the output directory); capture failures of
// individual modalities are recorded as warnings with OK reflecting whether any
// modality succeeded.
func (s Snapshotter) Capture(ctx context.Context, opts Options) (Result, error) {
	createdAt := s.now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err := s.FS.MkdirAll(opts.Dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create snapshot dir %q: %w", opts.Dir, err)
	}

	warnings := append([]string{}, opts.Warnings...)
	lines := clampLines(opts.Lines)

	if !s.Runner.IsInsideTmux() {
		warnings = append(warnings, "not inside tmux; pane text/ANSI capture may be unavailable")
	}
	if len(opts.Panes) == 0 {
		warnings = append(warnings, "no panes resolved to capture; pass --pane or run inside tmux")
	}

	var (
		panes      []PaneArtifact
		modalities []string
	)
	seenModality := map[string]bool{}
	addModality := func(m string) {
		if !seenModality[m] {
			seenModality[m] = true
			modalities = append(modalities, m)
		}
	}

	stemSeen := map[string]int{}
	for _, target := range opts.Panes {
		stem := uniqueStem(paneFileStem(target.Name), stemSeen)
		pane, ws := s.capturePane(opts.Dir, stem, target, lines)
		warnings = append(warnings, ws...)
		if pane == nil {
			continue
		}
		panes = append(panes, *pane)
		if pane.TextPath != "" {
			addModality("tmux_text")
		}
		if pane.AnsiPath != "" {
			addModality("tmux_ansi")
		}
	}

	var screenshotPath string
	if opts.PNG {
		path, ws := s.createScreenshot(ctx, opts.Dir)
		warnings = append(warnings, ws...)
		if path != "" {
			screenshotPath = path
			addModality("screenshot_png")
		}
	}

	result := Result{
		SchemaVersion:  SchemaVersion,
		Type:           Type,
		OK:             len(modalities) > 0,
		Dir:            opts.Dir,
		CreatedAt:      createdAt,
		Modalities:     modalities,
		Panes:          panes,
		ScreenshotPath: screenshotPath,
		Warnings:       warnings,
		ViewCommands:   viewCommands(panes, screenshotPath),
	}

	if err := s.writeBundle(result); err != nil {
		return result, fmt.Errorf("write snapshot bundle: %w", err)
	}
	return result, nil
}

func viewCommands(panes []PaneArtifact, screenshotPath string) []string {
	var cmds []string
	for _, pane := range panes {
		if pane.AnsiPath != "" {
			cmds = append(cmds, "less -R "+shellArg(pane.AnsiPath))
		}
		if pane.TextPath != "" {
			cmds = append(cmds, "less "+shellArg(pane.TextPath))
		}
	}
	if screenshotPath != "" {
		cmds = append(cmds, "xdg-open "+shellArg(screenshotPath))
	}
	return cmds
}

func shellArg(path string) string {
	if strings.ContainsAny(path, " \t") {
		return fmt.Sprintf("%q", path)
	}
	return path
}

func join(dir, name string) string { return filepath.Join(dir, name) }
