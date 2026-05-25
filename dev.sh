#!/usr/bin/env bash
# dev.sh — build + install a zmux binary for local testing.
#
# Usage: ./dev.sh [zmux|zzmux]   (default: zmux)
#   zmux   build + install the live binary, link Claude Code + Pi integration
#   zzmux  build + install an identical edge binary (binary only), so you can
#          test changes without overwriting the zmux you're currently running

set -euo pipefail

ZMUX_ROOT="$(cd "$(dirname "$0")" && pwd)"
TARGET="${1:-zmux}"

case "$TARGET" in
	zmux | zzmux) ;;
	*)
		printf "usage: %s [zmux|zzmux]\n" "$0" >&2
		exit 1
		;;
esac

bold='\033[1m'
dim='\033[38;2;90;99;120m'
green='\033[38;2;127;217;98m'
reset='\033[0m'

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# Build
printf "${dim}building ${bold}%s${reset}${dim}...${reset} " "$TARGET"
go build -ldflags "-X main.version=${VERSION}" -o "$TARGET" ./cmd/zmux/ 2>&1
printf "${green}ok${reset}\n"

# Install binary (handle "text file busy" by removing first)
printf "${dim}installing...${reset} "
rm -f "$HOME/.local/bin/$TARGET" 2>/dev/null || true
cp "$TARGET" "$HOME/.local/bin/$TARGET"
printf "${green}ok${reset}  ${dim}~/.local/bin/%s${reset}\n" "$TARGET"

# Claude + Pi integration links only for the live binary. The skill is
# brand-agnostic (repo is source of truth); edge testing doesn't relink.
if [ "$TARGET" = "zmux" ]; then
	printf "${dim}linking claude integration...${reset} "
	mkdir -p "$HOME/.claude/skills"
	rm -rf "$HOME/.claude/skills/zmux"
	ln -s "$ZMUX_ROOT/skills/zmux" "$HOME/.claude/skills/zmux"
	printf "${green}ok${reset}  ${dim}~/.claude/skills/zmux${reset}\n"

	printf "${dim}linking pi integration...${reset} "
	mkdir -p "$HOME/.pi/agent/skills" "$HOME/.pi/agent/extensions"
	rm -rf "$HOME/.pi/agent/skills/zmux" "$HOME/.pi/agent/extensions/pi-zmux"
	ln -s "$ZMUX_ROOT/skills/zmux" "$HOME/.pi/agent/skills/zmux"
	ln -s "$ZMUX_ROOT/pi-extension" "$HOME/.pi/agent/extensions/pi-zmux"
	printf "${green}ok${reset}  ${dim}~/.pi/agent/{skills/zmux,extensions/pi-zmux}${reset}\n"
fi
