#!/usr/bin/env node
// zmux-guard — PreToolUse:Bash gate (agent terminal-hygiene enforcement).
//
// Three slips this catches, in priority order:
//   1. raw `tmux` on a zmux-managed session — drops the @zmux_label pin +
//      session/workspace bookkeeping that keep tabs stably addressable (report
//      002). The skill *says* "never raw tmux"; Claude slipped anyway, twice,
//      deep in a long session. Advisory text is necessary but insufficient.
//   2. dev servers / background jobs in the agent's own shell — invisible to the
//      user and dead at end-of-turn. They belong in a named zmux tab.
//   3. interactive/remote commands (sudo, ssh, REPLs) — warned, not blocked, so
//      they live in a shared tab the user can actually see and drive.
//
// The classifier below MIRRORS internal/guard/guard.go and is held in lockstep
// with it by the shared golden corpus at testdata/zmux-guard-corpus.jsonl (the
// Go test and zmux-guard.test.mjs both assert against it — neither impl can
// silently drift). The Go `zmux guard` CLI is the equivalent surface for codex /
// other shell agents; this self-contained JS keeps the hot global hook fast and
// dependency-free.
//
// Decisions → exit codes: block → exit 2 (stderr fed to Claude, tool refused);
// warn → exit 0 + stderr nudge (proceeds); allow → exit 0, silent.
//
// Fails OPEN: any parse/runtime error allows the command. A guard must never
// wedge the Bash pipeline.

