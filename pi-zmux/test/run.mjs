import { chmodSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';

const root = new URL('..', import.meta.url).pathname.replace(/\/$/, '');
const outDir = mkdtempSync(join(tmpdir(), 'pi-zmux-test-'));
try {
  const packageFiles = [
    'doctrine-manifest.generated.json',
    'fixtures/config-project/.pi/zmux.json',
    'fixtures/config-project/README.md',
    'fixtures/dev-server/package.json',
    'fixtures/dev-server/server.mjs',
    'fixtures/dev-server/logs/app.txt',
  ];
  for (const path of packageFiles) {
    assert.ok(existsSync(join(root, path)), `canonical package fixture missing: ${path}`);
  }

  const tsc = join(root, 'node_modules/.bin/tsc');
  const compile = spawnSync(tsc, ['-p', join(root, 'tsconfig.json'), '--outDir', outDir, '--noEmit', 'false'], { stdio: 'inherit' });
  assert.equal(compile.status, 0, 'TypeScript compile failed');
  symlinkSync(join(root, 'node_modules'), join(outDir, 'node_modules'), 'dir');

  const { classifyBash, hasExplicitBypass, stripQuotedSegments } = await import(join(outDir, 'src/classify.js'));
  const { loadConfig } = await import(join(outDir, 'src/config.js'));
  const {
    autoPaneResizeAxis,
    buildCallbackWatchArgs,
    buildLogArgs,
    clearCallbacks,
    buildPaneOpenArgs,
    buildPiReloadScript,
    buildSessionListArgs,
    buildSessionRunArgs,
    buildSnapshotArgs,
    buildTabKillArgs,
    buildTabLabelArgs,
    buildTabMoveArgs,
    buildTabPeerArgs,
    buildTabPlacementArgs,
    buildTabStateArgs,
    buildTabStatusArgs,
    buildTmuxRespawnScript,
    buildWatchArgs,
    buildWaitArgs,
    buildZmuxRunArgs,
    listCallbacks,
    listRecentCallbackCompletions,
    startWatchCallback,
    statusWarnings,
    watchPatternPresentInText,
    zmuxRunResultDetails,
    zmuxRunSafetyWarnings,
  } = await import(join(outDir, 'src/zmux.js'));
  const { detectUserInputPrompt } = await import(join(outDir, 'src/interactive.js'));
  const { runFileStatus } = await import(join(outDir, 'src/shell.js'));
  const { reloadContinuationPath, shouldTriggerContinuation, takeReloadContinuation, writeReloadContinuation } = await import(join(outDir, 'src/reload-continuation.js'));
  const { respawnContinuationPath, takeRespawnContinuation, writeRespawnContinuation } = await import(join(outDir, 'src/respawn-continuation.js'));
  const { hasHeadlessAgentPrintMode, rejectHeadlessAgentPrintMode, shouldWaitForExit } = await import(join(outDir, 'src/safety.js'));
  const { default: registerExtension } = await import(join(outDir, 'src/index.js'));
  const { default: registerPeerLifecycleExtension } = await import(join(outDir, 'src/peer-lifecycle.js'));

  const registeredTools = [];
  const registeredCommands = [];
  const registeredHandlers = [];
  const registeredMessageRenderers = [];
  const fakePi = {
    registerTool(tool) { registeredTools.push(tool); },
    registerMessageRenderer(customType, renderer) { registeredMessageRenderers.push({ customType, renderer }); },
    registerCommand(name, options) { registeredCommands.push({ name, options }); },
    on(event, handler) { registeredHandlers.push({ event, handler }); },
    sendMessage() {},
    sendUserMessage() {},
  };
  registerExtension(fakePi);
  const toolNames = registeredTools.map((tool) => tool.name).sort();
  assert.deepEqual(toolNames, ['zmux'], 'production Pi tool surface must expose only the canonical dispatcher');
  assert.equal(new Set(toolNames).size, toolNames.length, 'tool names must be unique');
  assert.deepEqual(registeredCommands.map((cmd) => cmd.name), ['zmux']);
  assert.deepEqual(registeredMessageRenderers.map((entry) => entry.customType), ['pi-zmux-callback']);
  for (const eventName of ['agent_start', 'agent_end', 'session_shutdown', 'session_start', 'tool_call']) {
    assert.ok(registeredHandlers.some((handler) => handler.event === eventName), `expected handler for ${eventName}`);
  }
  assert.ok(!registeredHandlers.some((handler) => handler.event === 'before_agent_start'), 'production must not inject zmux state into every agent run');

  const peerLifecycleTools = [];
  const peerLifecycleCommands = [];
  const peerLifecycleHandlers = [];
  registerPeerLifecycleExtension({
    registerTool(tool) { peerLifecycleTools.push(tool); },
    registerCommand(name, options) { peerLifecycleCommands.push({ name, options }); },
    on(event, handler) { peerLifecycleHandlers.push({ event, handler }); },
    sendMessage() {},
    sendUserMessage() {},
  });
  assert.deepEqual(peerLifecycleTools, [], 'peer lifecycle entrypoint must not register tools');
  assert.deepEqual(peerLifecycleCommands, [], 'peer lifecycle entrypoint must not register commands');
  assert.deepEqual(peerLifecycleHandlers.map((handler) => handler.event).sort(), ['agent_end', 'agent_start', 'session_shutdown'].sort(), 'peer lifecycle entrypoint must register only lifecycle hooks');

  const encodedRemoteMutation = Buffer.from("Set-Content /etc/example.env 'TOKEN=redacted'", 'utf16le').toString('base64');
  const remoteRunWarnings = zmuxRunSafetyWarnings({
    command: `ssh node-a "remote-admin -EncodedCommand ${encodedRemoteMutation}"`,
    cwd: root,
    tab: 'remote-example2',
  });
  assert.match(remoteRunWarnings.text, /reuse.*remote-example/i, 'numbered remote tabs should warn toward one stable tab');
  assert.match(remoteRunWarnings.text, /opaque encoded remote\/admin payload/i, 'opaque encoded remote command should be called out generically');
  assert.match(remoteRunWarnings.text, /about to change.*node-a/i, 'remote env mutation should require an explicit pre-change status');
  assert.equal(remoteRunWarnings.details.recommendedTab, 'remote-example');
  assert.match(remoteRunWarnings.details.decodedRemoteCommandPreview, /Set-Content/);
  assert.equal(shouldWaitForExit('ssh onyxrock'), false);
  assert.equal(shouldWaitForExit('bash --norc'), false);
  assert.equal(shouldWaitForExit('sudo ufw status'), true);
  assert.equal(shouldWaitForExit('sudo -i'), false);

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
    ['zmux run --command "claude --dangerously-skip-permissions" -n claude-peer -d', 'direct_zmux'],
    ['zmux tab status claude-peer --json && zmux watch claude-peer -l 80', 'direct_zmux'],
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
    ['claude -p "review this"', 'headless_agent'],
    ['PI_ZMUX_BIN=zzmux pi -p "review this"', 'headless_agent'],
    ['rg "claude -p" skills/zmux', 'safe'],
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
  assert.match(classifyBash('zmux pane list', cfg).suggestion, /operation=panes\b/);
  assert.match(classifyBash('zmux type worker hello', cfg).suggestion, /operation=type_text or interactive_type/);
  assert.match(classifyBash('tmux list-panes', cfg).suggestion, /operation=panes\b/);
  assert.doesNotMatch(classifyBash('zmux pane list', cfg).suggestion, /operation=pane_list\b/);

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
  assert.equal(hasHeadlessAgentPrintMode('claude -p "review this"'), true);
  assert.equal(hasHeadlessAgentPrintMode('echo "; claude -p is banned"'), false);
  assert.equal(rejectHeadlessAgentPrintMode('echo "; pi --print is banned"'), undefined);
  assert.equal(hasHeadlessAgentPrintMode('claude --dangerously-skip-permissions'), false);
  assert.match(rejectHeadlessAgentPrintMode('pi -p "review this"'), /do not launch agent peers/i);
  assert.equal(rejectHeadlessAgentPrintMode('pi --model openai/gpt-5.5'), undefined);
  assert.equal(hasExplicitBypass('PI_ZMUX_ALLOW=1 zmux tabs'), true);
  assert.equal(hasExplicitBypass('zmux tabs # pi-zmux: allow'), true);
  assert.equal(hasExplicitBypass('zmux tabs'), false);
  assert.deepEqual(listCallbacks(), []);
  assert.deepEqual(listRecentCallbackCompletions(), []);
  const fakeZmuxBin = join(outDir, 'fake-zmux-wait.sh');
  writeFileSync(fakeZmuxBin, [
    '#!/bin/sh',
    'if [ "$1" = "pane" ] && [ "$2" = "current" ]; then echo "{}"; exit 0; fi',
    'sleep 30',
  ].join('\n'));
  chmodSync(fakeZmuxBin, 0o755);
  const previousZmuxBin = process.env.PI_ZMUX_BIN;
  process.env.PI_ZMUX_BIN = fakeZmuxBin;
  try {
    startWatchCallback(fakePi, { id: 'cleanup-test', tab: 'bench', cwd: root, waitFor: 'DONE', timeoutSeconds: 30 });
    assert.equal(listCallbacks().length, 1, 'callback should be active before shutdown cleanup');
    const shutdownHandlers = registeredHandlers.filter((handler) => handler.event === 'session_shutdown').map((handler) => handler.handler);
    assert.ok(shutdownHandlers.length > 0, 'session_shutdown handler registered');
    for (const shutdownHandler of shutdownHandlers) await shutdownHandler({}, { cwd: root });
    assert.deepEqual(listCallbacks(), [], 'session shutdown should cancel active callbacks');
    assert.deepEqual(listRecentCallbackCompletions(), [], 'shutdown cancellation should not report cancelled callbacks as completed');
  } finally {
    if (previousZmuxBin === undefined) delete process.env.PI_ZMUX_BIN;
    else process.env.PI_ZMUX_BIN = previousZmuxBin;
    clearCallbacks();
  }
  assert.equal(autoPaneResizeAxis({ paneWidth: 80, paneHeight: 6, windowWidth: 80, windowHeight: 23 }), 'height');
  assert.equal(autoPaneResizeAxis({ paneWidth: 35, paneHeight: 23, windowWidth: 80, windowHeight: 23 }), 'width');
  assert.equal(autoPaneResizeAxis(undefined), 'width');
  assert.equal(shouldTriggerContinuation('Reload complete. Continue with verification.'), true);
  assert.equal(shouldTriggerContinuation("Reload complete. Wait for the user's next instruction."), false);
  assert.equal(shouldTriggerContinuation('reload complete; wait for user next instruction'), false);
  assert.equal(detectUserInputPrompt('[sudo] password for user:')?.kind, 'sudo_password');
  assert.equal(detectUserInputPrompt('Enter passphrase for key ~/.ssh/id_ed25519:')?.kind, 'password');
  assert.equal(detectUserInputPrompt('Are you sure you want to continue connecting (yes/no/[fingerprint])?')?.kind, 'ssh_confirm');
  assert.equal(detectUserInputPrompt('Status: inactive'), undefined);
  assert.deepEqual(statusWarnings({ turnState: 'ready', turnAt: '100' }, { targetState: 'ready', expectFreshAfter: 100 }), {
    warnings: ['matching turn state is stale; readiness is unproven'],
    failureKind: 'stale_turn_state',
  });
  assert.deepEqual(statusWarnings({}, { targetState: 'ready' }), {
    warnings: ['turn state is unavailable; readiness is unproven'],
    failureKind: 'turn_state_unavailable',
  });

  assert.deepEqual(buildPaneOpenArgs({ name: 'logs', command: 'npm run dev', cwd: '/repo', direction: 'right', size: '40%' }), [
    'pane', 'open', 'logs', '--cwd', '/repo', '-r', '40%', '--no-focus', '--', 'bash', '-lc', 'npm run dev',
  ]);
  assert.deepEqual(buildPaneOpenArgs({ name: 'logs', command: 'npm run dev', cwd: '/repo', focus: true }), [
    'pane', 'open', 'logs', '--cwd', '/repo', '-r', '--', 'bash', '-lc', 'npm run dev',
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
  assert.deepEqual(buildTabStateArgs({ action: 'ready', tab: 'worker', msg: 'checkpoint ready', session: 'zws/repo' }), [
    'tab', 'state', 'ready', 'worker', '--msg', 'checkpoint ready', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildTabPeerArgs({ action: 'running', tab: 'claude-peer', role: 'claude', hostTab: 'ztab_host', hostPane: '%9', topic: 'plan review', ttl: '30m', source: 'peer', msg: 'ready', session: 'zws/repo' }), [
    'tab', 'peer', 'running', 'claude-peer', '--role', 'claude', '--host-tab', 'ztab_host', '--host-pane', '%9', '--topic', 'plan review', '--ttl', '30m', '--source', 'peer', '--msg', 'ready', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildTabStatusArgs({ tab: 'claude-peer', session: 'zws/repo' }), [
    'tab', 'status', 'claude-peer', '--json', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildWatchArgs({ tab: 'claude-peer', session: 'zws/repo', lines: 80, waitFor: 'ready', timeoutSeconds: 8 }), [
    'watch', 'claude-peer', '-l', '80', '--until', 'ready', '-T', '8', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildWatchArgs({ tab: 'server', idleSeconds: 2 }), [
    'watch', 'server', '-l', '120', '--idle', '2', '-T', '10',
  ]);
  assert.deepEqual(buildWaitArgs({ tab: 'claude-peer', session: 'zws/repo', lines: 80, waitFor: 'ready', timeoutSeconds: 8 }), [
    'wait', 'claude-peer', '--for', 'output:ready', '-l', '80', '-T', '8', '--json', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildWaitArgs({ tab: 'claude-peer', session: 'zws/repo', turnState: 'ready', timeoutSeconds: 90 }), [
    'wait', 'claude-peer', '--for', 'turn:ready', '-l', '120', '-T', '90', '--json', '-s', 'zws/repo',
  ]);
  assert.deepEqual(buildCallbackWatchArgs({ tab: 'bench', session: 'repo/main', waitFor: 'DONE', timeoutSeconds: 60 }), [
    'wait', 'bench', '--for', 'output:DONE', '-l', '160', '-T', '60', '--json', '-s', 'repo/main',
  ]);
  assert.equal(watchPatternPresentInText('ready-service', 'tail\nready-service'), true);
  assert.equal(watchPatternPresentInText('[invalid', 'tail [invalid'), false);
  assert.deepEqual(buildTabLabelArgs({ label: 'api', target: '%42' }), ['tab', 'label', '--target', '%42', 'api']);
  assert.deepEqual(buildTabMoveArgs({ tab: 'api', destination: 'repo/sidecar', force: true }), ['tab', 'move', 'api', 'repo/sidecar', '--force']);
  assert.deepEqual(buildTabMoveArgs({ tab: 'api', destination: 'repo/sidecar', session: 'repo/main' }), ['tab', 'move', 'api', 'repo/sidecar', '-s', 'repo/main']);
  assert.deepEqual(buildTabKillArgs({ tab: 'api', session: 'repo/main' }), ['tab', 'kill', 'api', '-s', 'repo/main']);
  assert.deepEqual(buildLogArgs({ action: 'tail', tab: 'server', session: 'zws/repo', lines: 40 }), ['log', 'tail', 'server', '-n', '40', '-s', 'zws/repo']);
  assert.deepEqual(buildLogArgs({ action: 'status', tab: 'server', session: 'zws/repo', lines: 40 }), ['log', 'status']);
  assert.deepEqual(buildSnapshotArgs({ noPng: true, panes: ['%1', '%2'], lines: 120, out: '/tmp/snap', json: true }), [
    'snapshot', '--no-png', '--pane', '%1', '--pane', '%2', '--lines', '120', '--out', '/tmp/snap', '--json',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'pane', tab: 'logs', into: 'pi', direction: 'right', size: '35%', session: 'zws/repo' }), [
    'tab', 'pane', 'logs', '--session', 'zws/repo', '--into', 'pi', '--right', '--size', '35%',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'pane', tab: 'logs', into: 'pi', focus: true }), [
    'tab', 'pane', 'logs', '--into', 'pi', '--focus',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'show', pane: '%42', session: 'zws/repo', direction: 'right', size: '35%', after: true }), [
    'tab', 'show', '--session', 'zws/repo', '--pane', '%42',
  ]);
  assert.deepEqual(buildTabPlacementArgs({ action: 'show', pane: '%42', focus: true }), [
    'tab', 'show', '--pane', '%42', '--focus',
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
  let loaded = loadConfig(project, { projectTrusted: true });
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
  loaded = loadConfig(project, { projectTrusted: true });
  assert.equal(loaded.policy.mode, 'observe');
  delete process.env.PI_ZMUX_POLICY;

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

  // Invalid JSON must fall back to enforce — env was cleared above, so this
  // exercises the real fallback rather than the env override.
  const malformed = mkdtempSync(join(tmpdir(), 'pi-zmux-bad-'));
  mkdirSync(join(malformed, '.pi'));
  writeFileSync(join(malformed, '.pi/zmux.json'), '{not json');
  loaded = loadConfig(malformed, { projectTrusted: true });
  assert.equal(loaded.policy.mode, 'enforce');
  assert.equal(loaded.ignoredReason, 'invalid-json');
  assert.deepEqual(loaded.runtimes, {});

  console.log('pi-zmux extension tests passed');
} finally {
  rmSync(outDir, { recursive: true, force: true });
}
