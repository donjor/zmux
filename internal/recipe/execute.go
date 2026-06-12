package recipe

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

const (
	sessionRecipeOption    = "@zmux_recipe"
	sessionWorkspaceOption = "@zmux_recipe_workspace"
	windowRecipeOption     = "@zmux_recipe"
	windowTabOption        = "@zmux_recipe_tab"
)

func Execute(runner tmux.Runner, store *workspace.Store, p Plan) error {
	if store != nil {
		if _, err := store.EnsureWorkspace(p.Workspace, p.CWD); err != nil {
			return err
		}
	}
	sessionTargets := make(map[string]string, len(p.Sessions))
	for _, sess := range p.Sessions {
		target := sess.Name
		var rec workspace.WorkspaceSession
		if store != nil && p.Workspace != "" {
			if existing, ok := store.SessionRecord(p.Workspace, sess.Name); ok {
				rec = existing
			} else {
				var err error
				rec, err = workspace.NewSessionRecord(p.Workspace, sess.Name)
				if err != nil {
					return err
				}
			}
			target = rec.TmuxName
		}
		sessionTargets[sess.Name] = target
		if !sess.Exists {
			if err := runner.NewSession(target, sess.CWD); err != nil {
				return fmt.Errorf("create session %q: %w", sess.Name, err)
			}
			if rec.TmuxName != "" {
				if err := workspace.StampSessionMetadata(runner, p.Workspace, rec); err != nil {
					_ = runner.KillSession(target)
					return err
				}
			}
		}
		_ = runner.SetSessionOption(target, sessionRecipeOption, p.RecipeName)
		_ = runner.SetSessionOption(target, sessionWorkspaceOption, p.Workspace)
		if store != nil {
			if rec.TmuxName == "" {
				if err := store.AddSession(p.Workspace, sess.Name); err != nil {
					return err
				}
			} else if err := store.AddSessionRecord(p.Workspace, rec); err != nil {
				return err
			}
		}
		if err := reconcileTabs(runner, p, sess, target); err != nil {
			return err
		}
	}
	if p.FocusSession != "" && !p.Detach {
		focusTarget := sessionTargets[p.FocusSession]
		if focusTarget == "" {
			focusTarget = p.FocusSession
		}
		if p.FocusTab != "" {
			if err := selectFocusTab(runner, p, focusTarget); err != nil {
				return err
			}
		}
		if store != nil {
			_ = store.SetLastActive(p.Workspace, p.FocusSession)
		}
		return session.Attach(runner, focusTarget)
	}
	return nil
}

func selectFocusTab(runner tmux.Runner, p Plan, focusTarget string) error {
	windows, err := runner.ListWindows(focusTarget)
	if err == nil {
		for _, win := range windows {
			if win.Name == p.FocusTab || win.Label == p.FocusTab {
				return runner.SelectWindow(focusTarget, win.Index)
			}
		}
	}
	for _, sess := range p.Sessions {
		if sess.Name != p.FocusSession || sess.Exists {
			continue
		}
		for i, tab := range sess.Tabs {
			if tab.Name == p.FocusTab {
				return runner.SelectWindow(focusTarget, i+1)
			}
		}
	}
	return nil
}

func reconcileTabs(runner tmux.Runner, p Plan, sess PlannedSession, sessionTarget string) error {
	for i, tab := range sess.Tabs {
		if tab.Exists {
			continue
		}
		target := sessionTarget + ":" + tab.Name
		if !sess.Exists && i == 0 {
			if err := runner.RenameWindow(sessionTarget, "1", tab.Name); err != nil {
				return fmt.Errorf("rename first tab %q: %w", tab.Name, err)
			}
		} else {
			paneID, err := runner.NewWindow(sessionTarget, tab.Name, tab.CWD, tmux.Detached())
			if err != nil {
				return fmt.Errorf("create tab %q: %w", tab.Name, err)
			}
			if paneID != "" {
				target = paneID
			}
		}
		_ = runner.SetWindowOption(sessionTarget+":"+tab.Name, windowRecipeOption, p.RecipeName)
		_ = runner.SetWindowOption(sessionTarget+":"+tab.Name, windowTabOption, tab.Name)
		if tab.Command != "" && p.TabMode != TabModeEmpty {
			keys := []string{tab.Command, "Enter"}
			if p.TabMode == TabModeReady {
				keys = []string{tab.Command}
			}
			if err := runner.SendKeys(target, keys...); err != nil {
				return fmt.Errorf("send command to %s: %w", target, err)
			}
		}
	}
	return nil
}
