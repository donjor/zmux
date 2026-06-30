package workspace

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// ForkSession creates a new managed workspace session by copying the source
// session's window names/order into a clean destination. It intentionally does
// not replay commands or pane layouts; the fork is topology only.
func ForkSession(runner tmux.Runner, store *Store, wsName, sourceSession, destLabel, dir string) (WorkspaceSession, error) {
	if runner == nil {
		return WorkspaceSession{}, fmt.Errorf("tmux runner unavailable")
	}
	if store == nil {
		return WorkspaceSession{}, fmt.Errorf("workspace store unavailable")
	}
	if strings.TrimSpace(sourceSession) == "" {
		return WorkspaceSession{}, fmt.Errorf("source session required")
	}

	sourceWindows, err := runner.ListWindows(sourceSession)
	if err != nil {
		return WorkspaceSession{}, fmt.Errorf("list source windows: %w", err)
	}
	if len(sourceWindows) == 0 {
		return WorkspaceSession{}, fmt.Errorf("source session %q has no windows", sourceSession)
	}

	rec, err := CreateManagedSession(runner, store, wsName, destLabel, dir)
	if err != nil {
		return WorkspaceSession{}, err
	}
	rollback := func(cause error) (WorkspaceSession, error) {
		_ = runner.KillSession(rec.TmuxName)
		_ = store.RemoveSession(rec.TmuxName)
		return WorkspaceSession{}, cause
	}

	destWindows, err := runner.ListWindows(rec.TmuxName)
	if err != nil {
		return rollback(fmt.Errorf("list destination windows: %w", err))
	}
	if len(destWindows) == 0 {
		return rollback(fmt.Errorf("destination session %q has no windows after creation", rec.TmuxName))
	}

	activePos := 0
	for i, w := range sourceWindows {
		if w.Active {
			activePos = i
			break
		}
	}

	first := destWindows[0]
	firstName := forkWindowName(sourceWindows[0], 1)
	firstIndex := strconv.Itoa(first.Index)
	if err := runner.RenameWindow(rec.TmuxName, firstIndex, firstName); err != nil {
		return rollback(fmt.Errorf("rename first window: %w", err))
	}
	firstPaneTarget := fmt.Sprintf("%s:%s", rec.TmuxName, firstIndex)
	firstPaneID, err := runner.DisplayMessage(firstPaneTarget, "#{pane_id}")
	if err != nil {
		return rollback(fmt.Errorf("resolve first forked pane: %w", err))
	}
	if err := stampForkedWindow(runner, firstPaneID, firstPaneTarget, sourceWindows[0]); err != nil {
		return rollback(fmt.Errorf("stamp first forked window: %w", err))
	}

	for i := 1; i < len(sourceWindows); i++ {
		w := sourceWindows[i]
		paneID, err := runner.NewWindow(rec.TmuxName, forkWindowName(w, i+1), dir, tmux.Detached())
		if err != nil {
			return rollback(fmt.Errorf("create forked window %q: %w", forkWindowName(w, i+1), err))
		}
		if err := stampForkedWindow(runner, paneID, "", w); err != nil {
			return rollback(fmt.Errorf("stamp forked window %q: %w", forkWindowName(w, i+1), err))
		}
	}

	destWindows, err = runner.ListWindows(rec.TmuxName)
	if err != nil {
		return rollback(fmt.Errorf("list forked windows: %w", err))
	}
	if activePos >= 0 && activePos < len(destWindows) {
		if err := runner.SelectWindow(rec.TmuxName, destWindows[activePos].Index); err != nil {
			return rollback(fmt.Errorf("select active forked window: %w", err))
		}
	}

	return rec, nil
}

func forkWindowName(w tmux.Window, fallbackIndex int) string {
	if strings.TrimSpace(w.Label) != "" {
		return strings.TrimSpace(w.Label)
	}
	if strings.TrimSpace(w.Name) != "" {
		return strings.TrimSpace(w.Name)
	}
	return fmt.Sprintf("tab-%d", fallbackIndex)
}

func stampForkedWindow(runner tmux.Runner, paneID, windowTarget string, source tmux.Window) error {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return fmt.Errorf("pane id unavailable")
	}
	label := strings.TrimSpace(source.Label)
	labelSource := ""
	if label != "" {
		labelSource = tablabel.SourceManual
	}
	_, err := tabs.Stamp(runner, paneID, windowTarget, label, labelSource)
	return err
}
