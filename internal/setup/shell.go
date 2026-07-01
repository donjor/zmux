package setup

import (
	"path/filepath"
	"strings"
)

// Shell identifies a supported login shell.
type Shell string

const (
	Bash    Shell = "bash"
	Zsh     Shell = "zsh"
	Fish    Shell = "fish"
	Unknown Shell = ""
)

// DetectShell maps a shell binary name (e.g. from $SHELL) to a Shell. The name
// may be a full path; only the base name matters.
func DetectShell(shellPath string) Shell {
	switch filepath.Base(shellPath) {
	case "bash":
		return Bash
	case "zsh":
		return Zsh
	case "fish":
		return Fish
	default:
		return Unknown
	}
}

// RCFile returns the rc file path for a shell under the given home directory.
// Returns ("", false) for unknown shells.
func RCFile(shell Shell, home string) (string, bool) {
	switch shell {
	case Bash:
		return filepath.Join(home, ".bashrc"), true
	case Zsh:
		return filepath.Join(home, ".zshrc"), true
	case Fish:
		return filepath.Join(home, ".config", "fish", "config.fish"), true
	default:
		return "", false
	}
}

// DefaultBashProfile is the login-shell bridge file used when no existing
// bash profile file was selected by the CLI.
func DefaultBashProfile(home string) string {
	return filepath.Join(home, ".bash_profile")
}

// autoStartSnippet returns the shell-specific managed block (without markers)
// that launches the active profile's picker outside tmux and records command
// lifecycle events inside zmux-managed panes. bin is the profile binary name
// (e.g. "zmux" or "zzmux"); an empty bin defaults to "zmux" so the edge profile
// writes "zzmux" rather than silently auto-launching the live binary. Ported
// from install.sh and extended for natural shell glyphs.
func autoStartSnippet(shell Shell, bin string) string {
	if bin == "" {
		bin = "zmux"
	}
	if shell == Fish {
		return fishAutoStartSnippet(bin) + "\n\n" + fishLifecycleSnippet(bin)
	}
	return posixAutoStartSnippet(bin) + "\n\n" + lifecycleSnippet(shell, bin)
}

func posixAutoStartSnippet(bin string) string {
	return "if command -v tmux &>/dev/null && [ -z \"$TMUX\" ]; then\n" +
		"    " + bin + "\n" +
		"fi"
}

func fishAutoStartSnippet(bin string) string {
	return "if command -v tmux >/dev/null 2>&1; and not set -q TMUX\n" +
		"    " + bin + "\n" +
		"end"
}

func lifecycleSnippet(shell Shell, bin string) string {
	switch shell {
	case Bash:
		return bashLifecycleSnippet(bin)
	case Zsh:
		return zshLifecycleSnippet(bin)
	default:
		return ""
	}
}

