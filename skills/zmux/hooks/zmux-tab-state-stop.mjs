#!/usr/bin/env node
// zmux-tab-state-stop — Stop hook (tab attention states, plan 026 P1).
//
// When a Claude turn ends inside a zmux tab, mark that tab `done` — or
// `attention` if its window isn't currently visible (--by-visibility; the
// zmux service decides by window membership). The human sees a glyph in the
// bar from any tab instead of polling agent tabs for idleness.
//
// Deliberately dumb (agent-CLI boundary: no adapter machinery): no transcript
// parsing, no output classification — the only signal used is "the turn
// ended". Targeting rides on $TMUX_PANE, inherited from the pane shell.
//
// Fails OPEN everywhere: outside tmux, zmux missing, dead pane — exit 0,
// no output. A status glyph must never wedge or noise up a Claude turn.

import { execFileSync } from 'node:child_process'

export function stopCommandArgs() {
  return ['tab', 'state', 'done', '--source', 'claude-stop', '--quiet', '--by-visibility']
}

export function shouldRun(env) {
  return Boolean(env.TMUX && env.TMUX_PANE)
}

function main() {
  if (!shouldRun(process.env)) return
  try {
    execFileSync('zmux', stopCommandArgs(), {
      timeout: 1500,
      stdio: ['ignore', 'ignore', 'ignore'],
    })
  } catch {
    // zmux absent or errored — the glyph just doesn't appear.
  }
}

// Run only as a hook process, not when imported by tests.
if (import.meta.url === `file://${process.argv[1]}`) {
  main()
  process.exit(0)
}
