import { existsSync, mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
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
    ['go test ./...', 'safe'],
    ['go run ./cmd/zmux', 'safe'],
    ['go run ./cmd/api', 'runtime'],
    ['cargo run', 'runtime'],
    ['docker compose up', 'runtime'],
    ['make serve', 'runtime'],
    ['zmux tab kill admin', 'direct_zmux'],
    ['zmux tabs', 'direct_zmux'],
    ['zmux watch server -l 20', 'direct_zmux'],
    ['tmux send-keys -t %347 l l l l l Enter', 'direct_tmux'],
    ['tmux kill-pane -t %347', 'direct_tmux'],
    ['tmux display-message -p "#{pane_id}"', 'safe'],
    ['rg -n "sudo|ssh|zmux tabs" src test', 'safe'],
    ['echo "tmux send-keys -t %347 l"', 'safe'],
    ['zmux terminal capabilities', 'safe'],
    ['sudo apt update', 'interactive'],
    ['ssh prod', 'interactive'],
    ['python server.py &', 'background'],
    ['nohup ./server', 'background'],
    ['npm test', 'safe'],
  ];
  for (const [command, want] of cases) {
    assert.equal(classifyBash(command, cfg).kind, want, command);
  }
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
