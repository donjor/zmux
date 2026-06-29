import { existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';

const root = new URL('..', import.meta.url).pathname.replace(/\/$/, '');
const outDir = mkdtempSync(join(tmpdir(), 'pi-zmux-test-'));
try {
  const tsc = join(root, 'node_modules/.bin/tsc');
  const compile = spawnSync(tsc, ['-p', join(root, 'tsconfig.json'), '--outDir', outDir, '--noEmit', 'false'], { stdio: 'inherit' });
  assert.equal(compile.status, 0, 'TypeScript compile failed');

  const { classifyBash, hasExplicitBypass, stripQuotedSegments } = await import(join(outDir, 'src/classify.js'));
  const { loadConfig } = await import(join(outDir, 'src/config.js'));
  const { detectUserInputPrompt } = await import(join(outDir, 'src/zmux.js'));
  const { respawnContinuationPath, takeRespawnContinuation, writeRespawnContinuation } = await import(join(outDir, 'src/respawn-continuation.js'));

  const cfg = { policy: { mode: 'enforce', blockBackgroundJobs: true, redirectInteractive: true }, runtimes: {} };
  const cases = [
    ['npm run dev', 'runtime'],
    ['pnpm dev && echo ok', 'runtime'],
    ['env PORT=3000 npm run dev', 'runtime'],
    ['export CLAUDE_CODE_SESSION_ID=x\nexport PI_SESSION_FILE=/tmp/session.jsonl\nnpm run dev', 'runtime'],
    ['go test ./...', 'safe'],
    ['go run ./cmd/zmux', 'safe'],
    ['go run ./cmd/api', 'runtime'],
    ['cargo run', 'runtime'],
    ['docker compose up', 'runtime'],
    ['make serve', 'runtime'],
    ['zmux tab kill admin', 'direct_zmux'],
    ['zmux tabs', 'direct_zmux'],
    ['export CLAUDE_CODE_SESSION_ID=x\nexport PI_SESSION_FILE=/tmp/session.jsonl\nzmux tabs', 'direct_zmux'],
    ['zmux watch server -l 20', 'direct_zmux'],
    ['tmux send-keys -t %347 l l l l l Enter', 'direct_tmux'],
    ['tmux kill-pane -t %347', 'direct_tmux'],
    ['tmux display-message -p "#{pane_id}"', 'safe'],
    ['rg -n "sudo|ssh|zmux tabs" src test', 'safe'],
    ['echo "tmux send-keys -t %347 l"', 'safe'],
    ['zmux terminal capabilities', 'safe'],
    ['sudo apt update', 'interactive'],
    ['export CLAUDE_CODE_SESSION_ID=x\nexport PI_SESSION_FILE=/tmp/session.jsonl\nsudo apt update', 'interactive'],
    ['ssh prod', 'interactive'],
    ['python server.py &', 'background'],
    ['nohup ./server', 'background'],
    ['npm test', 'safe'],
  ];
  for (const [command, want] of cases) {
    assert.equal(classifyBash(command, cfg).kind, want, command);
  }

  // Shared-corpus parity (the cross-impl drift gate). The corpus at
  // testdata/zmux-guard-corpus.jsonl is the source of truth for KIND, which is
  // agent-invariant — pi must agree with the Go classifier and the Claude hook.
  // Two documented divergences are excluded because they stem from pi's richer
  // surface, not a classification disagreement (see the corpus README):
  //   - `zmux ...` CLI calls: safe on the shell surface, but pi nudges to a typed
  //     tool (direct_zmux). pi-only, tested via the direct_zmux cases above.
  //   - socket-scoped tmux (`-L`): pi folds the exemption into `safe` (its block
  //     decision is kind-derived; the Go side keeps kind=direct_tmux + allow).
  const corpusPath = join(root, '..', 'testdata', 'zmux-guard-corpus.jsonl');
  const corpus = readFileSync(corpusPath, 'utf8').split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l));
  let corpusChecked = 0;
  for (const row of corpus) {
    if (/^\s*zmux\s/u.test(row.command)) continue; // pi direct_zmux territory
    // Socket-scoped tmux that the Go side classifies direct_tmux+allow is folded
    // into `safe` by pi (kind-derived decision) — skip only *those* rows, not any
    // row that merely mentions `-L`. A `tmux -L … && npm run dev` row is kind
    // `runtime` and MUST stay gated (pi still catches the dev server).
    if (row.kind === 'direct_tmux' && /(^|\s)-L(\s|=)/u.test(row.command)) continue;
    assert.equal(classifyBash(row.command, cfg).kind, row.kind, `corpus kind for ${JSON.stringify(row.command)} [${row.note}]`);
    corpusChecked++;
  }
  assert.ok(corpusChecked >= 60, `expected to check most corpus rows, got ${corpusChecked}`);
  console.log(`pi corpus parity: ${corpusChecked} rows matched`);
  assert.equal(stripQuotedSegments('rg -n "sudo|ssh|zmux tabs" src').includes('sudo'), false);
  assert.equal(stripQuotedSegments('rg -n "sudo|ssh|zmux tabs" src').includes('zmux tabs'), false);
  assert.equal(hasExplicitBypass('PI_ZMUX_ALLOW=1 zmux tabs'), true);
  assert.equal(hasExplicitBypass('zmux tabs # pi-zmux: allow'), true);
  assert.equal(hasExplicitBypass('zmux tabs'), false);
  assert.equal(detectUserInputPrompt('[sudo] password for user:')?.kind, 'sudo_password');
  assert.equal(detectUserInputPrompt('Enter passphrase for key ~/.ssh/id_ed25519:')?.kind, 'password');
  assert.equal(detectUserInputPrompt('Are you sure you want to continue connecting (yes/no/[fingerprint])?')?.kind, 'ssh_confirm');
  assert.equal(detectUserInputPrompt('Status: inactive'), undefined);

  const project = mkdtempSync(join(tmpdir(), 'pi-zmux-config-'));
  mkdirSync(join(project, '.pi'));
  writeFileSync(join(project, '.pi/zmux.json'), JSON.stringify({
    policy: { mode: 'warn', blockBackgroundJobs: false, redirectInteractive: false },
    runtimes: { server: { command: 'go run ./cmd/api', tab: 'api', readiness: 'ready' } },
  }));
  delete process.env.PI_ZMUX_POLICY;
  let loaded = loadConfig(project);
  assert.equal(loaded.policy.mode, 'warn');
  assert.equal(loaded.policy.blockBackgroundJobs, false);
  assert.equal(loaded.policy.redirectInteractive, false);
  assert.equal(loaded.runtimes.server.command, 'go run ./cmd/api');
  assert.equal(loaded.runtimes.server.tab, 'api');

  process.env.PI_ZMUX_POLICY = 'observe';
  loaded = loadConfig(project);
  assert.equal(loaded.policy.mode, 'observe');

  const continuationRoot = mkdtempSync(join(tmpdir(), 'pi-zmux-continuation-'));
  const continuationPath = writeRespawnContinuation(continuationRoot, { createdAt: '2026-05-04T00:00:00.000Z', prompt: 'continue smoke', handoffPath: '/tmp/handoff.md' });
  assert.equal(continuationPath, respawnContinuationPath(continuationRoot));
  assert.equal(takeRespawnContinuation(continuationRoot)?.prompt, 'continue smoke');
  assert.equal(existsSync(continuationPath), false);

  const malformed = mkdtempSync(join(tmpdir(), 'pi-zmux-bad-'));
  mkdirSync(join(malformed, '.pi'));
  writeFileSync(join(malformed, '.pi/zmux.json'), '{not json');
  loaded = loadConfig(malformed);
  assert.equal(loaded.policy.mode, 'observe');
  assert.deepEqual(loaded.runtimes, {});

  console.log('pi-zmux extension tests passed');
} finally {
  rmSync(outDir, { recursive: true, force: true });
}
