#!/usr/bin/env bash
# zmux installer — build from source, install binary, add shell integration
#
# Usage:
#   git clone https://github.com/donjor/zmux.git && cd zmux && ./install.sh
#
# Or from a remote source (once published):
#   curl -fsSL https://raw.githubusercontent.com/donjor/zmux/main/install.sh | bash
#
# What this script does:
#   1. Checks dependencies (go, tmux >= 3.2)
#   2. Builds the zmux binary
#   3. Installs to ~/.local/bin/zmux
#   4. Adds shell integration to your rc file (optional)
#   5. Tells you to run `zmux init`

set -euo pipefail

# ── Colors ──
bold='\033[1m'
dim='\033[38;2;90;99;120m'
gold='\033[38;2;230;180;80m'
green='\033[38;2;127;217;98m'
red='\033[38;2;255;100;100m'
reset='\033[0m'

info()    { printf "  ${dim}%s${reset}\n" "$1"; }
success() { printf "  ${green}✓${reset} %s\n" "$1"; }
warn()    { printf "  ${gold}!${reset} %s\n" "$1"; }
fail()    { printf "  ${red}✗${reset} %s\n" "$1"; }

BIN_DIR="${HOME}/.local/bin"
ZMUX_BIN="${BIN_DIR}/zmux"

# ── Header ──
printf "\n"
printf "  ${gold}${bold}zmux installer${reset}\n"
printf "  ${dim}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}\n\n"

# ── Step 1: Check dependencies ──
printf "  ${bold}Checking dependencies...${reset}\n"

# Go
if ! command -v go &>/dev/null; then
    fail "go not found"
    info "Install Go: https://go.dev/dl/"
    exit 1
fi
go_version=$(go version | awk '{print $3}' | sed 's/go//')
success "go ${go_version}"

# tmux
if ! command -v tmux &>/dev/null; then
    fail "tmux not found"
    info "Install tmux: apt install tmux / brew install tmux"
    exit 1
fi
tmux_version=$(tmux -V | awk '{print $2}')
tmux_major=$(echo "$tmux_version" | cut -d. -f1)
tmux_minor=$(echo "$tmux_version" | cut -d. -f2 | sed 's/[^0-9].*//')
if [ "$tmux_major" -lt 3 ] || { [ "$tmux_major" -eq 3 ] && [ "$tmux_minor" -lt 2 ]; }; then
    fail "tmux ${tmux_version} found, but zmux requires >= 3.2"
    info "Upgrade tmux: apt install tmux / brew install tmux"
    exit 1
fi
success "tmux ${tmux_version}"

printf "\n"

# ── Step 2: Build ──
printf "  ${bold}Building zmux...${reset}\n"

# Determine source directory — if we're in the repo, use it; otherwise clone
ZMUX_SRC=""
if [ -f "go.mod" ] && grep -q "donjor/zmux" go.mod 2>/dev/null; then
    ZMUX_SRC="$(pwd)"
elif [ -f "$(dirname "$0")/go.mod" ] && grep -q "donjor/zmux" "$(dirname "$0")/go.mod" 2>/dev/null; then
    ZMUX_SRC="$(cd "$(dirname "$0")" && pwd)"
else
    fail "Not in the zmux repo. Run this from the zmux source directory."
    info "git clone https://github.com/donjor/zmux.git && cd zmux && ./install.sh"
    exit 1
fi

cd "$ZMUX_SRC"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")

if ! go build -ldflags "-X main.version=${VERSION}" -o zmux ./cmd/zmux/ 2>&1; then
    fail "Build failed"
    exit 1
fi
success "Built zmux ${VERSION}"

printf "\n"

# ── Step 3: Install binary ──
printf "  ${bold}Installing...${reset}\n"

mkdir -p "$BIN_DIR"
cp zmux "$ZMUX_BIN"
chmod +x "$ZMUX_BIN"
success "Installed to ${ZMUX_BIN}"

# Check PATH
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$BIN_DIR"; then
    warn "${BIN_DIR} is not in your PATH"
    info "Add to your shell rc: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

printf "\n"

# ── Step 4: Shell integration (optional) ──
printf "  ${bold}Shell integration${reset}\n"

