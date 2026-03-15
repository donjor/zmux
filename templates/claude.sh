#!/usr/bin/env bash
# Claude Code session: claude, shell, git

tmux new-session -d -s "$ZMUX_SESSION" -c "$ZMUX_DIR" -n "claude"
tmux send-keys -t "$ZMUX_SESSION:claude" "claude" Enter

tmux new-window -t "$ZMUX_SESSION" -n "shell" -c "$ZMUX_DIR"

tmux new-window -t "$ZMUX_SESSION" -n "git" -c "$ZMUX_DIR"
tmux send-keys -t "$ZMUX_SESSION:git" "git status" Enter

tmux select-window -t "$ZMUX_SESSION:claude"
