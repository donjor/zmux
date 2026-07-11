import assert from 'node:assert/strict';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { spawnSync } from 'node:child_process';

const root = new URL('..', import.meta.url).pathname.replace(/\/$/, '');
const outDir = mkdtempSync(join(tmpdir(), 'pi-zmux-dispatcher-test-'));
const compiledOut = join(outDir, 'compiled');

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: 'inherit', ...options });
  assert.equal(result.status, 0, `${command} ${args.join(' ')} failed`);
}

function toolTokenEstimate(tool) {
  const promptShape = {
    name: tool.name,
    description: tool.description,
    promptSnippet: tool.promptSnippet,
    promptGuidelines: tool.promptGuidelines,
    parameters: tool.parameters,
  };
  return Math.ceil(JSON.stringify(promptShape).length / 4);
}

function findNodeModules(start, packageName) {
  let dir = start;
  while (dir !== dirname(dir)) {
    const candidate = join(dir, 'node_modules');
    if (existsSync(join(candidate, packageName))) return candidate;
    dir = dirname(dir);
  }
  throw new Error(`Could not find node_modules containing ${packageName} from ${start}`);
}

function fakePi() {
  const registeredTools = [];
  const registeredCommands = [];
  const registeredHandlers = [];
  const sentMessages = [];
  return {
    registeredTools,
    registeredCommands,
    registeredHandlers,
    sentMessages,
    api: {
      registerTool(tool) { registeredTools.push(tool); },
      registerCommand(name, options) { registeredCommands.push({ name, options }); },
      on(event, handler) { registeredHandlers.push({ event, handler }); },
      sendMessage(message, options) { sentMessages.push({ message, options }); },
      sendUserMessage() {},
    },
  };
}

function toolText(result) {
  return result.content.map((item) => item.text).join('\n');
}

function nonDisplayDetails(details) {
  const { display: _display, ...rest } = details;
  return rest;
}

async function waitFor(predicate, message, timeoutMs = 2_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 10));
  }
  assert.fail(message);
}

function createCommandRecorder(directory) {
  const path = join(directory, 'command-recorder.mjs');
  const logPath = join(directory, 'commands.jsonl');
  writeFileSync(path, `#!/usr/bin/env node
import { appendFileSync, existsSync, writeFileSync } from 'node:fs';
const args = process.argv.slice(2);
const typedMarker = process.env.PI_ZMUX_TEST_LOG + '.typed';
if (args[0] === 'type') writeFileSync(typedMarker, 'typed');
appendFileSync(process.env.PI_ZMUX_TEST_LOG, JSON.stringify({ args, cwd: process.cwd() }) + '\\n');
if (args.includes('display-message') && process.env.PI_ZMUX_TEST_PANE_DIMENSIONS) {
  console.log(process.env.PI_ZMUX_TEST_PANE_DIMENSIONS);
} else if (args[0] === 'pane' && args[1] === 'current' && process.env.PI_ZMUX_TEST_CURRENT_PANE) {
  console.log(process.env.PI_ZMUX_TEST_CURRENT_PANE);
} else if (args[0] === 'tabs' && process.env.PI_ZMUX_TEST_TABS_OUTPUT) {
  console.log(process.env.PI_ZMUX_TEST_TABS_OUTPUT);
} else if (args[0] === 'version' && process.env.PI_ZMUX_TEST_VERSION_OUTPUT) {
  console.log(process.env.PI_ZMUX_TEST_VERSION_OUTPUT);
} else if (args[0] === 'tab' && args[1] === 'status') {
  if (process.env.PI_ZMUX_TEST_FAIL_STATUS === '1') process.exit(1);
  const status = existsSync(typedMarker) ? process.env.PI_ZMUX_TEST_STATUS_AFTER : process.env.PI_ZMUX_TEST_STATUS_BEFORE;
  if (status) console.log(status);
} else if (args[0] === 'run' && process.env.PI_ZMUX_TEST_RUN_OUTPUT) {
  console.log(process.env.PI_ZMUX_TEST_RUN_OUTPUT);
} else if (args[0] === 'watch' && (process.env.PI_ZMUX_TEST_WATCH_OUTPUT || process.env.PI_ZMUX_TEST_WATCH_BEFORE || process.env.PI_ZMUX_TEST_WATCH_AFTER)) {
  console.log(existsSync(typedMarker) ? (process.env.PI_ZMUX_TEST_WATCH_AFTER || process.env.PI_ZMUX_TEST_WATCH_OUTPUT) : (process.env.PI_ZMUX_TEST_WATCH_BEFORE || process.env.PI_ZMUX_TEST_WATCH_OUTPUT));
} else if (process.env.PI_ZMUX_TEST_HOLD === '1' && args[0] === 'wait') {
  process.on('SIGTERM', () => process.exit(0));
  setInterval(() => {}, 1_000);
} else {
  console.log(args.join(' '));
}
`);
  chmodSync(path, 0o755);
  return { path, logPath };
}

function readCommandLog(path) {
  if (!existsSync(path)) return [];
  return readFileSync(path, 'utf8').trim().split('\n').filter(Boolean).map((line) => JSON.parse(line));
}

async function executeDispatcher(tool, params, cwd = '/repo', projectTrusted = true) {
  return tool.execute('dispatcher-contract', params, undefined, undefined, { cwd, isProjectTrusted: () => projectTrusted });
}

