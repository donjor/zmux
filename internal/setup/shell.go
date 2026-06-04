package setup

import "path/filepath"

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

// autoStartSnippet returns the shell-specific auto-start block (without markers)
// that launches the active profile's picker in a login shell when not already
// inside tmux. bin is the profile binary name (e.g. "zmux" or "zzmux"); an empty
// bin defaults to "zmux" so the edge profile writes "zzmux" rather than silently
// auto-launching the live binary. Ported from install.sh.
func autoStartSnippet(shell Shell, bin string) string {
	if bin == "" {
		bin = "zmux"
	}
	if shell == Fish {
		return "if command -v tmux >/dev/null 2>&1; and not set -q TMUX\n" +
			"    " + bin + "\n" +
			"end"
	}
	return "if command -v tmux &>/dev/null && [ -z \"$TMUX\" ]; then\n" +
		"    " + bin + "\n" +
		"fi"
}

// ShellInput is the pure input for planning shell integration.
type ShellInput struct {
	Shell Shell
	Home  string
	// Bin is the profile binary name to auto-launch (e.g. "zmux" | "zzmux").
	// Empty defaults to "zmux".
	Bin string
	// Remove plans removal of the integration instead of adding it.
	Remove bool
}

// PlanShellIntegration builds the plan for shell auto-start integration. It is
// pure: no disk access. Returns a Plan with a single Edit, or an empty Plan and
// false if the shell is unsupported.
func PlanShellIntegration(in ShellInput) (Plan, bool) {
	rc, ok := RCFile(in.Shell, in.Home)
	if !ok {
		return Plan{}, false
	}
	action := ActionAdd
	if in.Remove {
		action = ActionRemove
	}
	return Plan{Edits: []Edit{{
		File:   rc,
		Label:  "shell auto-start (" + filepath.Base(rc) + ")",
		Block:  autoStartSnippet(in.Shell, in.Bin),
		Action: action,
	}}}, true
}
