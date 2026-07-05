// Package setup ports zmux's shell integration from install.sh into Go.
//
// It models setup as a pure Plan (a set of desired Edits) that Apply
// reconciles against the filesystem behind config.FS. Each managed edit lives
// inside an idempotent marker block:
//
//	# >>> zmux-managed >>>
//	<block>
//	# <<< zmux-managed <<<
//
// so it can be cleanly added, updated, or removed, and Apply backs up the prior
// file to <path>.bak before writing. This improves on the legacy bash, which
// appended with a grep-guard and could not cleanly remove its lines.
package setup

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

const (
	markerBegin = "# >>> zmux-managed >>>"
	markerEnd   = "# <<< zmux-managed <<<"
)

// Action is what an Edit does to its target file.
type Action int

const (
	// ActionAdd ensures the managed block is present with the given content.
	ActionAdd Action = iota
	// ActionRemove ensures the managed block is absent.
	ActionRemove
)

// Edit is a single managed change to a file. It is pure data — computing it
// requires no disk access, which keeps Plan unit-testable.
type Edit struct {
	// File is the absolute path to edit.
	File string
	// Label is a short human description (e.g. "shell integration (.zshrc)").
	Label string
	// Block is the managed content (without markers). Required for ActionAdd.
	Block string
	// Action is add or remove.
	Action Action
}

// Plan is an ordered set of edits.
type Plan struct {
	Edits []Edit
}

// ApplyOptions controls how a plan is written.
type ApplyOptions struct {
	// DryRun computes results without touching disk.
	DryRun bool
	// Backup writes <path>.bak before modifying an existing file.
	Backup bool
}

// Result reports what Apply did (or would do) for one edit.
type Result struct {
	Edit    Edit
	Changed bool   // whether the file content would change
	Note    string // "added", "updated", "removed", "no change", "would add", ...
}

// Apply reconciles the plan against the filesystem via fs. With DryRun it only
// reports what would change.
func (p Plan) Apply(fs config.FS, opts ApplyOptions) ([]Result, error) {
	results := make([]Result, 0, len(p.Edits))
	for _, e := range p.Edits {
		res, err := applyEdit(fs, e, opts)
		if err != nil {
			return results, err
		}
		results = append(results, res)
	}
	return results, nil
}

func applyEdit(fs config.FS, e Edit, opts ApplyOptions) (Result, error) {
	var existing string
	if data, err := fs.ReadFile(e.File); err == nil {
		existing = string(data)
	}

	var next string
	switch e.Action {
	case ActionRemove:
		next = removeBlock(existing)
	default:
		next = upsertBlock(existing, e.Block)
	}

	if next == existing {
		return Result{Edit: e, Changed: false, Note: "no change"}, nil
	}

	present, past := changeVerbs(existing, e)
	if opts.DryRun {
		return Result{Edit: e, Changed: true, Note: "would " + present}, nil
	}

	if opts.Backup && existing != "" {
		if err := fs.WriteFile(e.File+".bak", []byte(existing), 0o644); err != nil {
			return Result{}, fmt.Errorf("backup %s: %w", e.File, err)
		}
	}
	if dir := filepath.Dir(e.File); dir != "." && dir != "" {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create parent %s: %w", dir, err)
		}
	}
	if err := fs.WriteFile(e.File, []byte(next), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", e.File, err)
	}
	return Result{Edit: e, Changed: true, Note: past}, nil
}

// changeVerbs returns the present- and past-tense verbs for an edit given the
// file's current content ("add"/"added", "update"/"updated", "remove"/"removed").
func changeVerbs(existing string, e Edit) (present, past string) {
	if e.Action == ActionRemove {
		return "remove", "removed"
	}
	if hasBlock(existing) {
		return "update", "updated"
	}
	return "add", "added"
}

// hasBlock reports whether content contains a zmux-managed block.
func hasBlock(content string) bool {
	return strings.Contains(content, markerBegin) && strings.Contains(content, markerEnd)
}

// ManagedBlock returns the content inside the zmux-managed markers, without the
// markers themselves. It is intended for doctor/status checks; Apply remains
// the only writer of managed blocks.
func ManagedBlock(content string) (string, bool) {
	if !hasBlock(content) {
		return "", false
	}
	start := strings.Index(content, markerBegin)
	end := strings.Index(content, markerEnd)
	if start < 0 || end < 0 || end < start {
		return "", false
	}
	start += len(markerBegin)
	block := content[start:end]
	block = strings.TrimPrefix(block, "\n")
	block = strings.TrimSuffix(block, "\n")
	return block, true
}

// upsertBlock returns content with the managed block set to block. If a managed
// block already exists it is replaced in place; otherwise the block is appended.
func upsertBlock(content, block string) string {
	managed := markerBegin + "\n" + strings.TrimRight(block, "\n") + "\n" + markerEnd + "\n"
	if hasBlock(content) {
		before, after := splitAroundBlock(content)
		return before + managed + after
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + managed
}

// removeBlock returns content with any managed block removed.
func removeBlock(content string) string {
	if !hasBlock(content) {
		return content
	}
	before, after := splitAroundBlock(content)
	// Collapse the blank line that preceded the block, if any.
	before = strings.TrimRight(before, "\n")
	if before != "" {
		before += "\n"
	}
	after = strings.TrimLeft(after, "\n")
	if before != "" && after != "" {
		return before + after
	}
	return before + after
}

// splitAroundBlock returns the content before markerBegin and after markerEnd.
func splitAroundBlock(content string) (before, after string) {
	start := strings.Index(content, markerBegin)
	end := strings.Index(content, markerEnd)
	if start < 0 || end < 0 || end < start {
		return content, ""
	}
	end += len(markerEnd)
	// Consume a trailing newline immediately after the end marker.
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:start], content[end:]
}