async function validateDispatcherContract(extension, dispatcherTool, testDirectory, dispatcherOperations, lifecycle) {
  const recorder = createCommandRecorder(testDirectory);
  const previousBin = process.env.PI_ZMUX_BIN;
  const previousLog = process.env.PI_ZMUX_TEST_LOG;
  const previousHold = process.env.PI_ZMUX_TEST_HOLD;
  const previousRunOutput = process.env.PI_ZMUX_TEST_RUN_OUTPUT;
  const previousSocket = process.env.PI_ZMUX_TMUX_SOCKET;
  const previousPath = process.env.PATH;
  const fakeBin = join(testDirectory, 'bin');
  const contextCwd = join(testDirectory, 'repo');
  const commandCwd = join(testDirectory, 'command-cwd');
  const paneCwd = join(testDirectory, 'pane-cwd');
  const respawnCwd = join(testDirectory, 'respawn-cwd');
  spawnSync('mkdir', ['-p', fakeBin, contextCwd, commandCwd, paneCwd, respawnCwd]);
  symlinkSync(recorder.path, join(fakeBin, 'tmux'));
  const execute = (params) => executeDispatcher(dispatcherTool, params, contextCwd);
  process.env.PI_ZMUX_BIN = recorder.path;
  process.env.PI_ZMUX_TEST_LOG = recorder.logPath;
  process.env.PI_ZMUX_TMUX_SOCKET = 'extension-test';
  process.env.PATH = `${fakeBin}:${previousPath}`;

  const cases = [
    ['current', { operation: 'current' }, ['pane', 'current', '--json']],
    ['tabs', { operation: 'tabs', options: { session: 's1' } }, ['tabs', '-s', 's1']],
    ['sessions', { operation: 'sessions', target: 'workspace', options: { flat: true } }, ['ls', '-s', 'workspace']],
    ['panes', { operation: 'panes', options: { session: 's1' } }, ['pane', 'list', '--session', '--target', 's1']],
    ['run', { operation: 'run', target: 'job', command: 'echo hi', options: { session: 's1', timeoutSeconds: 5, lines: 20, detach: true, keep: true, scope: 'task' } }, ['run', '--command', 'echo hi', '-n', 'job', '-s', 's1', '-T', '5', '--lines', '20', '-d', '--keep', '--scope', 'task']],
    ['session_run', { operation: 'session_run', target: 'session-a', command: 'worker', cwd: commandCwd, options: { tab: 'main', workspace: 'ws' } }, ['session', 'run', 'session-a', '-n', 'main', '--workspace', 'ws', '--cwd', commandCwd, '--', 'bash', '-lc', 'worker']],
    ['session_kill', { operation: 'session_kill', target: 'session-a' }, ['session', 'kill', 'session-a']],
    ['runtime_ensure', { operation: 'runtime_ensure', target: 'server', command: 'npm run dev', options: { kind: 'server', session: 's1' } }, ['run', '--command', 'npm run dev', '-n', 'server', '-d', '--keep', '--scope', 'server', '-s', 's1']],
    ['runtime_logs', { operation: 'runtime_logs', target: 'server', options: { lines: 80, waitFor: 'ready', timeoutSeconds: 9, session: 's1' } }, ['watch', 'server', '-l', '80', '--until', 'ready', '-T', '9', '-s', 's1']],
    ['runtime_stop', { operation: 'runtime_stop', target: 'server', options: { session: 's1' } }, ['send', 'server', 'C-c', '-s', 's1']],
    ['tab_state', { operation: 'tab_state', target: 'work', options: { state: 'attention', rawTarget: '%1', source: 'test', message: 'review', ifState: 'running', byVisibility: true, session: 's1' } }, ['tab', 'state', 'attention', 'work', '--target', '%1', '--source', 'test', '--msg', 'review', '--if-state', 'running', '--by-visibility', '-s', 's1']],
    ['tab_peer', { operation: 'tab_peer', target: 'peer', options: { action: 'ready', role: 'codex', hostTab: 'host', hostPane: '%2', topic: 'audit', ttl: '30m', source: 'test', message: 'done', session: 's1' } }, ['tab', 'peer', 'ready', 'peer', '--role', 'codex', '--host-tab', 'host', '--host-pane', '%2', '--topic', 'audit', '--ttl', '30m', '--source', 'test', '--msg', 'done', '-s', 's1']],
    ['tab_status', { operation: 'tab_status', target: 'work', options: { session: 's1' } }, ['tab', 'status', 'work', '--json', '-s', 's1']],
    ['tab_inspect', { operation: 'tab_inspect', target: 'work', options: { lines: 42, session: 's1' } }, ['tab', 'inspect', 'work', '--lines', '42', '-s', 's1']],
    ['tab_label', { operation: 'tab_label', target: 'release', options: { rawTarget: '%3', clear: true } }, ['tab', 'label', '--target', '%3', '--clear', 'release']],
    ['tab_move', { operation: 'tab_move', target: 'work', options: { destination: 'session-b', force: true, session: 's1' } }, ['tab', 'move', 'work', 'session-b', '--force', '-s', 's1']],
    ['tab_place', { operation: 'tab_place', target: 'child', options: { action: 'pane', session: 's1', into: 'host', direction: 'right', size: '40%', focus: true } }, ['tab', 'pane', 'child', '--session', 's1', '--into', 'host', '--right', '--size', '40%', '--focus']],
    ['tab_kill', { operation: 'tab_kill', target: 'work', options: { session: 's1' } }, ['tab', 'kill', 'work', '-s', 's1']],
    ['tab_focus', { operation: 'tab_focus', target: 'work' }, ['tab', 'focus', 'work']],
    ['send_keys', { operation: 'send_keys', target: 'work', options: { keys: ['C-c', 'Enter'], session: 's1' } }, ['send', 'work', 'C-c', 'Enter', '-s', 's1']],
    ['type_text', { operation: 'type_text', target: 'peer', options: { text: 'review this', session: 's1', markPeerRunning: true, waitForTurnState: 'ready', timeoutSeconds: 8, lines: 60, source: 'test', message: 'review' } }, ['type', 'peer', 'review this', '-s', 's1', '--mark-peer-running', '--wait-turn', 'ready', '-T', '8', '--lines', '60', '--source', 'test', '--msg', 'review']],
    ['peer_ensure', { operation: 'peer_ensure', target: 'peer', command: 'pi', options: { session: 's1', role: 'pi', hostTab: 'host', hostPane: '%5', topic: 'review', source: 'test', message: 'ready', readiness: 'prompt', waitForTurnState: 'ready', timeoutSeconds: 7, lines: 70, restart: true } }, ['tab', 'peer', 'ensure', 'peer', '--command', 'pi', '-s', 's1', '--role', 'pi', '--host-tab', 'host', '--host-pane', '%5', '--topic', 'review', '--source', 'test', '--msg', 'ready', '--readiness', 'prompt', '--wait-turn', 'ready', '-T', '7', '--lines', '70', '--restart']],
    ['pane_open', { operation: 'pane_open', target: 'sidecar', command: 'htop', cwd: paneCwd, options: { rawTarget: '%6', direction: 'left', size: '35%', labelTab: true, focus: false } }, ['pane', 'open', 'sidecar', '--cwd', paneCwd, '--target', '%6', '-l', '35%', '--label-tab', '--no-focus', '--', 'bash', '-lc', 'htop']],
    ['pane_close', { operation: 'pane_close', target: '%6' }, ['pane', 'close', '%6']],
    ['pane_resize', { operation: 'pane_resize', target: '%6', options: { size: '40%', axis: 'height' } }, ['pane', 'resize', '%6', '--height', '40%']],
    ['pane_focus', { operation: 'pane_focus', target: '%6' }, ['pane', 'focus', '%6']],
    ['log', { operation: 'log', target: 'work', options: { action: 'start', ansi: true, maxBytes: 4096, session: 's1' } }, ['log', 'start', 'work', '--ansi', '--max-bytes', '4096', '-s', 's1']],
    ['snapshot', { operation: 'snapshot', options: { noPng: true, panes: ['%1', '%2'], lines: 120, out: '/tmp/evidence', json: true } }, ['snapshot', '--no-png', '--pane', '%1', '--pane', '%2', '--lines', '120', '--out', '/tmp/evidence', '--json']],
    ['wait', { operation: 'wait', target: 'work', options: { waitFor: 'DONE', lines: 50, timeoutSeconds: 11, session: 's1' } }, ['wait', 'work', '--for', 'output:DONE', '-l', '50', '-T', '11', '--json', '-s', 's1']],
    ['terminal_current', { operation: 'terminal_current' }, ['terminal', 'current', '--json']],
    ['zmux_reload', { operation: 'zmux_reload' }, ['reload']],
  ];

  try {
    for (const [name, params, expectedArgs] of cases) {
      writeFileSync(recorder.logPath, '');
      const result = await execute(params);
      assert.deepEqual(result.details.args, expectedArgs, `${name} dispatcher mapping drifted`);
      assert.equal(result.details.cwd, params.cwd ?? contextCwd, `${name} cwd drifted`);
      assert.equal(result.details.display.operation, name, `${name} display metadata drifted`);
      assert.equal(result.details.display.raw.cwd, params.cwd ?? contextCwd, `${name} display cwd drifted`);
      assert.deepEqual(result.details.display.raw.args, expectedArgs, `${name} expanded argv metadata drifted`);
      const commands = readCommandLog(recorder.logPath);
      assert.deepEqual(commands, [{ args: expectedArgs, cwd: params.cwd ?? contextCwd }], `${name} process contract drifted`);
    }

    writeFileSync(recorder.logPath, '');
    process.env.PI_ZMUX_TEST_RUN_OUTPUT = 'ready localhost:43123';
    const immediateReadiness = await execute({ operation: 'runtime_ensure', target: 'server', command: 'npm run dev', options: { readiness: 'ready|localhost', timeoutSeconds: 4 } });
    assert.equal(immediateReadiness.details.ready, true);
    assert.equal(immediateReadiness.details.readinessBasis, 'initial-output');
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['run', '--command', 'npm run dev', '-n', 'server', '-d', '--keep', '--scope', 'daemon'],
    ]);
    delete process.env.PI_ZMUX_TEST_RUN_OUTPUT;

    writeFileSync(recorder.logPath, '');
    process.env.PI_ZMUX_TEST_WATCH_OUTPUT = 'ready localhost:43124';
    const delayedReadiness = await execute({ operation: 'runtime_ensure', target: 'server', command: 'npm run dev', options: { readiness: 'ready|localhost', timeoutSeconds: 4 } });
    assert.equal(delayedReadiness.details.ready, true);
    assert.equal(delayedReadiness.details.readinessBasis, 'fresh-watch');
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['run', '--command', 'npm run dev', '-n', 'server', '-d', '--keep', '--scope', 'daemon'],
      ['watch', 'server', '-l', '120', '--until', 'ready|localhost', '-T', '4'],
    ]);
    delete process.env.PI_ZMUX_TEST_WATCH_OUTPUT;

    spawnSync('mkdir', ['-p', join(contextCwd, '.pi')]);
    writeFileSync(join(contextCwd, '.pi/zmux.json'), JSON.stringify({
      runtimes: {
        configured: { command: 'npm run configured', tab: 'configured-tab', cwd: commandCwd, readiness: 'READY', timeoutSeconds: 12, session: 's2', kind: 'worker' },
      },
    }));
    writeFileSync(recorder.logPath, '');
    const configured = await execute({ operation: 'runtime_ensure', target: 'configured', options: { restart: true } });
    assert.equal(configured.details.configPath, join(contextCwd, '.pi/zmux.json'));
    assert.equal(configured.details.runtimeName, 'configured');
    assert.equal(configured.details.ready, true);
    assert.deepEqual(readCommandLog(recorder.logPath), [
      { args: ['send', 'configured-tab', 'C-c', '-s', 's2'], cwd: commandCwd },
      { args: ['run', '--command', 'npm run configured', '-n', 'configured-tab', '-d', '--keep', '--scope', 'worker', '-s', 's2'], cwd: commandCwd },
      { args: ['watch', 'configured-tab', '-l', '120', '--until', 'READY', '-T', '12', '-s', 's2'], cwd: commandCwd },
    ]);

    const untrusted = await executeDispatcher(dispatcherTool, { operation: 'runtime_ensure', target: 'configured' }, contextCwd, false);
    assert.match(toolText(untrusted), /runtime configured has no command/);
    assert.equal(untrusted.details.ignoredReason, 'project-untrusted');

    writeFileSync(recorder.logPath, '');
    const guarded = await execute({ operation: 'runtime_ensure', target: 'unsafe', command: 'pi -p "review"' });
    assert.match(toolText(guarded), /Do not launch agent peers with -p\/--print/);
    assert.deepEqual(nonDisplayDetails(guarded.details), { command: 'pi -p "review"', failed: true, failureKind: 'headless_agent_print_mode' });
    assert.deepEqual(readCommandLog(recorder.logPath), []);

    writeFileSync(recorder.logPath, '');
    const encodedRemoteMutation = Buffer.from("Set-Content /etc/example.env 'TOKEN=redacted'", 'utf16le').toString('base64');
    const remoteRun = await execute({ operation: 'run', target: 'remote-node2', command: `ssh node-a "remote-admin -EncodedCommand ${encodedRemoteMutation}"` });
    assert.equal(remoteRun.details.recommendedTab, 'remote-node');
    assert.equal(remoteRun.details.remoteHost, 'node-a');
    assert.match(remoteRun.details.decodedRemoteCommandPreview, /Set-Content/);
    assert.match(toolText(remoteRun), /numbered remote tab sprawl/);
    assert.match(toolText(remoteRun), /about to change remote host node-a/);

    writeFileSync(recorder.logPath, '');
    const interactive = await execute({ operation: 'interactive_type', target: 'admin', command: 'ssh prod', options: { session: 's1', focus: false } });
    assert.deepEqual(nonDisplayDetails(interactive.details), { tab: 'admin', command: 'ssh prod', waitForExit: false, focus: false, session: 's1' });
    assert.match(toolText(interactive), /without changing focus/);
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['tab', 'status', 'admin', '--json', '-s', 's1'],
      ['type', 'admin', 'ssh prod', '-s', 's1'],
    ]);

    writeFileSync(recorder.logPath, '');
    rmSync(`${recorder.logPath}.typed`, { force: true });
    process.env.PI_ZMUX_TEST_STATUS_BEFORE = JSON.stringify({ cmdSeq: '7', cmdState: 'done', lastExit: '0' });
    process.env.PI_ZMUX_TEST_STATUS_AFTER = JSON.stringify({ cmdSeq: '8', cmdState: 'done', lastExit: '0', command: 'sudo true' });
    process.env.PI_ZMUX_TEST_WATCH_OUTPUT = 'command output';
    const interactiveWait = await execute({ operation: 'interactive_type', target: 'admin', command: 'sudo true', options: { timeoutSeconds: 1, lines: 40 } });
    assert.equal(interactiveWait.details.completed, true);
    assert.equal(interactiveWait.details.exitCode, 0);
    assert.equal(interactiveWait.details.cmdSeq, 8);
    assert.equal(interactiveWait.details.cmdState, 'done');
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['tab', 'status', 'admin', '--json'],
      ['watch', 'admin', '-l', '40', '-T', '10'],
      ['tab', 'status', 'admin', '--json'],
      ['type', 'admin', 'sudo true'],
      ['watch', 'admin', '-l', '40', '-T', '10'],
      ['tab', 'status', 'admin', '--json'],
    ]);
    delete process.env.PI_ZMUX_TEST_STATUS_BEFORE;
    delete process.env.PI_ZMUX_TEST_STATUS_AFTER;
    delete process.env.PI_ZMUX_TEST_WATCH_OUTPUT;
    rmSync(`${recorder.logPath}.typed`, { force: true });

    process.env.PI_ZMUX_TEST_STATUS_BEFORE = JSON.stringify({});
    process.env.PI_ZMUX_TEST_STATUS_AFTER = JSON.stringify({});
    process.env.PI_ZMUX_TEST_WATCH_BEFORE = 'shell prompt';
    process.env.PI_ZMUX_TEST_WATCH_AFTER = 'shell prompt\nsudo: a password is required\n[ble: exit 1]\nshell prompt';
    const interactiveFallback = await execute({ operation: 'interactive_type', target: 'admin', command: 'sudo -n true', options: { waitForExit: true, timeoutSeconds: 0.01, lines: 40 } });
    assert.equal(interactiveFallback.details.completed, true);
    assert.equal(interactiveFallback.details.exitCode, 1);
    assert.equal(interactiveFallback.details.evidenceBasis, 'shell-output-exit-marker');
    assert.match(toolText(interactiveFallback), /sudo: a password is required/);
    delete process.env.PI_ZMUX_TEST_STATUS_BEFORE;
    delete process.env.PI_ZMUX_TEST_STATUS_AFTER;
    delete process.env.PI_ZMUX_TEST_WATCH_BEFORE;
    delete process.env.PI_ZMUX_TEST_WATCH_AFTER;
    rmSync(`${recorder.logPath}.typed`, { force: true });

    writeFileSync(recorder.logPath, '');
    process.env.PI_ZMUX_TEST_FAIL_STATUS = '1';
    const interactiveCreate = await execute({ operation: 'interactive_type', target: 'new-admin', command: 'ssh prod', options: { focus: true } });
    assert.equal(interactiveCreate.details.focus, true);
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['tab', 'status', 'new-admin', '--json'],
      ['run', '--command', 'exec bash -l', '-n', 'new-admin', '-d'],
      ['tab', 'focus', 'new-admin'],
      ['type', 'new-admin', 'ssh prod'],
    ]);
    delete process.env.PI_ZMUX_TEST_FAIL_STATUS;

    writeFileSync(recorder.logPath, '');
    process.env.PI_ZMUX_TEST_PANE_DIMENSIONS = '80 6 80 23';
    const paneResizeAuto = await execute({ operation: 'pane_resize', target: '%7', options: { size: '40%', axis: 'auto' } });
    assert.equal(paneResizeAuto.details.axis, 'height');
    assert.match(toolText(paneResizeAuto), /height to 40%/);
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['-L', 'extension-test', 'display-message', '-p', '-t', '%7', '#{pane_width} #{pane_height} #{window_width} #{window_height}'],
      ['pane', 'resize', '%7', '--height', '40%'],
    ]);
    delete process.env.PI_ZMUX_TEST_PANE_DIMENSIONS;

    writeFileSync(recorder.logPath, '');
    const paneKeys = await execute({ operation: 'pane_send_keys', target: '%7', options: { keys: ['C-c', 'Enter'], timeoutSeconds: 3 } });
    assert.deepEqual(nonDisplayDetails(paneKeys.details), { pane: '%7', keys: ['C-c', 'Enter'] });
    assert.deepEqual(readCommandLog(recorder.logPath), [{ args: ['-L', 'extension-test', 'send-keys', '-t', '%7', 'C-c', 'Enter'], cwd: contextCwd }]);

    writeFileSync(recorder.logPath, '');
    const paneType = await execute({ operation: 'pane_type', target: '%7', options: { text: 'echo hi' } });
    assert.match(toolText(paneType), /typed text into pane %7/);
    assert.deepEqual(readCommandLog(recorder.logPath).map((entry) => entry.args), [
      ['-L', 'extension-test', 'send-keys', '-t', '%7', '-l', 'echo hi'],
      ['-L', 'extension-test', 'send-keys', '-t', '%7', 'Enter'],
    ]);

    const reloadScript = lifecycle.buildPiReloadScript({ cwd: contextCwd, pane: '%8', delayMs: 0, retryAttempts: 2, retryDelayMs: 1_500 });
    assert.match(reloadScript, /capture-pane/);
    assert.match(reloadScript, /while \[ "\$attempt" -le 2 \]/);
    assert.match(reloadScript, /send-keys.*%8.*\/reload.*Enter/);
    assert.equal(spawnSync('bash', ['-n'], { input: reloadScript }).status, 0, 'reload retry script must parse as bash');
    const reload = await execute({ operation: 'pi_reload', target: '%8', options: { delayMs: 0, retryAttempts: 1, retryDelayMs: 0, continuationPrompt: 'continue reload smoke' } });
    assert.match(toolText(reload), /scheduled Pi \/reload for %8/);
    assert.equal(reload.details.method, 'tmux send-keys /reload Enter with warning retry');
    assert.equal(reload.details.retryAttempts, 1);
    assert.ok(existsSync(reload.details.continuationPath));
    assert.doesNotMatch(JSON.stringify(reload.details), /reload-helper|--keep/);
    const sessionStart = extension.registeredHandlers.find((handler) => handler.event === 'session_start');
    assert.ok(sessionStart, 'session_start continuation handler registered');
    await sessionStart.handler({}, { cwd: contextCwd, ui: { notify() {} } });
    assert.ok(extension.sentMessages.some(({ message }) => message.customType === 'pi-zmux-reload-continuation' && message.content === 'continue reload smoke'));

    const respawnScript = lifecycle.buildPiRespawnScript({ cwd: respawnCwd, pane: '%9', command: 'pi -c', delayMs: 0 });
    assert.match(respawnScript, /respawn-pane.*%9.*pi -c/);
    assert.equal(spawnSync('bash', ['-n'], { input: respawnScript }).status, 0, 'respawn script must parse as bash');
    const respawn = await execute({ operation: 'pi_respawn', target: '%9', cwd: respawnCwd, options: { delayMs: 0, continuationPrompt: 'continue respawn smoke' } });
    assert.match(toolText(respawn), /scheduled Pi pane respawn for %9 using pi -c/);
    assert.equal(respawn.details.method, 'tmux respawn-pane -k');
    assert.ok(existsSync(respawn.details.continuationPath));
    assert.ok(existsSync(respawn.details.continuationHandoff));
    await sessionStart.handler({}, { cwd: respawnCwd, ui: { notify() {} } });
    assert.ok(extension.sentMessages.some(({ message }) => message.customType === 'pi-zmux-respawn-continuation' && message.content === 'continue respawn smoke'));
    await assert.rejects(() => execute({ operation: 'pi_respawn', target: '%9', command: 'pi -c', options: { continuationPrompt: 'invalid combination' } }), /cannot be combined/);

    process.env.PI_ZMUX_TEST_CURRENT_PANE = JSON.stringify({ ID: '%10' });
    const resolvedRespawn = await execute({ operation: 'pi_respawn', options: { delayMs: 0 } });
    assert.equal(resolvedRespawn.details.pane, '%10');
    delete process.env.PI_ZMUX_TEST_CURRENT_PANE;
    const guardedRespawn = await execute({ operation: 'pi_respawn', command: 'codex --print "review"' });
    assert.equal(guardedRespawn.details.failureKind, 'headless_agent_print_mode');

    writeFileSync(recorder.logPath, '');
    process.env.PI_ZMUX_TEST_HOLD = '1';
    const peerHandoff = await execute({ operation: 'peer_handoff', target: 'peer', options: { id: 'peer-handoff-test', text: 'check branch', waitFor: 'PEER_RESPONSE_OK', lines: 30, timeoutSeconds: 7, markPeerRunning: true, source: 'test', message: 'branch check' } });
    assert.equal(peerHandoff.details.id, 'peer-handoff-test');
    assert.deepEqual(peerHandoff.details.args, ['wait', 'peer', '--for', 'output:PEER_RESPONSE_OK', '-l', '30', '-T', '7', '--json']);
    assert.deepEqual(peerHandoff.details.typeArgs, ['type', 'peer', 'check branch', '--mark-peer-running', '--source', 'test', '--msg', 'branch check']);
    const peerHandoffCommands = readCommandLog(recorder.logPath).map((entry) => JSON.stringify(entry.args));
    assert.ok(peerHandoffCommands.includes(JSON.stringify(peerHandoff.details.args)));
    assert.ok(peerHandoffCommands.includes(JSON.stringify(peerHandoff.details.typeArgs)));
    await execute({ operation: 'callback_cancel', target: 'peer-handoff-test' });
    delete process.env.PI_ZMUX_TEST_HOLD;
    await assert.rejects(
      () => execute({ operation: 'peer_handoff', target: 'peer', options: { text: 'reply with DONE', waitFor: 'DONE' } }),
      /waitFor pattern must not match options\.text/,
    );
    await assert.rejects(
      () => execute({ operation: 'peer_handoff', target: 'peer', options: { text: 'reply with PEER_RESPONSE_OK: main', waitFor: 'PEER_RESPONSE_[O]K:' } }),
      /waitFor pattern must not match options\.text/,
    );

    writeFileSync(recorder.logPath, '');
    const callback = await execute({ operation: 'callback_watch', target: 'work', options: { id: 'callback-complete', waitFor: 'DONE', lines: 25, timeoutSeconds: 6, deliverAs: 'followUp', triggerTurn: false } });
    assert.equal(callback.details.id, 'callback-complete');
    assert.deepEqual(callback.details.args, ['wait', 'work', '--for', 'output:DONE', '-l', '25', '-T', '6', '--json']);
    await waitFor(() => extension.sentMessages.some(({ message }) => message.details?.id === 'callback-complete'), 'callback completion message was not delivered');
    const callbackMessage = extension.sentMessages.find(({ message }) => message.details?.id === 'callback-complete');
    assert.deepEqual(callbackMessage.options, { deliverAs: 'followUp', triggerTurn: false });

    process.env.PI_ZMUX_TEST_HOLD = '1';
    const cancellable = await execute({ operation: 'callback_watch', target: 'work', options: { id: 'callback-cancel', idleSeconds: 1 } });
    assert.equal(cancellable.details.id, 'callback-cancel');
    const active = await execute({ operation: 'callback_list' });
    assert.match(toolText(active), /callback-cancel/);
    await assert.rejects(() => execute({ operation: 'callback_watch', target: 'other', options: { id: 'callback-cancel', waitFor: 'DONE' } }), /callback id already exists/);
    const cancelledMessageCount = extension.sentMessages.filter(({ message }) => message.details?.id === 'callback-cancel').length;
    const cancelled = await execute({ operation: 'callback_cancel', target: 'callback-cancel' });
    assert.deepEqual(nonDisplayDetails(cancelled.details), { id: 'callback-cancel', cancelled: true });
    const empty = await execute({ operation: 'callback_list' });
    assert.doesNotMatch(toolText(empty), /callback-cancel/);
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 50));
    assert.equal(extension.sentMessages.filter(({ message }) => message.details?.id === 'callback-cancel').length, cancelledMessageCount, 'cancelled callbacks must not deliver completion messages');
    await assert.rejects(() => execute({ operation: 'callback_watch', target: 'work' }), /requires waitFor or idleSeconds/);
    await assert.rejects(() => execute({ operation: 'callback_watch', target: 'work', options: { waitFor: 'DONE', idleSeconds: 1 } }), /cannot be combined/);
    await assert.rejects(() => execute({ operation: 'callback_watch', target: 'work', options: { waitFor: 'DONE', deliverAs: 'later' } }), /deliverAs must be one of/);

    const shutdownCallback = await execute({ operation: 'callback_watch', target: 'work', options: { id: 'callback-shutdown', idleSeconds: 1 } });
    assert.equal(shutdownCallback.details.id, 'callback-shutdown');
    const shutdown = extension.registeredHandlers.find((handler) => handler.event === 'session_shutdown');
    shutdown.handler();
    const afterShutdown = await execute({ operation: 'callback_list' });
    assert.equal(toolText(afterShutdown), 'no active zmux callbacks');
    delete process.env.PI_ZMUX_TEST_HOLD;

    const noFocus = await execute({ operation: 'tab_place', target: 'child', options: { action: 'pane', into: 'host', focus: false } });
    assert.doesNotMatch(noFocus.details.args.join(' '), /--focus/);
    const paneNoFocus = await execute({ operation: 'pane_open', target: 'sidecar', command: 'htop' });
    assert.ok(paneNoFocus.details.args.includes('--no-focus'));

    await assert.rejects(() => execute({ operation: 'unknown' }), /unknown zmux operation/);
    await assert.rejects(() => execute({ operation: 'run' }), /command is required/);
    await assert.rejects(() => execute({ operation: 'tab_kill' }), /tab is required/);
    await assert.rejects(() => execute({ operation: 'send_keys', target: 'work', options: { keys: 'C-c' } }), /must be an array of strings/);
    await assert.rejects(() => execute({ operation: 'tabs', options: { session: 42 } }), /must be a string/);
    await assert.rejects(() => execute({ operation: 'run', command: 'true', options: { timeoutSeconds: Number.NaN } }), /must be a finite number/);
    await assert.rejects(() => execute({ operation: 'tab_place', target: 'child', options: { action: 'pane', focus: 'yes' } }), /must be a boolean/);
    await assert.rejects(() => execute({ operation: 'runtime_logs', target: 'server', options: { waitFor: 'READY', idleSeconds: 1 } }), /cannot be combined/);
    await assert.rejects(() => execute({ operation: 'wait', target: 'server' }), /requires waitFor or idleSeconds/);
    await assert.rejects(() => execute({ operation: 'wait', target: 'server', options: { waitFor: 'READY', idleSeconds: 1 } }), /cannot be combined/);
    await assert.rejects(() => execute({ operation: 'wait', target: 'server', options: { waitFor: 'output:READY' } }), /output regex only.*omit.*output:/);
    await assert.rejects(() => execute({ operation: 'callback_watch', target: 'server', options: { waitFor: 'output:READY' } }), /output regex only.*omit.*output:/);
    await assert.rejects(() => execute({ operation: 'tab_state', target: 'work', options: { state: 'mystery' } }), /state must be one of/);
    await assert.rejects(() => execute({ operation: 'tab_peer', target: 'work', options: { action: 'mystery' } }), /action must be one of/);
    await assert.rejects(() => execute({ operation: 'tab_place', target: 'work', options: { action: 'sideways' } }), /action must be one of/);
    await assert.rejects(() => execute({ operation: 'tab_place', target: 'work', options: { action: 'pane', direction: 'diagonal' } }), /direction must be one of/);
    await assert.rejects(() => execute({ operation: 'pane_open', target: 'side', command: 'true', options: { direction: 'diagonal' } }), /direction must be one of/);
    await assert.rejects(() => execute({ operation: 'log', target: 'work', options: { action: 'mystery' } }), /action must be one of/);
    await assert.rejects(() => execute({ operation: 'log', options: { action: 'start' } }), /target is required/);
    await assert.rejects(() => execute({ operation: 'log', target: 'work', options: { action: 'status' } }), /target is not valid/);
    await assert.rejects(() => execute({ operation: 'log', target: 'work', options: { action: 'tail', ansi: true } }), /ansi is not valid/);
    await assert.rejects(() => execute({ operation: 'tab_place', options: { action: 'pane', into: 'host' } }), /target is required/);
    await assert.rejects(() => execute({ operation: 'tab_place', options: { action: 'show' } }), /target or options.pane is required/);
    await assert.rejects(() => execute({ operation: 'tab_place', target: 'work', options: { action: 'full', into: 'host' } }), /options.into is not valid/);
    await assert.rejects(() => execute({ operation: 'tab_place', target: 'work', options: { action: 'hide', focus: true } }), /options.focus is not valid/);

    writeFileSync(recorder.logPath, '');
    const plainLogs = await execute({ operation: 'runtime_logs', target: 'plain' });
    assert.deepEqual(plainLogs.details.args, ['watch', 'plain', '-l', '120']);

    process.env.PI_ZMUX_BIN = '/bin/false';
    await assert.rejects(() => execute({ operation: 'current' }), /zmux current failed: pane current --json/);
  } finally {
    if (previousBin === undefined) delete process.env.PI_ZMUX_BIN; else process.env.PI_ZMUX_BIN = previousBin;
    if (previousLog === undefined) delete process.env.PI_ZMUX_TEST_LOG; else process.env.PI_ZMUX_TEST_LOG = previousLog;
    if (previousHold === undefined) delete process.env.PI_ZMUX_TEST_HOLD; else process.env.PI_ZMUX_TEST_HOLD = previousHold;
    if (previousRunOutput === undefined) delete process.env.PI_ZMUX_TEST_RUN_OUTPUT; else process.env.PI_ZMUX_TEST_RUN_OUTPUT = previousRunOutput;
    if (previousSocket === undefined) delete process.env.PI_ZMUX_TMUX_SOCKET; else process.env.PI_ZMUX_TMUX_SOCKET = previousSocket;
    if (previousPath === undefined) delete process.env.PATH; else process.env.PATH = previousPath;
  }

  assert.equal(cases.length + 9, dispatcherOperations.length, 'every dispatcher operation must have a deterministic contract test');
  return cases.length + 9;
}

