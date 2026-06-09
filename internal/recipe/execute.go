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
	for _, sess := range p.Sessions {
		if !sess.Exists {
			if err := runner.NewSession(sess.Name, sess.CWD); err != nil {
				return fmt.Errorf("create session %q: %w", sess.Name, err)
			}
		}
		_ = runner.SetSessionOption(sess.Name, sessionRecipeOption, p.RecipeName)
		_ = runner.SetSessionOption(sess.Name, sessionWorkspaceOption, p.Workspace)
		if store != nil {
			if err := store.AddSession(p.Workspace, sess.Name); err != nil {
				return err
			}
		}
		if err := reconcileTabs(runner, p, sess); err != nil {
			return err
		}
	}
	if p.FocusSession != "" && !p.Detach {
		if p.FocusTab != "" {
			if err := selectFocusTab(runner, p); err != nil {
				return err
			}
		}
		if store != nil {
			_ = store.SetLastActive(p.Workspace, p.FocusSession)
		}
		return session.Attach(runner, p.FocusSession)
	}
	return nil
}

func selectFocusTab(runner tmux.Runner, p Plan) error {
	windows, err := runner.ListWindows(p.FocusSession)
	if err == nil {
		for _, win := range windows {
			if win.Name == p.FocusTab || win.Label == p.FocusTab {
				return runner.SelectWindow(p.FocusSession, win.Index)
			}
		}
	}
	for _, sess := range p.Sessions {
		if sess.Name != p.FocusSession || sess.Exists {
			continue
		}
		for i, tab := range sess.Tabs {
			if tab.Name == p.FocusTab {
				return runner.SelectWindow(p.FocusSession, i+1)
			}
		}
	}
	return nil
}

func reconcileTabs(runner tmux.Runner, p Plan, sess PlannedSession) error {
	for i, tab := range sess.Tabs {
		if tab.Exists {
			continue
		}
		target := sess.Name + ":" + tab.Name
		if !sess.Exists && i == 0 {
			if err := runner.RenameWindow(sess.Name, "1", tab.Name); err != nil {
				return fmt.Errorf("rename first tab %q: %w", tab.Name, err)
			}
		} else {
			paneID, err := runner.NewWindow(sess.Name, tab.Name, tab.CWD, tmux.Detached())
			if err != nil {
				return fmt.Errorf("create tab %q: %w", tab.Name, err)
			}
			if paneID != "" {
				target = paneID
			}
		}
		_ = runner.SetWindowOption(sess.Name+":"+tab.Name, windowRecipeOption, p.RecipeName)
		_ = runner.SetWindowOption(sess.Name+":"+tab.Name, windowTabOption, tab.Name)
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
