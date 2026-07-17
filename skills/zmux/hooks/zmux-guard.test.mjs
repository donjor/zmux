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
import { execFileSync, spawnSync } from 'node:child_process'

import { classify, guard, isPureZmuxWait, sprawlNudge } from './zmux-guard.mjs'

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

function runHook(command, cwd = '/home/user/some/project', toolInput = {}) {
  const payload = JSON.stringify({ tool_input: { command, ...toolInput }, cwd })
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

// --- adapter: Bash `run_in_background: true` is the harness-native long-running
// path the string classifier can't see (report 013) ---

test('hook exits 2 on a safe command launched with run_in_background', () => {
  const cmd = 'timeout 6000 bun run scripts/eval.ts --cap 12'
  assert.equal(runHook(cmd), 0) // command string alone is safe…
  assert.equal(runHook(cmd, undefined, { run_in_background: true }), 2) // …the bg flag blocks it
})

test('run_in_background block honors an explicit bypass token', () => {
  const cmd = 'timeout 6000 bun run scripts/eval.ts # zmux: allow'
  assert.equal(runHook(cmd, undefined, { run_in_background: true }), 0)
})

test('run_in_background: false leaves a safe command alone', () => {
  assert.equal(runHook('git status', undefined, { run_in_background: false }), 0)
})

// --- adapter: pure `zmux wait` is the conductor wake channel, allowed in
// background; anything chained onto it forfeits the carve-out ---

test('isPureZmuxWait accepts a lone zmux wait and rejects chained/expanded forms', () => {
  assert.equal(isPureZmuxWait('zmux wait pi-worker -s worker-auth --for turn:ready,failed,attention -T 570 --json'), true)
  assert.equal(isPureZmuxWait('zmux wait x --for turn:ready && npm run dev'), false)
  assert.equal(isPureZmuxWait('zmux wait $(cmd) --for idle:3'), false)
  assert.equal(isPureZmuxWait('zmux wait x --for turn:ready | tee log'), false)
  // Command substitution executes inside double quotes, so it must be caught on
  // the raw command — the stripped scan blanks the quoted span and can't see it.
  assert.equal(isPureZmuxWait('zmux wait "$(rm -rf ~)" --for idle:3'), false)
  assert.equal(isPureZmuxWait('zmux wait "`rm -rf ~`" --for idle:3'), false)
  assert.equal(isPureZmuxWait('npm run dev'), false)
})

test('hook allows a pure zmux wait in background (conductor wake channel)', () => {
  const cmd = 'zmux wait pi-worker -s worker-auth --for turn:ready,failed,attention -T 570 --json'
  assert.equal(runHook(cmd, undefined, { run_in_background: true }), 0)
})

test('hook still blocks chained/expanded zmux wait in background', () => {
  assert.equal(runHook('npm run dev', undefined, { run_in_background: true }), 2)
  assert.equal(runHook('zmux wait x --for turn:ready && npm run dev', undefined, { run_in_background: true }), 2)
  assert.equal(runHook('zmux wait $(cmd) --for idle:3', undefined, { run_in_background: true }), 2)
  assert.equal(runHook('zmux wait "$(rm -rf ~)" --for idle:3', undefined, { run_in_background: true }), 2)
})

test('foreground zmux wait is unaffected (allow)', () => {
  assert.equal(runHook('zmux wait x --for turn:ready -T 30'), 0)
  assert.equal(runHook('zmux wait x --for turn:ready -T 30', undefined, { run_in_background: false }), 0)
})

// --- Part C: sprawl nudge (hook-only, non-blocking) --------------------------

test('sprawlNudge fires on an ad-hoc named bounded run, not on roster/durable/unnamed', () => {
  // Ad-hoc named bounded run → nudge.
  assert.match(sprawlNudge("zmux run 'go test ./...' -n eval-2"), /ad-hoc tab/)
  assert.match(sprawlNudge('zmux run "bun test" --name test-run'), /ad-hoc tab/)
  // Roster names keep their own tab → silent.
  assert.equal(sprawlNudge("zmux run 'npm run dev' -n dev"), null)
  assert.equal(sprawlNudge("zmux run 'claude' -n codex-peer"), null)
  assert.equal(sprawlNudge("zmux run 'x' -n worker-auth"), null)
  // Durable/no-exit runs legitimately keep a tab → silent.
  assert.equal(sprawlNudge("zmux run 'npm run dev' -n myserver -d"), null)
  assert.equal(sprawlNudge("zmux run 'x' -n keeper --keep"), null)
  assert.equal(sprawlNudge("zmux run 'redis' -n db --scope daemon"), null)
  // Unnamed run already defaults to scratch → nothing to nudge.
  assert.equal(sprawlNudge("zmux run 'go test ./...'"), null)
  // Not a zmux run at all → silent.
  assert.equal(sprawlNudge('git status'), null)
})

test('hook proceeds (exit 0) and prints the sprawl nudge on stderr', () => {
  // execFileSync only returns stdout; the nudge goes to stderr, so capture it
  // via spawnSync-style stdio. Non-blocking means status must stay 0.
  const payload = JSON.stringify({ tool_input: { command: "zmux run 'go test ./...' -n eval-2" }, cwd: '/home/user/some/project' })
  const res = spawnSync('node', [hookPath], { input: payload, encoding: 'utf8' })
  assert.equal(res.status, 0, 'ad-hoc bounded run must still proceed (non-blocking nudge)')
  assert.match(res.stderr, /ad-hoc tab/)
})

test('hook stays silent for an unnamed run (already scratch-defaulted)', () => {
  const payload = JSON.stringify({ tool_input: { command: "zmux run 'go test ./...'" }, cwd: '/home/user/some/project' })
  const res = spawnSync('node', [hookPath], { input: payload, encoding: 'utf8' })
  assert.equal(res.status, 0)
  assert.doesNotMatch(res.stderr, /ad-hoc tab/)
})