func bashLifecycleSnippet(bin string) string {
	q := shellQuote(bin)
	return "# zmux command lifecycle glyphs (root interactive shell only)\n" +
		"__zmux_setup_lifecycle() {\n" +
		"    local __zmux_pane __zmux_socket __zmux_profile\n" +
		"    __zmux_pane=\"${TMUX_PANE:-}\"\n" +
		"    if [ -z \"$__zmux_pane\" ] && [ -n \"${TMUX:-}\" ] && command -v tmux >/dev/null 2>&1; then\n" +
		"        __zmux_pane=\"$(tmux display-message -p '#{pane_id}' 2>/dev/null || true)\"\n" +
		"    fi\n" +
		"    if [ -n \"${TMUX:-}\" ] && [ -n \"$__zmux_pane\" ] && { [ \"${ZMUX_SHELL_ROOT:-}\" != \"$__zmux_pane\" ] || [ \"${ZMUX_SHELL_ROOT_PID:-}\" = \"$$\" ]; }; then\n" +
		"        export TMUX_PANE=\"$__zmux_pane\"\n" +
		"        export ZMUX_SHELL_ROOT=\"$__zmux_pane\"\n" +
		"        export ZMUX_SHELL_ROOT_PID=\"$$\"\n" +
		"        if [ -z \"${ZMUX_BIN:-}\" ]; then\n" +
		"            __zmux_socket=\"${TMUX%%,*}\"\n" +
		"            __zmux_profile=\"$(basename \"$__zmux_socket\" 2>/dev/null || true)\"\n" +
		"            case \"$__zmux_profile\" in \"\"|default) export ZMUX_BIN=" + q + " ;; *) export ZMUX_BIN=\"$__zmux_profile\" ;; esac\n" +
		"        fi\n" +
		"        __zmux_command_running=0\n" +
		"        __zmux_at_prompt=0\n" +
		"        __zmux_install_precmd() {\n" +
		"            if declare -p PROMPT_COMMAND 2>/dev/null | grep -q '^declare \\-[^ ]*a'; then\n" +
		"                local __zmux_pc __zmux_new=()\n" +
		"                for __zmux_pc in \"${PROMPT_COMMAND[@]}\"; do\n" +
		"                    [ \"$__zmux_pc\" = \"__zmux_precmd\" ] && continue\n" +
		"                    [ \"$__zmux_pc\" = \"__zmux_prompt_ready\" ] && continue\n" +
		"                    __zmux_new+=(\"$__zmux_pc\")\n" +
		"                done\n" +
		"                PROMPT_COMMAND=(__zmux_precmd \"${__zmux_new[@]}\" __zmux_prompt_ready)\n" +
		"            elif [ -n \"${PROMPT_COMMAND:-}\" ]; then\n" +
		"                local __zmux_pc_string=\"$PROMPT_COMMAND\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//; __zmux_precmd/}\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//__zmux_precmd; /}\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//__zmux_precmd/}\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//; __zmux_prompt_ready/}\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//__zmux_prompt_ready; /}\"\n" +
		"                __zmux_pc_string=\"${__zmux_pc_string//__zmux_prompt_ready/}\"\n" +
		"                if [ -n \"$__zmux_pc_string\" ]; then\n" +
		"                    PROMPT_COMMAND=\"__zmux_precmd; $__zmux_pc_string; __zmux_prompt_ready\"\n" +
		"                else\n" +
		"                    PROMPT_COMMAND=\"__zmux_precmd; __zmux_prompt_ready\"\n" +
		"                fi\n" +
		"            else\n" +
		"                PROMPT_COMMAND=\"__zmux_precmd; __zmux_prompt_ready\"\n" +
		"            fi\n" +
		"        }\n" +
		"        __zmux_preexec() {\n" +
		"            [ -n \"${ZMUX_SHELL_EVENT_ACTIVE:-}\" ] && return\n" +
		"            [ \"${__zmux_at_prompt:-0}\" = \"1\" ] || return\n" +
		"            [ \"${__zmux_command_running:-0}\" = \"1\" ] && return\n" +
		"            __zmux_at_prompt=0\n" +
		"            __zmux_command_running=1\n" +
		"            ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event start --command \"$BASH_COMMAND\" >/dev/null 2>&1 || true\n" +
		"        }\n" +
		"        __zmux_precmd() {\n" +
		"            local __zmux_ec=$?\n" +
		"            __zmux_at_prompt=0\n" +
		"            if [ \"${__zmux_command_running:-0}\" = \"1\" ]; then\n" +
		"                __zmux_command_running=0\n" +
		"                ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event end --exit \"$__zmux_ec\" >/dev/null 2>&1 || true\n" +
		"            fi\n" +
		"            return $__zmux_ec\n" +
		"        }\n" +
		"        __zmux_prompt_ready() {\n" +
		"            local __zmux_ec=$?\n" +
		"            __zmux_at_prompt=1\n" +
		"            return $__zmux_ec\n" +
		"        }\n" +
		"        __zmux_install_precmd\n" +
		"        trap 'case \"$BASH_COMMAND\" in __zmux_precmd*|__zmux_prompt_ready*|__zmux_preexec*|__zmux_install_precmd*|__zmux_setup_lifecycle*) ;; *) __zmux_preexec ;; esac' DEBUG\n" +
		"    fi\n" +
		"}\n" +
		"__zmux_setup_lifecycle"
}

func zshLifecycleSnippet(bin string) string {
	q := shellQuote(bin)
	return "# zmux command lifecycle glyphs (root interactive shell only)\n" +
		"__zmux_pane=\"${TMUX_PANE:-}\"\n" +
		"if [[ -z \"$__zmux_pane\" && -n \"${TMUX:-}\" ]] && command -v tmux >/dev/null 2>&1; then\n" +
		"    __zmux_pane=\"$(tmux display-message -p '#{pane_id}' 2>/dev/null || true)\"\n" +
		"fi\n" +
		"if [[ -n \"${TMUX:-}\" && -n \"$__zmux_pane\" && \"${ZMUX_SHELL_ROOT:-}\" != \"$__zmux_pane\" ]]; then\n" +
		"    export TMUX_PANE=\"$__zmux_pane\"\n" +
		"    export ZMUX_SHELL_ROOT=\"$__zmux_pane\"\n" +
		"    if [[ -z \"${ZMUX_BIN:-}\" ]]; then\n" +
		"        __zmux_socket=\"${TMUX%%,*}\"\n" +
		"        __zmux_profile=\"$(basename \"$__zmux_socket\" 2>/dev/null || true)\"\n" +
		"        case \"$__zmux_profile\" in \"\"|default) export ZMUX_BIN=" + q + " ;; *) export ZMUX_BIN=\"$__zmux_profile\" ;; esac\n" +
		"    fi\n" +
		"    typeset -g __zmux_command_running=0\n" +
		"    __zmux_preexec() {\n" +
		"        [[ -n \"${ZMUX_SHELL_EVENT_ACTIVE:-}\" ]] && return\n" +
		"        __zmux_command_running=1\n" +
		"        ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event start --command \"$1\" >/dev/null 2>&1 || true\n" +
		"    }\n" +
		"    __zmux_precmd() {\n" +
		"        local __zmux_ec=$?\n" +
		"        if [[ \"${__zmux_command_running:-0}\" == \"1\" ]]; then\n" +
		"            __zmux_command_running=0\n" +
		"            ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event end --exit \"$__zmux_ec\" >/dev/null 2>&1 || true\n" +
		"        fi\n" +
		"        return $__zmux_ec\n" +
		"    }\n" +
		"    autoload -Uz add-zsh-hook\n" +
		"    add-zsh-hook -d preexec __zmux_preexec 2>/dev/null || true\n" +
		"    add-zsh-hook -d precmd __zmux_precmd 2>/dev/null || true\n" +
		"    typeset -ga preexec_functions precmd_functions\n" +
		"    preexec_functions=(__zmux_preexec ${preexec_functions:#__zmux_preexec})\n" +
		"    precmd_functions=(__zmux_precmd ${precmd_functions:#__zmux_precmd})\n" +
		"fi"
}