try {
  const tsc = join(root, 'node_modules/.bin/tsc');
  const nodeModules = findNodeModules(root, 'typebox');
  run(tsc, ['-p', join(root, 'tsconfig.json'), '--outDir', compiledOut, '--noEmit', 'false']);
  symlinkSync(nodeModules, join(compiledOut, 'node_modules'), 'dir');

  const { default: registerExtension } = await import(join(compiledOut, 'index.js'));
  const { ZMUX_OPERATIONS } = await import(join(compiledOut, 'src/dispatcher.js'));
  const piLifecycle = await import(join(compiledOut, 'src/zmux/pi-lifecycle.js'));
  const {
    OPERATION_DESCRIPTORS,
    buildDisplayMetadata,
    formatZmuxCall,
    formatZmuxResult,
  } = await import(join(compiledOut, 'src/rendering.js'));
  const { visibleWidth } = await import('@earendil-works/pi-tui');
  const lifecycle = { ...piLifecycle, buildPiRespawnScript: piLifecycle.buildTmuxRespawnScript };

  const extension = fakePi();
  registerExtension(extension.api);
  assert.deepEqual(extension.registeredTools.map((tool) => tool.name), ['zmux'], 'production profile must expose exactly one canonical dispatcher tool');
  assert.ok(extension.registeredHandlers.some((handler) => handler.event === 'session_shutdown'), 'production profile must clean callback children on shutdown');

  const schemaTokens = extension.registeredTools.reduce((sum, tool) => sum + toolTokenEstimate(tool), 0);
  assert.ok(schemaTokens <= 1200, `dispatcher schema estimate should stay near the 1k target, got ${schemaTokens}`);

  const dispatcher = extension.registeredTools[0];
  assert.match(dispatcher.description, /canonical zmux dispatcher/i);
  const guidelines = dispatcher.promptGuidelines.join('\n');
  assert.match(guidelines, /do not start duplicate/i);
  assert.match(guidelines, /another.*copy.*before.*logs.*runtime_logs.*existing/is);
  assert.match(guidelines, /sudo.*interactive_type.*never.*run/is);
  assert.match(guidelines, /callback_watch.*waitFor.*idleSeconds.*never.*text/is);
  assert.match(guidelines, /peer_handoff.*waitFor.*never.*type_text.*callback_watch/is);
  assert.match(guidelines, /pi_reload.*omit.*target.*continuation.*proves.*completion.*terminal_current/is);
  assert.match(guidelines, /pi_reload.*pi_respawn.*continuationPrompt.*never.*deliverAs/is);
  assert.ok(dispatcher.parameters.properties.operation.description.includes('runtime_ensure'));
  assert.match(dispatcher.parameters.properties.options.description, /waitForExit/);
  assert.equal(typeof dispatcher.renderCall, 'function', 'dispatcher must provide a native call renderer');
  assert.equal(typeof dispatcher.renderResult, 'function', 'dispatcher must provide a native result renderer');

  const plainTheme = {
    fg(_color, text) { return text; },
    bold(text) { return text; },
    italic(text) { return `_${text}_`; },
  };
  assert.deepEqual(Object.keys(OPERATION_DESCRIPTORS).sort(), [...ZMUX_OPERATIONS].sort(), 'every dispatcher operation must have a renderer descriptor');
  for (const operation of ZMUX_OPERATIONS) {
    const rendered = formatZmuxCall({ operation }, false, plainTheme);
    assert.match(rendered, new RegExp(OPERATION_DESCRIPTORS[operation].verb.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `${operation} must render its operation verb`);
  }

  const typedText = 'Review the current output in full before restarting anything.';
  const typeRenderArgs = { operation: 'type_text', target: 'pi-peer', options: { session: 'main', text: typedText, waitForTurnState: 'ready', focus: false } };
  const typeCall = formatZmuxCall(typeRenderArgs, false, plainTheme);
  assert.match(typeCall, /^┫ 󱂬 zmux ┣  󰅐 type text/, 'every call heading must carry the top-level zmux identity chip');
  assert.match(typeCall, / main\n└─ 󰓩 pi-peer/, 'tab destination must render as a tree');
  assert.ok(typeCall.includes(`_${typedText}_`), 'typed text must render in italics and in full');
  assert.doesNotMatch(typeCall, new RegExp(`"${typedText}"`), 'typed text must not be quoted');
  assert.match(typeCall, /wait for ready · focus unchanged/);

  const hugeText = Array.from({ length: 22 }, (_, index) => `line ${index} ${'x'.repeat(70)}`).join('\n');
  const hugeArgs = { operation: 'peer_handoff', target: 'pi-peer', options: { session: 'main', text: hugeText } };
  const hugeCollapsed = formatZmuxCall(hugeArgs, false, plainTheme);
  assert.doesNotMatch(hugeCollapsed, new RegExp(`_${hugeText.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}_`), 'exceptionally large text may collapse');
  assert.match(hugeCollapsed, /characters · 22 lines · Ctrl\+O to show all/);
  assert.ok(formatZmuxCall(hugeArgs, true, plainTheme).includes(`_${hugeText}_`), 'expanded rendering must restore the full payload');

  const secret = 'token=do-not-render';
  const secretCall = formatZmuxCall({ operation: 'pane_type', target: '%272', options: { text: secret, sensitive: true } }, true, plainTheme);
  assert.match(secretCall, /sensitive input redacted/);
  assert.doesNotMatch(secretCall, /do-not-render/);
  assert.doesNotMatch(secretCall, /%272/, 'collapsed call UI must not expose raw pane ids');
  const secretDisplay = buildDisplayMetadata({ operation: 'pane_type', target: '%272', options: { text: secret, sensitive: true } }, '/repo', {}, { args: ['tmux-type', '%272', secret], exitCode: 0, output: 'typed' });
  const secretExpanded = formatZmuxResult({ content: [{ type: 'text', text: 'typed' }], details: { display: secretDisplay } }, { operation: 'pane_type', target: '%272', options: { text: secret, sensitive: true } }, { expanded: true, isPartial: false }, false, plainTheme);
  assert.doesNotMatch(secretExpanded, /do-not-render/, 'expanded metadata must redact sensitive argv');

  const paneParams = { operation: 'pane_resize', target: '%272', options: { size: '40%', axis: 'height' } };
  const paneDisplay = buildDisplayMetadata(paneParams, '/repo', { axis: 'height' }, { args: ['pane', 'resize', '%272', '--height', '40%'], exitCode: 0, output: '{"status":"done"}' });
  const paneResult = { content: [{ type: 'text', text: '{"status":"done"}' }], details: { display: paneDisplay } };
  const paneCollapsed = formatZmuxResult(paneResult, paneParams, { expanded: false, isPartial: false }, false, plainTheme);
  assert.match(paneCollapsed, /^┫ 󱂬 zmux ┣  ✓ resize pane done/, 'every result heading must carry the top-level zmux identity chip');
  assert.match(paneCollapsed, /󰏤 pane/);
  assert.doesNotMatch(paneCollapsed, /%272|\{"status"/, 'collapsed results must hide raw ids and JSON');
  const paneExpanded = formatZmuxResult(paneResult, paneParams, { expanded: true, isPartial: false }, false, plainTheme);
  assert.match(paneExpanded, /pane\s+%272/);
  assert.match(paneExpanded, /argv\s+pane resize %272 --height 40%/);

  const tabsParams = { operation: 'tabs', options: { session: 'main' } };
  const tabsResult = { content: [{ type: 'text', text: '1: pi ready\n2: api running' }], details: { display: buildDisplayMetadata(tabsParams, '/repo', {}, { output: '1: pi ready\n2: api running' }) } };
  const tabsRendered = formatZmuxResult(tabsResult, tabsParams, { expanded: false, isPartial: false }, false, plainTheme);
  assert.match(tabsRendered, / main\n├─ 󰓩 1: pi ready\n└─ 󰓩 2: api running/, 'tab lists must extend the destination tree');

  const renderContext = {
    args: typeRenderArgs,
    lastComponent: undefined,
    expanded: false,
    state: {},
    toolCallId: 'render-smoke',
    invalidate() {},
    cwd: '/repo',
    executionStarted: false,
    argsComplete: true,
    isPartial: false,
    showImages: false,
    isError: false,
  };
  const narrowCall = dispatcher.renderCall(typeRenderArgs, plainTheme, renderContext);
  for (const line of narrowCall.render(24)) {
    assert.ok(visibleWidth(line) <= 24, `narrow renderer overflowed: ${JSON.stringify(line)}`);
  }

  assert.equal(
    extension.registeredHandlers.filter((handler) => handler.event === 'before_agent_start').length,
    0,
    'production must not inject zmux state into every agent run',
  );

  const contextDirectory = join(outDir, 'context-command');
  const contextRecorderDirectory = join(outDir, 'context-recorder');
  mkdirSync(join(contextDirectory, '.pi'), { recursive: true });
  mkdirSync(contextRecorderDirectory, { recursive: true });
  writeFileSync(join(contextDirectory, '.pi/zmux.json'), JSON.stringify({
    runtimes: { server: { command: 'npm run dev', tab: 'server' } },
  }));
  const contextRecorder = createCommandRecorder(contextRecorderDirectory);
  const savedContextEnv = Object.fromEntries(['PI_ZMUX_BIN', 'PI_ZMUX_TEST_LOG', 'PI_ZMUX_TEST_CURRENT_PANE', 'PI_ZMUX_TEST_TABS_OUTPUT', 'PI_ZMUX_TEST_VERSION_OUTPUT'].map((name) => [name, process.env[name]]));
  try {
    process.env.PI_ZMUX_BIN = contextRecorder.path;
    process.env.PI_ZMUX_TEST_LOG = contextRecorder.logPath;
    process.env.PI_ZMUX_TEST_CURRENT_PANE = JSON.stringify({ ID: '%1', Session: 'test', WindowIndex: 1, Dir: contextDirectory });
    process.env.PI_ZMUX_TEST_TABS_OUTPUT = '1: pi ready\n2: server running';
    process.env.PI_ZMUX_TEST_VERSION_OUTPUT = 'zmux test';
    const statusCommand = extension.registeredCommands.find((command) => command.name === 'zmux');
    assert.ok(statusCommand, '/zmux diagnostic command registered');
    const notifications = [];
    await statusCommand.options.handler('status', {
      cwd: contextDirectory,
      isProjectTrusted: () => true,
      ui: { notify(message, level) { notifications.push({ message, level }); } },
    });
    assert.equal(notifications.length, 1);
    assert.match(notifications[0].message, /^pi-zmux context:/);
    assert.match(notifications[0].message, /current zmux: session=test pane=%1 tab=1/);
    assert.match(notifications[0].message, /configured runtimes:\nserver: tab=server cmd=npm run dev/);
    assert.match(notifications[0].message, /visible tabs:\n1: pi ready\n2: server running/);

    writeFileSync(join(contextDirectory, '.pi/zmux.json'), JSON.stringify({
      runtimes: Object.fromEntries(Array.from({ length: 8 }, (_, index) => [`runtime-${index}`, { command: `serve-${index} ${'x'.repeat(400)}`, tab: `tab-${index}` }])),
    }));
    process.env.PI_ZMUX_TEST_TABS_OUTPUT = Array.from({ length: 80 }, (_, index) => `${index}: tab-${index} ready with a deliberately long status label`).join('\n');
    await statusCommand.options.handler('status', {
      cwd: contextDirectory,
      isProjectTrusted: () => true,
      ui: { notify(message, level) { notifications.push({ message, level }); } },
    });
    assert.equal(notifications.length, 2);
    assert.match(notifications[1].message, /configured runtimes:[\s\S]*\[truncated\]/);
    assert.match(notifications[1].message, /visible tabs:[\s\S]*\[truncated\]/);
    assert.ok(notifications[1].message.length <= 2_200, `human /zmux status diagnostic should stay bounded, got ${notifications[1].message.length} characters`);
  } finally {
    for (const [name, value] of Object.entries(savedContextEnv)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  }

  const dispatcherContracts = await validateDispatcherContract(extension, dispatcher, outDir, ZMUX_OPERATIONS, lifecycle);
  console.log(`pi-zmux dispatcher tests passed: tools=1 schemaTokens≈${schemaTokens} automaticContextTokens=0 operations=${dispatcherContracts}`);
} finally {
  rmSync(outDir, { recursive: true, force: true });
}
