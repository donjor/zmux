#!/usr/bin/env bash
# dev.sh — build, install zmux, and sync skill for local testing
# Usage: ./dev.sh

set -euo pipefail

ZMUX_ROOT="$(cd "$(dirname "$0")" && pwd)"

bold='\033[1m'
dim='\033[38;2;90;99;120m'
green='\033[38;2;127;217;98m'
reset='\033[0m'

# Build
printf "${dim}building...${reset} "
go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o zmux ./cmd/zmux/ 2>&1
printf "${green}ok${reset}\n"

# Install binary (handle "text file busy" by removing first)
printf "${dim}installing...${reset} "
rm -f ~/.local/bin/zmux 2>/dev/null || true
cp zmux ~/.local/bin/zmux
printf "${green}ok${reset}  ${dim}~/.local/bin/zmux${reset}\n"

# Sync Claude skill (repo → ~/.claude/skills/zmux/)
SKILL_SRC="${ZMUX_ROOT}/skills/zmux/SKILL.md"
SKILL_DST="$HOME/.claude/skills/zmux/SKILL.md"
if [ -f "$SKILL_SRC" ]; then
    if ! cmp -s "$SKILL_SRC" "$SKILL_DST" 2>/dev/null; then
        mkdir -p "$HOME/.claude/skills/zmux"
        cp "$SKILL_SRC" "$SKILL_DST"
        printf "${dim}skill synced${reset}  ${dim}~/.claude/skills/zmux/${reset}\n"
    fi
fi
