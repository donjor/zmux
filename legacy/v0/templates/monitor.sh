#!/usr/bin/env bash
# System monitoring: htop, gpu, logs

tmux new-session -d -s "$ZMUX_SESSION" -c "$ZMUX_DIR" -n "htop"
tmux send-keys -t "$ZMUX_SESSION:htop" "htop" Enter

tmux new-window -t "$ZMUX_SESSION" -n "gpu" -c "$ZMUX_DIR"
tmux send-keys -t "$ZMUX_SESSION:gpu" "watch -n1 nvidia-smi" Enter

tmux new-window -t "$ZMUX_SESSION" -n "logs" -c "$ZMUX_DIR"
tmux send-keys -t "$ZMUX_SESSION:logs" "journalctl -f" Enter

tmux select-window -t "$ZMUX_SESSION:htop"