func fishLifecycleSnippet(bin string) string {
	q := shellQuote(bin)
	return "# zmux command lifecycle glyphs (root interactive shell only)\n" +
		"set -l __zmux_pane \"$TMUX_PANE\"\n" +
		"if test -z \"$__zmux_pane\"; and set -q TMUX; and command -q tmux\n" +
		"    set __zmux_pane (tmux display-message -p '#{pane_id}' 2>/dev/null)\n" +
		"end\n" +
		"if set -q TMUX; and test -n \"$__zmux_pane\"; and test \"$ZMUX_SHELL_ROOT\" != \"$__zmux_pane\"\n" +
		"    set -gx TMUX_PANE \"$__zmux_pane\"\n" +
		"    set -gx ZMUX_SHELL_ROOT \"$__zmux_pane\"\n" +
		"    if not set -q ZMUX_BIN\n" +
		"        set -l __zmux_socket (string split -m1 , \"$TMUX\")[1]\n" +
		"        set -l __zmux_profile (basename \"$__zmux_socket\" 2>/dev/null)\n" +
		"        switch \"$__zmux_profile\"\n" +
		"            case '' default\n" +
		"                set -gx ZMUX_BIN " + q + "\n" +
		"            case '*'\n" +
		"                set -gx ZMUX_BIN \"$__zmux_profile\"\n" +
		"        end\n" +
		"    end\n" +
		"    set -g __zmux_command_running 0\n" +
		"    function __zmux_preexec --on-event fish_preexec\n" +
		"        test -n \"$ZMUX_SHELL_EVENT_ACTIVE\"; and return\n" +
		"        set -g __zmux_command_running 1\n" +
		"        env ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event start --command \"$argv[1]\" >/dev/null 2>&1; or true\n" +
		"    end\n" +
		"    function __zmux_postexec --on-event fish_postexec\n" +
		"        set -l __zmux_ec $status\n" +
		"        if test \"$__zmux_command_running\" = \"1\"\n" +
		"            set -g __zmux_command_running 0\n" +
		"            env ZMUX_SHELL_EVENT_ACTIVE=1 \"$ZMUX_BIN\" shell-event end --exit \"$__zmux_ec\" >/dev/null 2>&1; or true\n" +
		"        end\n" +
		"    end\n" +
		"end"
}

func bashProfileBridgeSnippet() string {
	return "# zmux bash login bridge: tmux starts bash login shells for new tabs on some systems\n" +
		"case $- in\n" +
		"  *i*)\n" +
		"    if command -v __zmux_setup_lifecycle >/dev/null 2>&1; then\n" +
		"      __zmux_setup_lifecycle\n" +
		"    elif [ -z \"${ZMUX_BASHRC_BRIDGED:-}\" ] && [ -f \"$HOME/.bashrc\" ]; then\n" +
		"      export ZMUX_BASHRC_BRIDGED=1\n" +
		"      . \"$HOME/.bashrc\"\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ShellInput is the pure input for planning shell integration.
type ShellInput struct {
	Shell Shell
	Home  string
	// Bin is the profile binary name to auto-launch (e.g. "zmux" | "zzmux").
	// Empty defaults to "zmux".
	Bin string
	// BashProfile is the login-shell bridge file for bash. Empty defaults to
	// ~/.bash_profile. Ignored for non-bash shells.
	BashProfile string
	// Remove plans removal of the integration instead of adding it.
	Remove bool
}

// PlanShellIntegration builds the plan for shell integration. It is pure: no
// disk access. Returns a Plan with managed edits, or an empty Plan and false if
// the shell is unsupported.
func PlanShellIntegration(in ShellInput) (Plan, bool) {
	rc, ok := RCFile(in.Shell, in.Home)
	if !ok {
		return Plan{}, false
	}
	action := ActionAdd
	if in.Remove {
		action = ActionRemove
	}
	edits := []Edit{{
		File:   rc,
		Label:  "shell integration (" + filepath.Base(rc) + ")",
		Block:  autoStartSnippet(in.Shell, in.Bin),
		Action: action,
	}}
	if in.Shell == Bash {
		profile := in.BashProfile
		if profile == "" {
			profile = DefaultBashProfile(in.Home)
		}
		edits = append(edits, Edit{
			File:   profile,
			Label:  "bash login bridge (" + filepath.Base(profile) + ")",
			Block:  bashProfileBridgeSnippet(),
			Action: action,
		})
	}
	return Plan{Edits: edits}, true
}
