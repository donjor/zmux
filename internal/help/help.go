// Package help is the single source of truth for zmux's user-facing help
// content: the command reference plus the keybinding reference. Both the
// `zmux help` text printer and the prefix+? interactive viewer render from
// Sections(), so the two surfaces (and the generated docs) never drift on what
// help exists.
package help

import "github.com/donjor/zmux/internal/keys"

// Entry is one help row: a command or keybinding and what it does.
type Entry struct {
	Label string // e.g. "zmux new <ws>" or "prefix + p"
	Desc  string
}

// Scope tags whether a section is a CLI command group or a keybinding group, so
// the help viewer can filter to commands, keys, or all. ScopeCommand is the zero
// value, so command sections need not set it.
type Scope int

const (
	ScopeCommand Scope = iota
	ScopeKeybinding
)

// Section groups related entries under a title.
type Section struct {
	Title   string
	Scope   Scope
	Entries []Entry
}

// Sections returns the full help content — the command reference followed by
// the keybinding reference. Keybinding sections are derived from the keys
// registry (the same source the tmux conf and generated docs use), so a new
// binding surfaces here automatically.
func Sections() []Section {
	return append(commandSections(), keybindingSections()...)
}

// FilterScope returns only the sections in the given scope — the helper behind
// the viewer's commands / keys / all toggle.
func FilterScope(sections []Section, scope Scope) []Section {
	out := make([]Section, 0, len(sections))
	for _, s := range sections {
		if s.Scope == scope {
			out = append(out, s)
		}
	}
	return out
}

// BandLabel is the scope-band header shown above a run of same-scope sections.
func BandLabel(scope Scope) string {
	if scope == ScopeKeybinding {
		return "KEYBINDINGS"
	}
	return "COMMANDS"
}

// keybindingSections renders the keybinding reference. Prefix keys are grouped
// by their category — the same small-titled-section rhythm as the command
// reference, rather than one compressed lump — followed by the instant and
// inherited tables. Derived from the keys registry, so a new binding surfaces
// here automatically.
func keybindingSections() []Section {
	var order []keys.Category
	byCat := map[keys.Category][]keys.Binding{}
	for _, b := range keys.PrefixBindings {
		if _, ok := byCat[b.Category]; !ok {
			order = append(order, b.Category)
		}
		byCat[b.Category] = append(byCat[b.Category], b)
	}

	out := make([]Section, 0, len(order)+2)
	for _, cat := range order {
		out = append(out, keySection(string(cat), byCat[cat]))
	}
	// The "no prefix" and "from tmux" distinctions are worth their own sections.
	out = append(out, keySection("No prefix (instant)", keys.NoPrefixBindings))
	out = append(out, keySection("Inherited from tmux", keys.InheritedBindings))
	return out
}

func keySection(title string, bindings []keys.Binding) Section {
	entries := make([]Entry, 0, len(bindings))
	for _, b := range bindings {
		entries = append(entries, Entry{Label: b.DisplayKey(), Desc: b.Help})
	}
	return Section{Title: title, Scope: ScopeKeybinding, Entries: entries}
}

