#!/usr/bin/env bash
# Full dev environment: editor, server, git

tmux new-session -d -s "$ZMUX_SESSION" -c "$ZMUX_DIR" -n "editor"
tmux send-keys -t "$ZMUX_SESSION:editor" "nvim ." Enter

tmux new-window -t "$ZMUX_SESSION" -n "server" -c "$ZMUX_DIR"

tmux new-window -t "$ZMUX_SESSION" -n "git" -c "$ZMUX_DIR"
tmux send-keys -t "$ZMUX_SESSION:git" "git status" Enter

tmux select-window -t "$ZMUX_SESSION:editor"
