#!/usr/bin/env bash
# dev.sh — build, install zmux, and link Pi integration for local testing
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

# Link Pi skill and extension (repo is source of truth)
printf "${dim}linking pi integration...${reset} "
mkdir -p "$HOME/.pi/agent/skills" "$HOME/.pi/agent/extensions"
rm -rf "$HOME/.pi/agent/skills/zmux" "$HOME/.pi/agent/extensions/pi-zmux"
ln -s "$ZMUX_ROOT/skills/zmux" "$HOME/.pi/agent/skills/zmux"
ln -s "$ZMUX_ROOT/pi-extension" "$HOME/.pi/agent/extensions/pi-zmux"
printf "${green}ok${reset}  ${dim}~/.pi/agent/{skills/zmux,extensions/pi-zmux}${reset}\n"
