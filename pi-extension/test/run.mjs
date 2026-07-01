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
  const {
    buildLogArgs,
    buildPaneOpenArgs,
    buildPiReloadScript,
    buildSessionListArgs,
    buildSessionRunArgs,
    buildSnapshotArgs,
    buildTabLabelArgs,
    buildTabMoveArgs,
    buildTabPeerArgs,
    buildTabPlacementArgs,
    buildTabStateArgs,
    buildTabStatusArgs,
    buildTmuxRespawnScript,
    buildZmuxRunArgs,
    detectUserInputPrompt,
    settledFreshCommandStatus,
    zmuxRunResultDetails,
  } = await import(join(outDir, 'src/zmux.js'));
  const { runFileStatus } = await import(join(outDir, 'src/shell.js'));
  const { reloadContinuationPath, shouldTriggerContinuation, takeReloadContinuation, writeReloadContinuation } = await import(join(outDir, 'src/reload-continuation.js'));
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
    ['zmux tab state failed worker --msg done', 'direct_zmux'],
    ['zmux tab status claude-peer --json', 'direct_zmux'],
    ['zmux tab label worker', 'direct_zmux'],
    ['zmux tab move worker other-session', 'direct_zmux'],
    ['zmux tab pane worker --right', 'direct_zmux'],
    ['zmux tabs', 'direct_zmux'],
    ['zmux ls -s', 'direct_zmux'],
    ['export CLAUDE_CODE_SESSION_ID=x\nexport PI_SESSION_FILE=/tmp/session.jsonl\nzmux tabs', 'direct_zmux'],
    ['zmux session run peer -n agy-peer -- agy', 'direct_zmux'],
    ['zmux session kill peer', 'direct_zmux'],
    ['zmux run --command "npm test" -n tests', 'direct_zmux'],
    ['zmux watch server -l 20', 'direct_zmux'],
    ['zmux log tail server -n 20', 'direct_zmux'],
    ['zmux snapshot --no-png', 'direct_zmux'],
    ['zmux terminal current --json', 'direct_zmux'],
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
  assert.equal(shouldTriggerContinuation('Reload complete. Continue with verification.'), true);
  assert.equal(shouldTriggerContinuation("Reload complete. Wait for the user's next instruction."), false);
  assert.equal(shouldTriggerContinuation('reload complete; wait for user next instruction'), false);
  assert.equal(detectUserInputPrompt('[sudo] password for user:')?.kind, 'sudo_password');
  assert.equal(detectUserInputPrompt('Enter passphrase for key ~/.ssh/id_ed25519:')?.kind, 'password');
  assert.equal(detectUserInputPrompt('Are you sure you want to continue connecting (yes/no/[fingerprint])?')?.kind, 'ssh_confirm');
  assert.equal(detectUserInputPrompt('Status: inactive'), undefined);
  assert.deepEqual(settledFreshCommandStatus({ cmdSeq: '7', cmdState: 'done', lastExit: '0' }, 7), { fresh: false, settled: false, state: 'done', cmdSeq: 7 });
  assert.deepEqual(settledFreshCommandStatus({ cmdSeq: '8', cmdState: 'running' }, 7), { fresh: true, settled: false, state: 'running', cmdSeq: 8 });
  assert.deepEqual(settledFreshCommandStatus({ cmdSeq: '8', cmdState: 'failed', lastExit: '2' }, 7), { fresh: true, settled: true, state: 'failed', exitCode: 2, cmdSeq: 8 });

  assert.deepEqual(buildPaneOpenArgs({ name: 'logs', command: 'npm run dev', cwd: '/repo', direction: 'right', size: '40%' }), [
    'pane', 'open', 'logs', '--cwd', '/repo', '-r', '40%', '--', 'bash', '-lc', 'npm run dev',
  ]);
  assert.deepEqual(buildZmuxRunArgs({ command: 'npm test', cwd: '/repo', tab: 'tests', timeoutSeconds: 45, lines: 80, session: 'zws/repo' }), [
    'run', '--command', 'npm test', '-n', 'tests', '-T', '45', '--lines', '80', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildZmuxRunArgs({ command: 'npm run dev', cwd: '/repo', tab: 'server', detach: true, keep: true, scope: 'daemon' }), [
    'run', '--command', 'npm run dev', '-n', 'server', '-d', '--keep', '--scope', 'daemon',
  ]);
  assert.deepEqual(buildSessionListArgs({ flat: true, workspace: 'donjor' }), ['ls', '-s', 'donjor']);
  assert.deepEqual(buildSessionRunArgs({ sessionName: 'peer', tab: 'agy-peer', command: 'agy --model fast', workspace: 'zmux', cwd: '/repo' }), [
    'session', 'run', 'peer', '-n', 'agy-peer', '--workspace', 'zmux', '--cwd', '/repo', '--', 'bash', '-lc', 'agy --model fast',
  ]);
  assert.deepEqual(buildTabStateArgs({ action: 'failed', tab: 'worker', msg: 'needs attention', byVisibility: true, session: 'zws/repo' }), [
    'tab', 'state', 'failed', 'worker', '--msg', 'needs attention', '--by-visibility', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildTabPeerArgs({ action: 'running', tab: 'claude-peer', role: 'claude', hostTab: 'ztab_host', hostPane: '%9', topic: 'plan review', ttl: '30m', source: 'peer', msg: 'ready', session: 'zws/repo' }), [
    'tab', 'peer', 'running', 'claude-peer', '--role', 'claude', '--host-tab', 'ztab_host', '--host-pane', '%9', '--topic', 'plan review', '--ttl', '30m', '--source', 'peer', '--msg', 'ready', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildTabStatusArgs({ tab: 'claude-peer', session: 'zws/repo' }), [
    'tab', 'status', 'claude-peer', '--json', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildTabLabelArgs({ label: 'api', target: '%42' }), ['tab', 'label', '--target', '%42', 'api']);
  assert.deepEqual(buildTabMoveArgs({ tab: 'api', destination: 'repo/sidecar', force: true }), ['tab', 'move', 'api', 'repo/sidecar', '--force']);
  assert.deepEqual(buildLogArgs({ action: 'tail', tab: 'server', session: 'zws/repo', lines: 40 }), ['log', 'tail', 'server', '-n', '40', '-s', 'zws/repo']);
  assert.deepEqual(buildLogArgs({ action: 'status', tab: 'server', session: 'zws/repo', lines: 40 }), ['log', 'status']);
  assert.deepEqual(buildSnapshotArgs({ noPng: true, panes: ['%1', '%2'], lines: 120, out: '/tmp/snap', json: true }), [
    'snapshot', '--no-png', '--pane', '%1', '--pane', '%2', '--lines', '120', '--out', '/tmp/snap', '--json',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'pane', tab: 'logs', into: 'pi', direction: 'right', size: '35%', session: 'zws/repo' }), [
    'tab', 'pane', 'logs', '--session', 'zws/repo', '--into', 'pi', '--right', '--size', '35%',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'show', pane: '%42', session: 'zws/repo', direction: 'right', size: '35%', after: true }), [
    'tab', 'show', '--session', 'zws/repo', '--pane', '%42',
  ]);

  const nonZero = await runFileStatus(process.execPath, ['-e', 'process.exit(7)']);
  assert.equal(nonZero.failed, true);
  assert.equal(nonZero.exitCode, 7);
  assert.deepEqual(zmuxRunResultDetails({ stdout: '', stderr: 'Error: command exited with code 7', failed: true, exitCode: 1 }, 'Error: command exited with code 7'), {
    zmuxExitCode: 1,
    failed: true,
    failureKind: 'command_exit',
    exitCode: 7,
    warning: 'command exited with 7',
  });
  assert.deepEqual(zmuxRunResultDetails({ stdout: '', stderr: 'Error: timeout after 5s', failed: true, exitCode: 1 }, 'Error: timeout after 5s'), {
    zmuxExitCode: 1,
    failed: true,
    failureKind: 'zmux_timeout',
    timeoutSeconds: 5,
    warning: 'zmux run timed out after 5s',
  });
  assert.deepEqual(zmuxRunResultDetails({ stdout: '', stderr: '', failed: true, exitCode: null, signal: 'SIGTERM', timedOut: true, message: 'tool timeout' }, ''), {
    zmuxExitCode: null,
    failed: true,
    signal: 'SIGTERM',
    failureKind: 'tool_timeout',
    warning: 'tool timeout',
  });

  delete process.env.PI_ZMUX_TMUX_SOCKET;
  process.env.PI_ZMUX_BIN = 'zzmux';
  const reloadZzmux = buildPiReloadScript({ cwd: '/repo', pane: '%42', delayMs: 750, retryAttempts: 2, retryDelayMs: 1500 });
  assert.match(reloadZzmux, /^cd '\/repo'\n/);
  assert.ok(reloadZzmux.includes("warning='Wait for the current response to finish before reloading.'"));
  assert.ok(reloadZzmux.includes("'tmux' '-L' 'zzmux' 'capture-pane' '-t' '%42' '-p' '-S' '-' '-J'"));
  assert.ok(reloadZzmux.includes("sleep 0.75"));
  assert.ok(reloadZzmux.includes("while [ \"$attempt\" -le 2 ]; do"));
  assert.ok(reloadZzmux.includes("'tmux' '-L' 'zzmux' 'send-keys' '-t' '%42' '/reload' 'Enter'"));
  assert.ok(reloadZzmux.includes("sleep 1.5"));
  assert.equal(spawnSync('bash', ['-n'], { input: reloadZzmux }).status, 0, 'reload retry script must parse as bash');
  assert.equal(buildTmuxRespawnScript({ cwd: '/repo', pane: '%42', command: 'pi -c', delayMs: 300 }), "cd '/repo'; sleep 0.3; 'tmux' '-L' 'zzmux' 'respawn-pane' '-k' '-t' '%42' '-c' '/repo' 'pi -c'");
  process.env.PI_ZMUX_TMUX_SOCKET = 'edge';
  const reloadEdge = buildPiReloadScript({ cwd: '/repo', pane: '%42', delayMs: 0 });
  assert.ok(reloadEdge.includes("sleep 0"));
  assert.ok(reloadEdge.includes("while [ \"$attempt\" -le 3 ]; do"));
  assert.ok(reloadEdge.includes("'tmux' '-L' 'edge' 'send-keys' '-t' '%42' '/reload' 'Enter'"));
  assert.equal(spawnSync('bash', ['-n'], { input: reloadEdge }).status, 0, 'default reload retry script must parse as bash');
  assert.equal(buildTmuxRespawnScript({ cwd: '/repo', pane: '%42', command: 'pi -c', delayMs: 0 }), "cd '/repo'; sleep 0; 'tmux' '-L' 'edge' 'respawn-pane' '-k' '-t' '%42' '-c' '/repo' 'pi -c'");
  delete process.env.PI_ZMUX_BIN;
  delete process.env.PI_ZMUX_TMUX_SOCKET;

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
  assert.equal(loaded.projectTrusted, true);

  loaded = loadConfig(project, { projectTrusted: false });
  assert.equal(loaded.policy.mode, 'enforce');
  assert.equal(loaded.ignoredReason, 'project-untrusted');
  assert.equal(loaded.path.endsWith('.pi/zmux.json'), true);
  assert.deepEqual(loaded.runtimes, {});

  process.env.PI_ZMUX_POLICY = 'observe';
  loaded = loadConfig(project);
  assert.equal(loaded.policy.mode, 'observe');

  const reloadContinuationRoot = mkdtempSync(join(tmpdir(), 'pi-zmux-reload-continuation-'));
  const reloadPath = writeReloadContinuation(reloadContinuationRoot, { createdAt: '2026-05-04T00:00:00.000Z', prompt: 'continue after reload' });
  assert.equal(reloadPath, reloadContinuationPath(reloadContinuationRoot));
  assert.equal(takeReloadContinuation(reloadContinuationRoot)?.prompt, 'continue after reload');
  assert.equal(existsSync(reloadPath), false);

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