// commandSections is the curated CLI command reference. Keep entries terse; the
// label is copy-pasteable and the description fits one line.
func commandSections() []Section {
	return []Section{
		{Title: "Session Management", Entries: []Entry{
			{"zmux", "Workspace picker (outside tmux) / dashboard (inside)"},
			{"zmux new <ws>", "Create workspace + main session, attach"},
			{"zmux new <ws> <session...>", "Create workspace + named sessions"},
			{"zmux new", "Create tmp-N session (no workspace)"},
			{"zmux open <ws> [session]", "Open workspace session (aliases: attach, a)"},
			{"zmux open <ws> [session] --pin-view", "Create persistent grouped viewport"},
			{"zmux fork <new-session-label>", "Fork current session tabs into a new local session"},
			{"zmux kill <name>", "Smart kill — workspace-first (alias: k)"},
			{"zmux ls", "List workspaces (workspace-primary)"},
			{"zmux ls <ws>", "List sessions within a workspace"},
			{"zmux ls -s", "Flat session list"},
			{"zmux tabs [session]", "List tabs (alias: t)"},
			{"zmux where [--json]", "Current workspace/session/tab/pane (alias: whoami)"},
		}},
		{Title: "Recipes", Entries: []Entry{
			{"zmux run <recipe>", "Open the recipe form with defaults"},
			{"zmux run <recipe> -y", "Run recipe defaults without prompting"},
			{"zmux run <recipe> --dry-run", "Print the exact recipe plan"},
			{"zmux run --command <cmd>", "Force command mode on name collisions"},
			{"zmux recipe list", "List bundled and user recipes"},
			{"zmux recipe show <recipe>", "Show recipe TOML"},
			{"zmux recipe lint [recipe]", "Validate recipes"},
			{"zmux recipe fork <recipe>", "Copy a bundled recipe for editing"},
			{"zmux recipe edit <recipe>", "Edit a user recipe"},
		}},
		{Title: "Workspace Management", Entries: []Entry{
			{"zmux workspace list", "List workspaces (alias: ws)"},
			{"zmux workspace show <ws>", "Show workspace sessions"},
			{"zmux workspace kill <ws>", "Kill workspace + all sessions"},
			{"zmux session kill <session>", "Kill a session"},
			{"zmux session run <s> -n <t> -- <cmd>", "Detached session + command as first tab (workers)"},
			{"zmux tab move <tab> <dest>", "Move tab to another session"},
			{"zmux tab label [label]", "Set/clear stable tab label"},
			{"zmux tab state <state> [tab]", "Set/clear tab lifecycle glyph"},
			{"zmux tab status <tab>", "Show lifecycle/command status"},
			{"zmux tab peer <action> [tab]", "Update prompt-scoped peer lifecycle"},
			{"zmux tab pane <tab>", "Join as pane (--focus selects it)"},
			{"zmux tab full [tab]", "Promote visible/hidden pane-tab to full tab"},
			{"zmux tab hide [tab]", "Hide a joined pane under its parent tab"},
			{"zmux tab show <tab|N>", "Rejoin hidden pane (--focus selects it)"},
			{"zmux tab kill <tab>", "Kill a tab"},
			{"zmux reap [--dry-run]", "Adopt/flag/kill stale tabs by lifecycle policy"},
		}},
		{Title: "Terminal Commands", Entries: []Entry{
			{"zmux run '<cmd>' -n <tab>", "Run + wait for completion"},
			{"zmux run '<cmd>' -n <tab> -d", "Run detached (servers)"},
			{"zmux run '<cmd>' -n <tab> -f", "Run + follow output"},
			{"zmux run '<cmd>' -n <tab> --keep", "Exempt the tab from auto-reaping"},
			{"zmux run '<cmd>' -n <tab> --scope daemon", "Long-lived tab — never auto-reaped"},
			{"zmux watch <tab>", "Capture tab output"},
			{"zmux watch <tab> --until <pat>", "Wait for pattern match"},
			{"zmux watch <tab> --idle <s>", "Wait until quiet for N seconds"},
			{"zmux watch <tab> -f", "Follow output (tail -f)"},
			{"zmux send <tab> <keys>", "Send keystrokes to tab"},
			{"zmux type <tab> '<text>'", "Type text + Enter"},
			{"zmux pane open <name> -r 40", "Open pane (--no-focus for agents)"},
			{"zmux pane open --label-tab ...", "Preserve tab label before sidecar split"},
			{"zmux pane toggle <name> -r 40 -- <cmd>", "Toggle named pane"},
			{"zmux pane current [--json]", "Print current pane id/details"},
			{"zmux pane list / zmux panes", "List panes in current window"},
			{"zmux pane focus/close <pane>", "Focus or close pane by id/title/index"},
			{"zmux pane resize <pane> --size 40%", "Resize pane width"},
			{"zmux terminal current --json", "Resolve visible terminal screenshot target"},
			{"zmux terminal capabilities", "Diagnose tmux truecolor/RGB support"},
			{"zmux terminal refresh", "Reattach client to refresh RGB features"},
			{"zmux snapshot [--no-png]", "Capture pane text/ANSI + PNG evidence bundle"},
		}},
		{Title: "Theming", Entries: []Entry{
			{"zmux theme", "Interactive theme picker"},
			{"zmux theme set <name>", "Set theme directly"},
			{"zmux theme list", "List available themes"},
			{"zmux theme sync", "Sync from default target"},
			{"zmux theme pull <target>", "Pull from ghostty/nvim"},
		}},
		{Title: "Configuration", Entries: []Entry{
			{"zmux bar", "Bar presets with previews"},
			{"zmux bar <preset>", "Set preset"},
			{"zmux init", "Setup wizard"},
			{"zmux apply", "Regenerate + apply config"},
			{"zmux refresh", "Apply config + refresh current client"},
			{"zmux status", "Show config summary"},
		}},
		{Title: "Other", Entries: []Entry{
			{"zmux doctor", "Check shell integration freshness"},
			{"zmux version", "Version info"},
			{"zmux completion <shell>", "Completions (bash/zsh/fish)"},
			{"zmux help", "This help"},
		}},
	}
}
