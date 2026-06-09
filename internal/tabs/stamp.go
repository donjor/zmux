package tabs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
)

// NewID returns a fresh opaque tab id (ztab_<12 hex>, crypto/rand). Labels
// are mutable and duplicable; the id is what survives rename, join, break,
// hide and show.
func NewID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("tab id entropy: %w", err)
	}
	return "ztab_" + hex.EncodeToString(b[:]), nil
}

// Stamp claims a pane as a logical tab: ensures a pane-scoped id, writes the
// pane-canonical label, and mirrors the label to the window (presentation
// while full). One batched tmux invocation. Existing ids are preserved —
// stamping is idempotent on identity. Returns the tab id.
func Stamp(r tmux.Runner, paneID, windowTarget, label, source string) (string, error) {
	id, err := r.ShowPaneOption(paneID, OptTabID)
	if err != nil {
		return "", err
	}
	if id == "" {
		if id, err = NewID(); err != nil {
			return "", err
		}
	}
	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: paneID, Key: OptTabID, Value: id},
	}
	if label != "" {
		writes = append(writes,
			tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: tablabel.Option, Value: label},
			tmux.OptionWrite{Scope: tmux.ScopePane, Target: paneID, Key: tablabel.SourceOption, Value: source},
		)
		if windowTarget != "" {
			writes = append(writes,
				tmux.OptionWrite{Scope: tmux.ScopeWindow, Target: windowTarget, Key: tablabel.Option, Value: label},
				tmux.OptionWrite{Scope: tmux.ScopeWindow, Target: windowTarget, Key: tablabel.SourceOption, Value: source},
			)
		}
	}
	if err := r.ApplyOptions(writes); err != nil {
		return "", err
	}
	return id, nil
}

// MigrateWindowLabel claims a legacy window-labeled tab for a pane: copies
// the window-scoped label (scope-exact read — format reads can't tell window
// from pane values) onto the pane and stamps an id. No-op (returning "")
// when the window carries no label or the pane is already managed.
func MigrateWindowLabel(r tmux.Runner, windowTarget, paneID string) (string, error) {
	label, err := r.ShowWindowOption(windowTarget, tablabel.Option)
	if err != nil {
		return "", err
	}
	if label == "" {
		return "", nil
	}
	if id, err := r.ShowPaneOption(paneID, OptTabID); err != nil {
		return "", err
	} else if id != "" {
		return "", nil
	}
	source, err := r.ShowWindowOption(windowTarget, tablabel.SourceOption)
	if err != nil {
		return "", err
	}
	if source == "" {
		source = tablabel.SourceManual
	}
	return Stamp(r, paneID, windowTarget, label, source)
}
