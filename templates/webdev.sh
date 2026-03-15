#!/usr/bin/env bash
# Web development: editor, dev server, tests, git

tmux new-session -d -s "$ZMUX_SESSION" -c "$ZMUX_DIR" -n "editor"
tmux send-keys -t "$ZMUX_SESSION:editor" "nvim ." Enter

tmux new-window -t "$ZMUX_SESSION" -n "dev" -c "$ZMUX_DIR"

tmux new-window -t "$ZMUX_SESSION" -n "test" -c "$ZMUX_DIR"

tmux new-window -t "$ZMUX_SESSION" -n "git" -c "$ZMUX_DIR"
tmux send-keys -t "$ZMUX_SESSION:git" "git status" Enter

tmux select-window -t "$ZMUX_SESSION:editor"
