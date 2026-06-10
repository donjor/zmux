// Package guard classifies a shell command for agent terminal-hygiene
// enforcement: does it reach past zmux (raw tmux), start something that keeps
// running outside a visible tab (dev server, background job), or need shared
// interaction (sudo/ssh/REPL)? The ruleset is exercised against the shared
// corpus at testdata/zmux-guard-corpus.jsonl, which is also consumed by the
// Claude PreToolUse hook and the pi-extension classifier so the three can't
// silently drift. Pure leaf: no side effects, no deps.
package guard

import (
	"regexp"
	"strings"
)

// Kind is the classification of a command.
type Kind string

const (
	Safe        Kind = "safe"
	DirectTmux  Kind = "direct_tmux"
	Runtime     Kind = "runtime"
	Background  Kind = "background"
	Interactive Kind = "interactive"
)

// Decision is the enforce-mode outcome for a Kind.
type Decision string

const (
	Allow Decision = "allow" // pass through
	Warn  Decision = "warn"  // visible nudge, non-blocking
	Block Decision = "block" // refuse; redirect to the zmux equivalent
)

// Result is the classifier verdict. Target is a semantic suggestion key
// (watch, send, runtime, …) that each agent renders to its own surface.
type Result struct {
	Kind     Kind     `json:"kind"`
	Decision Decision `json:"decision"`
	Target   string   `json:"target,omitempty"`
	Reason   string   `json:"reason,omitempty"`
}

// Options carry environmental facts the classifier can't read from the command.
type Options struct {
	// RepoCwd is true when the command runs inside the zmux repo, where raw
	// tmux is a legitimate development tool and is therefore exempt.
	RepoCwd bool
}

var (
	bypassEnv     = regexp.MustCompile(`(^|\s)ZMUX_ALLOW=1(\s|$)`)
	bypassComment = regexp.MustCompile(`(?i)#\s*zmux:\s*allow\b`)

	bgWord = regexp.MustCompile(`(^|\s)(nohup|disown)\b`)
	// bgAmp matches a lone `&` control operator (backgrounding) — excluding `&&`
	// (logical and), `>&`/`&>` (fd redirects like `2>&1`), and `|&` (bash pipe-both)
	// by requiring the preceding char not be `&`, `>`, or `|` and the following not
	// be `&` or `>`.
	bgAmp = regexp.MustCompile(`(^|[^&>|])&([^&>]|$)`)

	// envAssignPrefix strips a run of NAME=VALUE assignments (optionally led by
	// `env`) at command position, so `NODE_ENV=prod npm run dev` still classifies
	// on `npm`. It runs on the raw command (before quotes are blanked), so the
	// value alternation handles quoted values with spaces (`FOO="a b" npm …`).
	// Mirrors the pi-extension's stripEnvPrefix; pi's lookahead is dropped (RE2
	// has none) — the `+` over NAME=VALUE tokens stops at the real command word.
	envAssignPrefix = regexp.MustCompile(`(^|[;&|]\s*)(env\s+)?([A-Za-z_][A-Za-z0-9_]*=("[^"]*"|'[^']*'|\S+)\s+)+`)

	// runtime: software that keeps running and should live in a named zmux tab.
	runtimePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(^|[;&|]\s*)(npm|pnpm|yarn|bun)\s+(run\s+)?(dev|serve|start:dev|watch)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(vite|next\s+dev|nuxt\s+dev|astro\s+dev|svelte-kit\s+dev)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(rails\s+s|rails\s+server|bin/rails\s+s)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)python\s+manage\.py\s+runserver\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(uvicorn|hypercorn|fastapi\s+dev|flask\s+run)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)air\b`),
		regexp.MustCompile(`(^|[;&|]\s*)go\s+run\s+\./(cmd/)?(server|api|web)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)cargo\s+(run|watch)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)make\s+(dev|serve|server|run|watch|start)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(watchexec|entr|nodemon|ts-node-dev)\b`),
	}

	// dockerComposeUpSeg matches `docker compose up` at the head of an
	// operator-split segment. It's handled apart from runtimePatterns because the
	// detached form (`-d`/`--detach`) hands the stack to dockerd and returns in
	// ~1s — a one-shot that stays safe; only the foreground form, which streams
	// logs, belongs in a visible zmux tab. RE2 has no lookahead, so the detach
	// exemption is a second per-segment regex rather than a negative match.
	dockerComposeUpSeg = regexp.MustCompile(`^\s*docker\s+compose\s+up\b`)
	detachFlag         = regexp.MustCompile(`(^|\s)(-d|--detach)(\s|$)`)

	// interactive: needs shared visibility / manual input.
	interactivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(^|[;&|]\s*)sudo\b`),
		regexp.MustCompile(`(^|[;&|]\s*)su\b`),
		regexp.MustCompile(`(^|[;&|]\s*)ssh\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(psql|mysql|sqlite3|redis-cli)\b`),
		regexp.MustCompile(`(^|[;&|]\s*)(python|node|irb|pry|iex|ghci)\s*$`),
	}

	// segSplit breaks a (quote-stripped) command into simple-command segments on
	// shell control operators, so each can be checked for a command-position tmux.
	segSplit = regexp.MustCompile(`[;&|\n]+`)

	// heredocStart matches a here-document redirection (`<<EOF`, `<<-'EOF'`,
	// `<< "EOF"`) and captures the delimiter word. RE2 can't backreference it, so
	// stripHeredocs closes the body with a line scan rather than one regex.
	heredocStart = regexp.MustCompile(`<<-?\s*["']?([A-Za-z_][A-Za-z0-9_]*)["']?`)
)

