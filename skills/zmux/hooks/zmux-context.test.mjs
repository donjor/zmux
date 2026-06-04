// SessionStart priming hook: verify it (1) stays silent outside tmux, (2) fails
// OPEN when zmux is broken/absent, and (3) emits a valid additionalContext
// payload carrying the session + tabs when inside a session. A stub `zmux` on a
// temp PATH makes the happy path deterministic without a live tmux server.
//
// Run: node --test skills/zmux/hooks/zmux-context.test.mjs

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, writeFileSync, chmodSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const hook = join(dirname(fileURLToPath(import.meta.url)), 'zmux-context.mjs')

// runHook returns { code, stdout }. Never throws — mirrors the hook's fail-open.
function runHook(env) {
  try {
    // Invoke node by absolute path so a stripped PATH only affects the hook's
    // own `zmux` lookup, not node resolution itself.
    const stdout = execFileSync(process.execPath, [hook], {
      input: '{"session_id":"t","source":"startup","cwd":"/tmp"}',
      env: { ...process.env, ...env },
      encoding: 'utf8',
    })
    return { code: 0, stdout }
  } catch (e) {
    return { code: e.status ?? 1, stdout: e.stdout?.toString() ?? '' }
  }
}

// stubZmuxDir writes an executable `zmux` that responds to the hook's two calls.
function stubZmuxDir(body) {
  const dir = mkdtempSync(join(tmpdir(), 'zmux-context-stub-'))
  const script = join(dir, 'zmux')
  writeFileSync(script, body)
  chmodSync(script, 0o755)
  return dir
}

test('stays silent (no output) when not inside tmux', () => {
  const { code, stdout } = runHook({ TMUX: '' })
  assert.equal(code, 0)
  assert.equal(stdout.trim(), '')
})

test('fails open (exit 0, no output) when zmux is absent', () => {
  // Empty PATH dir → `zmux` not found → execFileSync throws inside the hook.
  const dir = stubZmuxDir('#!/bin/sh\nexit 0\n')
  rmSync(join(dir, 'zmux')) // remove the stub so zmux truly can't be found
  try {
    const { code, stdout } = runHook({ TMUX: 'x', PATH: dir })
    assert.equal(code, 0)
    assert.equal(stdout.trim(), '')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('fails open when zmux errors (broken binary)', () => {
  const dir = stubZmuxDir('#!/bin/sh\nexit 3\n')
  try {
    const { code, stdout } = runHook({ TMUX: 'x', PATH: `${dir}:${process.env.PATH}` })
    assert.equal(code, 0)
    assert.equal(stdout.trim(), '')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('emits additionalContext with session + tabs inside a session', () => {
  const dir = stubZmuxDir(
    [
      '#!/bin/sh',
      'if [ "$1" = "pane" ]; then',
      '  echo \'{"Session":"dev","Dir":"/home/user/app"}\'',
      'elif [ "$1" = "tabs" ]; then',
      '  printf "  * 1: claude  claude  ~/app\\n  2: server  node  ~/app\\n"',
      'fi',
    ].join('\n') + '\n',
  )
  try {
    const { code, stdout } = runHook({ TMUX: 'x', PATH: `${dir}:${process.env.PATH}` })
    assert.equal(code, 0)
    const out = JSON.parse(stdout)
    assert.equal(out.hookSpecificOutput.hookEventName, 'SessionStart')
    const ctx = out.hookSpecificOutput.additionalContext
    assert.match(ctx, /session "dev"/)
    assert.match(ctx, /\/home\/user\/app/)
    assert.match(ctx, /server/) // tab name surfaced
    assert.match(ctx, /zmux run/) // how-to nudge present
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})
