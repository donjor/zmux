// Stop-hook contract tests: command construction + fail-open gating.
// No transcript parsing, no output classification — if these tests grow
// either, the agent-CLI "no adapter machinery" boundary is being crossed.
//
// Run: node --test skills/zmux/hooks/zmux-tab-state-stop.test.mjs

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

import { stopCommandArgs, shouldRun } from './zmux-tab-state-stop.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const hookPath = join(here, 'zmux-tab-state-stop.mjs')

test('command is the quiet visibility-aware done write', () => {
  assert.deepEqual(stopCommandArgs(), [
    'tab',
    'state',
    'done',
    '--source',
    'claude-stop',
    '--quiet',
    '--by-visibility',
  ])
})

test('runs only inside a tmux pane', () => {
  assert.equal(shouldRun({ TMUX: '/tmp/tmux-1000/default,1,0', TMUX_PANE: '%5' }), true)
  assert.equal(shouldRun({ TMUX: '/tmp/tmux-1000/default,1,0' }), false, 'no pane id')
  assert.equal(shouldRun({ TMUX_PANE: '%5' }), false, 'no tmux')
  assert.equal(shouldRun({}), false)
})

test('hook process exits 0 outside tmux (fail-open, no output)', () => {
  const out = execFileSync(process.execPath, [hookPath], {
    encoding: 'utf8',
    env: { PATH: process.env.PATH }, // no TMUX/TMUX_PANE
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  assert.equal(out, '', 'must stay silent')
})

test('hook process exits 0 even when zmux is missing', () => {
  const out = execFileSync(process.execPath, [hookPath], {
    encoding: 'utf8',
    env: { TMUX: 'x,1,0', TMUX_PANE: '%1', PATH: '/nonexistent' },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  assert.equal(out, '', 'must stay silent')
})