// tmuxTargets maps a raw tmux subcommand (long form + common alias) to the
// semantic zmux suggestion key. A subcommand absent here has no clean zmux
// equivalent (info, has-session, …) and is left alone.
var tmuxTargets = map[string]string{
	"capture-pane": "watch", "capturep": "watch",
	"send-keys": "send", "send": "send",
	"list-windows": "tabs", "lsw": "tabs",
	"list-sessions": "ls", "ls": "ls",
	"list-panes": "pane-list", "lsp": "pane-list",
	"split-window": "pane-open", "splitw": "pane-open",
	"select-pane": "pane-focus", "selectp": "pane-focus",
	"kill-pane": "pane-close", "killp": "pane-close",
	"resize-pane": "pane-resize", "resizep": "pane-resize",
	"new-window": "run", "neww": "run",
	"kill-window": "tab-kill", "killw": "tab-kill",
	"rename-window": "tab-label", "renamew": "tab-label",
	"move-window": "tab-move", "movew": "tab-move",
	"select-window": "tabs", "selectw": "tabs",
	"new-session": "new", "new": "new",
	"kill-session":   "session-kill",
	"attach-session": "open", "attach": "open",
	"switch-client": "open", "switchc": "open",
}

// Classify returns the verdict for command under opts. It never panics.
// An explicit bypass (ZMUX_ALLOW=1 / "# zmux: allow") keeps the command's
// natural Kind but forces the Decision to Allow — so logs still show what was
// waved through.
func Classify(command string, opts Options) Result {
	res := classify(command, opts)
	if res.Decision != Allow && (bypassEnv.MatchString(command) || bypassComment.MatchString(command)) {
		res.Decision = Allow
		res.Reason = "explicit bypass (" + string(res.Kind) + ")"
	}
	return res
}

func classify(command string, opts Options) Result {
	// Pipeline: strip env-var prefixes (quote-aware) → blank here-doc bodies →
	// blank quoted spans, all BEFORE the dimension scans. Env-strip is first so
	// `FOO="bar baz" npm run dev` classifies on `npm`; heredoc-strip is before
	// quote-strip so a `<<'EOF'` delimiter survives to bound the body it removes.
	scan := stripQuotedSegments(stripHeredocs(stripEnvPrefix(command)))

	if bgWord.MatchString(scan) || bgAmp.MatchString(scan) {
		return Result{Background, Block, "runtime", "background job hides process state — run it in a named zmux tab"}
	}

	// Raw-tmux dimension: a blockable invocation wins outright. An exempt one
	// (socket/repo) is only a fallback, applied after interactive/runtime are
	// ruled out — so `tmux -L s x && npm run dev` still blocks the dev server.
	block, exemptSeen := scanTmux(scan, opts)
	if block != nil {
		return *block
	}

	for _, re := range interactivePatterns {
		if re.MatchString(scan) {
			return Result{Interactive, Warn, "interactive", "interactive/remote command — prefer a shared zmux tab so it stays visible"}
		}
	}

	if foregroundComposeUp(scan) {
		return Result{Runtime, Block, "runtime", "long-running process — start it with zmux run -n <name> -d"}
	}

	for _, re := range runtimePatterns {
		if re.MatchString(scan) {
			return Result{Runtime, Block, "runtime", "long-running process — start it with zmux run -n <name> -d"}
		}
	}

	if exemptSeen {
		return Result{DirectTmux, Allow, "", "exempt (zmux repo / socket-scoped)"}
	}
	return Result{Safe, Allow, "", ""}
}