SHELL_NAME=$(basename "${SHELL:-/bin/bash}")
RC_FILE=""

case "$SHELL_NAME" in
    bash) RC_FILE="$HOME/.bashrc" ;;
    zsh)  RC_FILE="$HOME/.zshrc" ;;
    fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
    *)    RC_FILE="" ;;
esac

ZMUX_MARKER="# zmux — auto-start tmux session picker"

add_shell_integration() {
    local rc="$1"

    # Don't add if already present
    if grep -qF "$ZMUX_MARKER" "$rc" 2>/dev/null; then
        success "Shell integration already in $(basename "$rc")"
        return
    fi

    local snippet=""
    if [ "$SHELL_NAME" = "fish" ]; then
        snippet="
${ZMUX_MARKER}
if command -v tmux >/dev/null 2>&1; and not set -q TMUX
    zmux
end"
    else
        snippet="
${ZMUX_MARKER}
if command -v tmux &>/dev/null && [ -z \"\$TMUX\" ]; then
    zmux
fi"
    fi

    printf "\n" >> "$rc"
    printf "%s\n" "$snippet" >> "$rc"
    success "Added auto-start to $(basename "$rc")"
    info "Opens the session picker when you open a terminal"
}

if [ -n "$RC_FILE" ]; then
    printf "\n"
    printf "  Add zmux auto-start to ${bold}$(basename "$RC_FILE")${reset}?\n"
    printf "  This launches the session picker when you open a terminal.\n\n"
    printf "  ${dim}[y/N]${reset} "
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        add_shell_integration "$RC_FILE"
    else
        info "Skipped — you can add it manually later:"
        if [ "$SHELL_NAME" = "fish" ]; then
            info "  echo 'if command -v tmux >/dev/null 2>&1; and not set -q TMUX; zmux; end' >> $RC_FILE"
        else
            info "  Add to $RC_FILE:"
            info "  if command -v tmux &>/dev/null && [ -z \"\$TMUX\" ]; then zmux; fi"
        fi
    fi
else
    warn "Unknown shell '${SHELL_NAME}' — add zmux to your rc file manually"
fi

printf "\n"

# ── Step 5: Install Claude Code skill ──
SKILL_SRC="${ZMUX_SRC}/skills/zmux/SKILL.md"
CLAUDE_SKILLS_DIR="$HOME/.claude/skills/zmux"

if [ -f "$SKILL_SRC" ]; then
    printf "  ${bold}Claude Code integration${reset}\n\n"
    printf "  Install the zmux skill for Claude Code?\n"
    printf "  This teaches Claude how to use zmux for terminal management:\n"
    printf "  ${dim}• Run commands in named tabs (zmux run)${reset}\n"
    printf "  ${dim}• Read terminal output (zmux watch)${reset}\n"
    printf "  ${dim}• Share terminals between you and Claude${reset}\n"
    printf "  ${dim}• Wait for commands to finish with exit codes${reset}\n\n"
    printf "  ${dim}[Y/n]${reset} "
    read -r response
    if [[ ! "$response" =~ ^[Nn]$ ]]; then
        mkdir -p "$CLAUDE_SKILLS_DIR"
        cp "$SKILL_SRC" "$CLAUDE_SKILLS_DIR/SKILL.md"
        success "Installed Claude skill to ~/.claude/skills/zmux/"
    else
        info "Skipped — copy skills/zmux/SKILL.md to ~/.claude/skills/zmux/ manually"
    fi
    printf "\n"
fi

# ── Step 6: Run zmux init ──
printf "  ${bold}Setup${reset}\n\n"
printf "  Run ${gold}zmux init${reset} now to configure themes, bar preset, and tmux.conf?\n\n"
printf "  ${dim}[Y/n]${reset} "
read -r response
if [[ ! "$response" =~ ^[Nn]$ ]]; then
    printf "\n"
    "$ZMUX_BIN" init
else
    info "Skipped — run 'zmux init' whenever you're ready"
fi

printf "\n"
printf "  ${gold}${bold}Done!${reset}\n\n"
printf "  Restart your terminal for shell integration to take effect.\n"
printf "\n"