import { readFileSync, realpathSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

// ---------------------------------------------------------------------------
// Classifier — keep in lockstep with internal/guard/guard.go (corpus is the gate)
// ---------------------------------------------------------------------------

const BYPASS_ENV = /(^|\s)ZMUX_ALLOW=1(\s|$)/
const BYPASS_COMMENT = /#\s*zmux:\s*allow\b/i

const BG_WORD = /(^|\s)(nohup|disown)\b/
// A lone `&` control operator (backgrounding) — excluding `&&`, `>&`/`&>`
// redirects, and `|&` (bash pipe-both) by requiring the preceding char not be
// `&`, `>`, or `|` and the following not be `&` or `>`.
const BG_AMP = /(^|[^&>|])&([^&>]|$)/

// Strips a run of NAME=VALUE assignments (optionally led by `env`) at command
// position, so `NODE_ENV=prod npm run dev` still classifies on `npm`. Runs on the
// raw command (before quotes are blanked), so the value alternation handles
// quoted values with spaces. Mirrors guard.go's envAssignPrefix.
const ENV_ASSIGN_PREFIX = /(^|[;&|]\s*)(env\s+)?([A-Za-z_][A-Za-z0-9_]*=("[^"]*"|'[^']*'|\S+)\s+)+/g

// runtime: software that keeps running and belongs in a named zmux tab.
const RUNTIME = [
  /(^|[;&|]\s*)(npm|pnpm|yarn|bun)\s+(run\s+)?(dev|serve|start:dev|watch)\b/,
  /(^|[;&|]\s*)(vite|next\s+dev|nuxt\s+dev|astro\s+dev|svelte-kit\s+dev)\b/,
  /(^|[;&|]\s*)(rails\s+s|rails\s+server|bin\/rails\s+s)\b/,
  /(^|[;&|]\s*)python\s+manage\.py\s+runserver\b/,
  /(^|[;&|]\s*)(uvicorn|hypercorn|fastapi\s+dev|flask\s+run)\b/,
  /(^|[;&|]\s*)air\b/,
  /(^|[;&|]\s*)go\s+run\s+\.\/(cmd\/)?(server|api|web)\b/,
  /(^|[;&|]\s*)cargo\s+(run|watch)\b/,
  /(^|[;&|]\s*)make\s+(dev|serve|server|run|watch|start)\b/,
  /(^|[;&|]\s*)(watchexec|entr|nodemon|ts-node-dev)\b/,
]

// docker compose up is runtime only in its foreground form. The detached form
// (-d/--detach) hands the stack to dockerd and returns in ~1s — a one-shot that
// stays safe. Checked per-segment (matching guard.go) so a detach flag in one
// segment can't excuse a foreground compose in another. Mirrors guard.go.
const DOCKER_COMPOSE_UP_SEG = /^\s*docker\s+compose\s+up\b/
const DETACH_FLAG = /(^|\s)(-d|--detach)(\s|$)/

// interactive: needs shared visibility / manual input.
const INTERACTIVE = [
  /(^|[;&|]\s*)sudo\b/,
  /(^|[;&|]\s*)su\b/,
  /(^|[;&|]\s*)ssh\b/,
  /(^|[;&|]\s*)(psql|mysql|sqlite3|redis-cli)\b/,
  /(^|[;&|]\s*)(python|node|irb|pry|iex|ghci)\s*$/,
]

// segSplit breaks a (quote-stripped) command into simple-command segments on
// shell control operators, so each can be checked for a command-position tmux.
const SEG_SPLIT = /[;&|\n]+/

// Matches a here-document redirection (`<<EOF`, `<<-'EOF'`, `<< "EOF"`),
// capturing the delimiter word so stripHeredocs can close the body by line scan.
const HEREDOC_START = /<<-?\s*["']?([A-Za-z_][A-Za-z0-9_]*)["']?/

// tmux subcommand (long form + common alias) -> semantic suggestion key. A
// subcommand absent here has no clean zmux verb (info, has-session, ...) and is
// left alone.
const TMUX_TARGETS = {
  'capture-pane': 'watch', capturep: 'watch',
  'send-keys': 'send', send: 'send',
  'list-windows': 'tabs', lsw: 'tabs',
  'list-sessions': 'ls', ls: 'ls',
  'list-panes': 'pane-list', lsp: 'pane-list',
  'split-window': 'pane-open', splitw: 'pane-open',
  'select-pane': 'pane-focus', selectp: 'pane-focus',
  'kill-pane': 'pane-close', killp: 'pane-close',
  'resize-pane': 'pane-resize', resizep: 'pane-resize',
  'new-window': 'run', neww: 'run',
  'kill-window': 'tab-kill', killw: 'tab-kill',
  'rename-window': 'tab-label', renamew: 'tab-label',
  'move-window': 'tab-move', movew: 'tab-move',
  'select-window': 'tabs', selectw: 'tabs',
  'new-session': 'new', new: 'new',
  'kill-session': 'session-kill',
  'attach-session': 'open', attach: 'open',
  'switch-client': 'open', switchc: 'open',
}

// Concrete zmux form for each semantic target — the shell surface Claude sees.
const SUGGEST = {
  watch: 'zmux watch <tab>   (read-only; --until baselines the buffer. `zmux snapshot` for a PNG/ANSI bundle)',
  send: "zmux send <tab> <keys>   (or `zmux type <tab> 'text'`)",
  tabs: 'zmux tabs',
  ls: 'zmux ls   (or `zmux ls -s` for a flat list)',
  'pane-list': 'zmux pane list --json',
  'pane-open': 'zmux pane open <name> -r 35 -- <cmd>',
  'pane-focus': 'zmux pane focus <pane>',
  'pane-close': 'zmux pane close <pane>',
  'pane-resize': 'zmux pane resize <pane> --size 40%',
  run: "zmux run '<cmd>' -n <name>   (add -d for servers)",
  'tab-kill': 'zmux tab kill <tab>',
  'tab-label': "zmux tab label '<label>'",
  'tab-move': 'zmux tab move <tab> <dest-session>',
  new: 'zmux new <ws> [session]   (or `zmux open <ws> <session>`)',
  'session-kill': 'zmux session kill <session>   (or `zmux kill <name>`)',
  open: 'zmux open <ws> [session]',
  runtime: "zmux run '<cmd>' -n <name> -d   (keeps it in a visible, named tab)",
  interactive: "run it in a shared tab — zmux run '<cmd>' -n admin -d, then drive it with zmux send/type",
}

const FLAG_WITH_ARG = new Set(['-L', '-f', '-S', '-c'])

// tmuxSub returns the first token after any global flags (flags taking a value
// skip their value too), mirroring guard.go's tmuxSubcommand.
function tmuxSub(rest) {
  const toks = rest.trim().split(/\s+/).filter(Boolean)
  let i = 0
  while (i < toks.length) {
    if (toks[i].startsWith('-')) {
      i += FLAG_WITH_ARG.has(toks[i]) ? 2 : 1
      continue
    }
    return toks[i]
  }
  return ''
}

// scanTmux inspects each simple-command segment for a command-position raw tmux
// call (first token === 'tmux'). Returns { block } for a mapped, non-exempt
// subcommand, and exemptSeen for any socket/repo invocation (classify's fallback).
// Scans every segment, closing the `tmux info; tmux capture-pane` first-match hole.
function scanTmux(scan, opts) {
  let exemptSeen = false
  for (const seg of scan.split(SEG_SPLIT)) {
    const toks = seg.trim().split(/\s+/).filter(Boolean)
    if (toks.length === 0 || toks[0] !== 'tmux') continue
    const args = toks.slice(1)
    if (opts.repoCwd || hasSocketFlag(args)) {
      exemptSeen = true
      continue
    }
    const sub = tmuxSub(args.join(' '))
    const target = TMUX_TARGETS[sub]
    if (target) {
      return {
        block: { kind: 'direct_tmux', decision: 'block', target, reason: `raw tmux ${sub} — use the zmux wrapper` },
        exemptSeen,
      }
    }
    // unmapped subcommand (info, has-session, ...) — no zmux verb; keep scanning
  }
  return { block: null, exemptSeen }
}

// hasSocketFlag reports whether a tmux arg list is socket-scoped (`-L <socket>`).
function hasSocketFlag(args) {
  return args.some((a) => a === '-L' || a.startsWith('-L'))
}

// stripEnvPrefix removes leading NAME=VALUE assignments (optionally introduced
// by `env`) at command position. Mirrors guard.go's stripEnvPrefix.
export function stripEnvPrefix(s) {
  return s.replace(ENV_ASSIGN_PREFIX, '$1')
}

// stripHeredocs blanks the body of any here-document (`cmd <<EOF` … `EOF`) so
// shell metacharacters or a `tmux` inside a literal body aren't read as
// operators/commands. The body is stdin data, never executed, so removing it is
// loss-free. The opening line (the real command) is kept; body + closing
// delimiter lines are blanked. Mirrors guard.go's stripHeredocs.
export function stripHeredocs(s) {
  if (!s.includes('<<')) return s
  const lines = s.split('\n')
  let tag = '' // non-empty while inside a here-doc body
  for (let i = 0; i < lines.length; i++) {
    if (tag) {
      if (lines[i].trim() === tag) tag = '' // closing delimiter reached
      lines[i] = '' // blank body + closing-delimiter lines
      continue
    }
    const m = lines[i].match(HEREDOC_START)
    if (m) tag = m[1]
  }
  return lines.join('\n')
}

// stripQuotedSegments blanks out single/double/back-quoted spans (length- and
// newline-preserving) so a token inside a string literal — `echo "tmux ..."` —
// isn't mistaken for a real invocation. Ported from the pi-extension classifier.
export function stripQuotedSegments(s) {
  let out = ''
  let quote
  let escaped = false
  for (const ch of s) {
    if (quote) {
      if (quote === '"' && !escaped && ch === '\\') {
        escaped = true
        out += ' '
        continue
      }
      if (!escaped && ch === quote) quote = undefined
      else out += ch === '\n' ? '\n' : ' '
      escaped = false
      continue
    }
    if (ch === "'" || ch === '"' || ch === '`') {
      quote = ch
      out += ' '
      continue
    }
    out += ch
  }
  return out
}

// classify returns { kind, decision, target, reason } for command. Mirrors
// guard.go classify() exactly: background → tmux (blockable wins; exempt is a
// foregroundComposeUp reports whether any segment runs a foreground
// `docker compose up` (no -d/--detach) — the only compose-up form that streams
// logs and belongs in a tab. Mirrors guard.go's foregroundComposeUp.
function foregroundComposeUp(scan) {
  for (const seg of scan.split(SEG_SPLIT)) {
    if (DOCKER_COMPOSE_UP_SEG.test(seg) && !DETACH_FLAG.test(seg)) return true
  }
  return false
}

// fallback) → interactive → runtime → safe. Pure; never throws on normal input.
export function classify(command, opts = {}) {
  // Pipeline: env-strip (quote-aware) → blank here-doc bodies → blank quoted
  // spans, all before the dimension scans. Env-strip first so `FOO="bar baz" npm
  // run dev` classifies on `npm`; heredoc-strip before quote-strip so a `<<'EOF'`
  // delimiter survives to bound the body it removes.
  const scan = stripQuotedSegments(stripHeredocs(stripEnvPrefix(command)))

  if (BG_WORD.test(scan) || BG_AMP.test(scan)) {
    return { kind: 'background', decision: 'block', target: 'runtime', reason: 'background job hides process state — run it in a named zmux tab' }
  }

  // A blockable raw tmux wins outright; an exempt one (socket/repo) is only a
  // fallback once interactive/runtime are ruled out, so `tmux -L s x && npm run
  // dev` still blocks the dev server.
  const { block, exemptSeen } = scanTmux(scan, opts)
  if (block) return block

  for (const re of INTERACTIVE) {
    if (re.test(scan)) {
      return { kind: 'interactive', decision: 'warn', target: 'interactive', reason: 'interactive/remote command — prefer a shared zmux tab so it stays visible' }
    }
  }

  if (foregroundComposeUp(scan)) {
    return { kind: 'runtime', decision: 'block', target: 'runtime', reason: 'long-running process — start it with zmux run -n <name> -d' }
  }

  for (const re of RUNTIME) {
    if (re.test(scan)) {
      return { kind: 'runtime', decision: 'block', target: 'runtime', reason: 'long-running process — start it with zmux run -n <name> -d' }
    }
  }

  if (exemptSeen) {
    return { kind: 'direct_tmux', decision: 'allow', target: '', reason: 'exempt (zmux repo / socket-scoped)' }
  }
  return { kind: 'safe', decision: 'allow', target: '', reason: '' }
}

// guard is classify + bypass: an explicit ZMUX_ALLOW=1 / "# zmux: allow" keeps
// the natural Kind but forces the Decision to Allow (logs still show what was
// waved through). Mirrors guard.go's exported Classify.
export function guard(command, opts = {}) {
  const res = classify(command, opts)
  if (res.decision !== 'allow' && (BYPASS_ENV.test(command) || BYPASS_COMMENT.test(command))) {
    return { ...res, decision: 'allow', reason: `explicit bypass (${res.kind})` }
  }
  return res
}

// repoCwdFromPath reports whether cwd sits inside zmux's own source tree, where
// raw tmux is a legitimate dev tool (matches the Go CLI's go.mod walk in spirit).
export function repoCwdFromPath(cwd) {
  return /\/donjor\/zmux(\b|\/|\.)/.test(cwd || '')
}

// render produces the stderr message for a non-allow verdict.
export function render(res) {
  const suggest = SUGGEST[res.target]
  const arrow = suggest ? `\n  → ${suggest}\n` : '\n'

  if (res.kind === 'direct_tmux') {
    return [
      `zmux-guard: ${res.reason}.`,
      arrow.trimEnd(),
      ``,
      `That exact slip is why this gate exists. zmux wraps tmux; reaching past it drops`,
      `the @zmux_label pin + session/workspace bookkeeping that keep tabs stably`,
      `addressable. Read state with \`zmux watch <tab>\` (read-only; --until baselines`,
      `the buffer) — never re-probe with raw capture-pane.`,
      ``,
      `Map: capture-pane→watch · send-keys→send/type · list-windows→tabs · list-sessions→ls`,
      `· list-panes→pane list · split-window→pane open · *-pane→pane focus/close/resize`,
      `· *-window→tab kill/label/move.`,
      ``,
      `Genuinely need raw tmux (zmux dev, socket inspection)? Prefix \`ZMUX_ALLOW=1\`,`,
      `append \`# zmux: allow\`, add an explicit \`-L <socket>\`, or run from the zmux repo.`,
    ].join('\n')
  }

  if (res.decision === 'warn') {
    return [
      `zmux-guard (warn): ${res.reason}.`,
      arrow.trimEnd(),
      `Not blocking — but a sudo/ssh/REPL in a private shell is invisible to the user.`,
      `Bypass the nudge with \`ZMUX_ALLOW=1\` or \`# zmux: allow\`.`,
    ].join('\n')
  }

  // runtime / background block
  return [
    `zmux-guard: ${res.reason}.`,
    arrow.trimEnd(),
    ``,
    `A dev server / long-running job in your own shell is invisible to the user and`,
    `dies at end-of-turn. Put it in a named zmux tab so it's shared and inspectable`,
    `(\`zmux watch <name>\`). One-off builds/tests are fine — this is for things that keep running.`,
    ``,
    `Genuinely need it inline? Prefix \`ZMUX_ALLOW=1\` or append \`# zmux: allow\`.`,
  ].join('\n')
}

// ---------------------------------------------------------------------------
// Hook entry — only runs when invoked directly (not when imported by the test).
// ---------------------------------------------------------------------------

function main() {
  let payload
  try {
    payload = JSON.parse(readFileSync(0, 'utf8') || '{}')
  } catch {
    process.exit(0)
  }

  const command = payload?.tool_input?.command
  const cwd = payload?.cwd || ''
  if (typeof command !== 'string' || command.length === 0) process.exit(0)

  let res
  try {
    res = guard(command, { repoCwd: repoCwdFromPath(cwd) })
  } catch {
    process.exit(0)
  }

  if (res.decision === 'allow') process.exit(0)

  process.stderr.write(render(res) + '\n')
  process.exit(res.decision === 'block' ? 2 : 0)
}

// Robust entry detection across the ~/.claude/hooks symlink: compare realpaths.
// On any resolution failure, do NOT run main (avoid reading fd 0 on import).
function isMainEntry() {
  try {
    if (!process.argv[1]) return false
    return realpathSync(process.argv[1]) === realpathSync(fileURLToPath(import.meta.url))
  } catch {
    return false
  }
}

if (isMainEntry()) main()