// foregroundComposeUp reports whether any segment runs a foreground
// `docker compose up` (no `-d`/`--detach`). The detached form returns
// immediately and is safe; only the log-streaming foreground form is runtime.
func foregroundComposeUp(scan string) bool {
	for _, seg := range segSplit.Split(scan, -1) {
		if dockerComposeUpSeg.MatchString(seg) && !detachFlag.MatchString(seg) {
			return true
		}
	}
	return false
}

// scanTmux inspects each simple-command segment of scan for a command-position
// raw tmux call (first token == "tmux", not zmux/tmuxinator/an arg of echo/rg).
// A mapped, non-exempt subcommand returns a Block result; the bool reports
// whether any exempt (socket/repo) tmux invocation was seen, which classify uses
// as a fallback once nothing else is actionable. Scanning every segment closes
// the `tmux info; tmux capture-pane` first-match hole.
func scanTmux(scan string, opts Options) (*Result, bool) {
	exemptSeen := false
	for _, seg := range segSplit.Split(scan, -1) {
		toks := strings.Fields(seg)
		if len(toks) == 0 || toks[0] != "tmux" {
			continue
		}
		args := toks[1:]
		if opts.RepoCwd || hasSocketFlag(args) {
			exemptSeen = true
			continue
		}
		sub := tmuxSubcommand(strings.Join(args, " "))
		if target, ok := tmuxTargets[sub]; ok {
			return &Result{DirectTmux, Block, target, "raw tmux " + sub + " — use the zmux wrapper"}, exemptSeen
		}
		// unmapped subcommand (info, has-session, ...) — no zmux verb; keep scanning
	}
	return nil, exemptSeen
}

// hasSocketFlag reports whether a tmux arg list is socket-scoped (`-L <socket>`),
// marking it as zzmux/profile testing and therefore exempt.
func hasSocketFlag(args []string) bool {
	for _, a := range args {
		if a == "-L" || strings.HasPrefix(a, "-L") {
			return true
		}
	}
	return false
}

var flagWithArg = map[string]bool{"-L": true, "-f": true, "-S": true, "-c": true}

// tmuxSubcommand returns the first token after any global flags.
func tmuxSubcommand(rest string) string {
	toks := strings.Fields(rest)
	for i := 0; i < len(toks); {
		if strings.HasPrefix(toks[i], "-") {
			if flagWithArg[toks[i]] {
				i += 2
			} else {
				i++
			}
			continue
		}
		return toks[i]
	}
	return ""
}

// stripEnvPrefix removes leading NAME=VALUE assignments (optionally introduced
// by `env`) at command position so an env-prefixed command still matches on its
// real verb. Mirrors the pi-extension classifier.
func stripEnvPrefix(s string) string {
	return envAssignPrefix.ReplaceAllString(s, "$1")
}

// stripHeredocs blanks the body of any here-document (`cmd <<EOF` … `EOF`) so
// shell metacharacters or a `tmux` inside a literal body aren't read as
// operators/commands. The body is stdin data, never executed, so removing it is
// loss-free for classification. The opening line (carrying the real command) is
// kept; body and closing-delimiter lines are blanked, preserving line offsets.
func stripHeredocs(s string) string {
	if !strings.Contains(s, "<<") {
		return s
	}
	lines := strings.Split(s, "\n")
	tag := "" // non-empty while inside a here-doc body
	for i, line := range lines {
		if tag != "" {
			if strings.TrimSpace(line) == tag {
				tag = "" // closing delimiter reached
			}
			lines[i] = "" // blank body + closing-delimiter lines
			continue
		}
		if m := heredocStart.FindStringSubmatch(line); m != nil {
			tag = m[1]
		}
	}
	return strings.Join(lines, "\n")
}

// stripQuotedSegments blanks out single/double/back-quoted spans (length- and
// newline-preserving) so a token inside a string literal isn't mistaken for a
// real invocation. Ported from the pi-extension classifier.
func stripQuotedSegments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var quote rune
	escaped := false
	for _, ch := range s {
		if quote != 0 {
			if quote == '"' && !escaped && ch == '\\' {
				escaped = true
				b.WriteByte(' ')
				continue
			}
			if !escaped && ch == quote {
				quote = 0
			} else if ch == '\n' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
			escaped = false
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			quote = ch
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}
