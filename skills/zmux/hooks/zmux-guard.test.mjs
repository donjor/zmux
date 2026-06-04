// Drift gate for the Claude hook's classifier. Runs the SAME shared corpus as
// internal/guard/guard_test.go (testdata/zmux-guard-corpus.jsonl), so the JS and
// Go implementations cannot silently disagree. Plus a few adapter checks that the
// hook process maps decisions to the right exit codes.
//
// Run: node --test skills/zmux/hooks/zmux-guard.test.mjs

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import { execFileSync } from 'node:child_process'

import { classify, guard } from './zmux-guard.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const hookPath = join(here, 'zmux-guard.mjs')
const corpusPath = join(here, '..', '..', '..', 'testdata', 'zmux-guard-corpus.jsonl')

function loadCorpus() {
  const rows = readFileSync(corpusPath, 'utf8')
    .split('\n')
    .filter((l) => l.trim().length > 0)
    .map((l) => JSON.parse(l))
  assert.ok(rows.length > 0, 'corpus is empty')
  return rows
}

test('JS classifier matches the shared corpus row-for-row', () => {
  for (const r of loadCorpus()) {
    const got = guard(r.command, { repoCwd: r.cwd === 'repo' })
    assert.equal(got.kind, r.kind, `kind for ${JSON.stringify(r.command)} [${r.note}]`)
    assert.equal(got.decision, r.decision, `decision for ${JSON.stringify(r.command)} [${r.note}]`)
  }
})

test('every blocked row carries a suggestion target', () => {
  for (const r of loadCorpus()) {
    if (r.decision !== 'block') continue
    const got = guard(r.command, { repoCwd: r.cwd === 'repo' })
    assert.ok(got.target && got.target.length > 0, `blocked ${JSON.stringify(r.command)} has no target`)
  }
})

test('bypass keeps the natural kind but flips decision to allow', () => {
  const blocked = classify('npm run dev', {})
  assert.equal(blocked.decision, 'block')
  const bypassed = guard('npm run dev # zmux: allow', {})
  assert.equal(bypassed.kind, 'runtime')
  assert.equal(bypassed.decision, 'allow')
})

// --- adapter: the hook process maps decision → exit code as documented ---

function runHook(command, cwd = '/home/user/some/project') {
  const payload = JSON.stringify({ tool_input: { command }, cwd })
  try {
    execFileSync('node', [hookPath], { input: payload, stdio: ['pipe', 'pipe', 'pipe'] })
    return 0
  } catch (e) {
    return e.status ?? 1
  }
}

test('hook exits 2 on a blocked command (raw tmux)', () => {
  assert.equal(runHook('tmux capture-pane -t main -p'), 2)
})

test('hook exits 2 on a dev server', () => {
  assert.equal(runHook('npm run dev'), 2)
})

test('hook exits 0 (warn, non-blocking) on ssh', () => {
  assert.equal(runHook('ssh build@host uptime'), 0)
})

test('hook exits 0 on a safe command', () => {
  assert.equal(runHook('git status'), 0)
})

test('hook exits 0 inside the zmux repo (raw tmux exempt)', () => {
  assert.equal(runHook('tmux capture-pane -t main -p', '/home/user/donjor/zmux'), 0)
})
