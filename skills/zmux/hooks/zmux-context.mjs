#!/usr/bin/env node
// zmux-context — SessionStart hook (proactive priming).
//
// The guard hook (zmux-context's sibling) is *reactive*: it catches a raw-tmux
// or background-job slip after the agent reaches for it. This hook is the
// *proactive* half — at session start it tells the agent, up front, that it is
// inside a zmux session and which tabs already exist. Knowing the tabs by name
// is what makes the agent reuse them (`zmux watch server`) instead of spawning a
// fresh shell or probing with raw tmux. Re-injects on resume/compact too, when
// that context would otherwise have been summarized away.
//
// Scope: only primes when actually inside a tmux/zmux session (TMUX set). Outside
// tmux — or where zmux isn't installed — it injects nothing, so it's silent in
// projects that don't use zmux.
//
// Output contract (SessionStart): JSON on stdout with
// hookSpecificOutput.additionalContext. Plain stdout is ignored. Fails OPEN:
// any error → exit 0 with no output. A priming hook must never wedge startup.

import { execFileSync } from 'node:child_process'

function done(context) {
  if (context) {
    process.stdout.write(
      JSON.stringify({
        hookSpecificOutput: {
          hookEventName: 'SessionStart',
          additionalContext: context,
        },
      }),
    )
  }
  process.exit(0)
}

// We don't need the payload fields; only prime inside a live tmux/zmux session.
if (!process.env.TMUX) done(null)

function zmux(args) {
  return execFileSync('zmux', args, {
    encoding: 'utf8',
    timeout: 1500,
    stdio: ['ignore', 'pipe', 'ignore'],
  })
}

let session = ''
let dir = ''
try {
  const pane = JSON.parse(zmux(['pane', 'current', '--json']))
  session = typeof pane.Session === 'string' ? pane.Session : ''
  dir = typeof pane.Dir === 'string' ? pane.Dir : ''
} catch {
  done(null) // not a resolvable zmux session, or zmux absent → stay silent
}

if (!session) done(null)

// Tag this pane as the agent's home shell (origin=agent, scope=agent-shell).
// This is the root signal the reaper's origin inheritance needs: tabs the agent
// spawns with `zmux run` then inherit origin=agent (short idle TTL), while this
// shell itself is never auto-reaped. Idempotent, fire-and-forget, fail-open — a
// priming hook must never wedge startup.
try {
  zmux(['tab', 'mark-agent'])
} catch {
  // best-effort: an older zmux without `tab mark-agent`, or no resolvable pane
}

let tabs = ''
try {
  tabs = zmux(['tabs'])
    .split('\n')
    .map((l) => l.replace(/\s+$/, ''))
    .filter((l) => l.trim().length > 0)
    .slice(0, 15)
    .join('\n')
} catch {
  tabs = ''
}

const lines = [
  `[zmux] This shell is inside zmux session "${session}"${dir ? ` (cwd ${dir})` : ''}.`,
]
if (tabs) {
  lines.push(
    'Existing tabs in this session — reuse them by name, do not spawn fresh shells or reach for raw tmux:',
    tabs,
  )
}
lines.push(
  "Read a tab: `zmux watch <tab>` (read-only). Interact: `zmux send <tab> <keys>` / `zmux type <tab> '<text>'`. " +
    "New work: `zmux run '<cmd>' -n <name>` (add -d for servers). Other sessions: `zmux ls -s`. " +
    'A PreToolUse guard blocks raw tmux and background jobs (shell `&`/`nohup` and Bash `run_in_background`), so reach for zmux first.',
)

done(lines.join('\n'))
