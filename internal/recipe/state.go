package recipe

import (
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func BuildState(runner tmux.Runner, store *workspace.Store) (State, error) {
	state := State{
		Sessions:   map[string]SessionState{},
		Workspaces: map[string]WorkspaceState{},
	}

	sessions, err := runner.ListSessions()
	if err != nil {
		return state, err
	}
	for _, sess := range sessions {
		ss := SessionState{Name: sess.Name, Windows: map[string]WindowState{}}
		windows, err := runner.ListWindows(sess.Name)
		if err == nil {
			for _, win := range windows {
				ss.Windows[win.Name] = WindowState{Name: win.Name}
				if win.Label != "" {
					ss.Windows[win.Label] = WindowState{Name: win.Label}
				}
			}
		}
		state.Sessions[sess.Name] = ss
	}

	if store != nil {
		workspaces, err := store.ListWorkspaces()
		if err == nil {
			for _, ws := range workspaces {
				wss := WorkspaceState{Name: ws.Name, Sessions: map[string]bool{}}
				for _, sess := range ws.Sessions {
					wss.Sessions[sess.Label] = true
					wss.Sessions[sess.TmuxName] = true
				}
				state.Workspaces[ws.Name] = wss
			}
		}
	}

	return state, nil
}
