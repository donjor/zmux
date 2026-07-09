import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';

const root = new URL('..', import.meta.url).pathname.replace(/\/$/, '');
const repoRoot = resolve(root, '..');
const currentRoot = join(repoRoot, 'pi-extension');
const outDir = mkdtempSync(join(tmpdir(), 'pi-zmux-lite-test-'));
const liteOut = join(outDir, 'lite');
const currentOut = join(outDir, 'current');

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

function fakePi() {
  const registeredTools = [];
  const registeredCommands = [];
  const registeredHandlers = [];
  return {
    registeredTools,
    registeredCommands,
    registeredHandlers,
    api: {
      registerTool(tool) { registeredTools.push(tool); },
      registerCommand(name, options) { registeredCommands.push({ name, options }); },
      on(event, handler) { registeredHandlers.push({ event, handler }); },
      sendMessage() {},
      sendUserMessage() {},
    },
  };
}

try {
  const liteTsc = join(root, 'node_modules/.bin/tsc');
  const currentTsc = join(currentRoot, 'node_modules/.bin/tsc');
  run(liteTsc, ['-p', join(root, 'tsconfig.json'), '--outDir', liteOut, '--noEmit', 'false']);
  run(currentTsc, ['-p', join(currentRoot, 'tsconfig.json'), '--outDir', currentOut, '--noEmit', 'false']);
  symlinkSync(join(root, 'node_modules'), join(liteOut, 'node_modules'), 'dir');
  symlinkSync(join(currentRoot, 'node_modules'), join(currentOut, 'node_modules'), 'dir');

  const { default: registerLite } = await import(join(liteOut, 'index.js'));
  const { LITE_OPERATIONS } = await import(join(liteOut, 'src/dispatcher.js'));
  const { default: registerCurrent } = await import(join(currentOut, 'src/index.js'));

  const lite = fakePi();
  registerLite(lite.api);
  assert.deepEqual(lite.registeredTools.map((tool) => tool.name), ['zmux_lite'], 'lite profile must expose exactly one tool');
  assert.ok(lite.registeredHandlers.some((handler) => handler.event === 'session_shutdown'), 'lite profile must clean callback children on shutdown');

  const current = fakePi();
  registerCurrent(current.api);
  assert.equal(current.registeredTools.length, 37, 'current pi-zmux baseline tool count drifted');

  const liteTokens = lite.registeredTools.reduce((sum, tool) => sum + toolTokenEstimate(tool), 0);
  const currentTokens = current.registeredTools.reduce((sum, tool) => sum + toolTokenEstimate(tool), 0);
  assert.ok(liteTokens <= 1200, `lite schema estimate should stay near the 1k target, got ${liteTokens}`);
  assert.ok(liteTokens < currentTokens / 3, `lite schema should be materially smaller than current (${liteTokens} vs ${currentTokens})`);

  const liteTool = lite.registeredTools[0];
  assert.match(liteTool.description, /one dispatcher/i);
  assert.match(liteTool.promptGuidelines.join('\n'), /do not start duplicate/i);
  assert.ok(liteTool.parameters.properties.operation.description.includes('runtime_ensure'));

  const scenariosPath = join(root, 'scenarios/zmux-lite-scenarios.json');
  const scenarios = JSON.parse(readFileSync(scenariosPath, 'utf8'));
  assert.equal(scenarios.schema, 'donjor.zmux-lite-scenarios.v1');
  assert.ok(Array.isArray(scenarios.scenarios));
  assert.ok(scenarios.scenarios.length >= 12, 'scenario suite should cover natural and adversarial prompts');
  const ids = new Set();
  let natural = 0;
  let adversarial = 0;
  for (const scenario of scenarios.scenarios) {
    assert.match(scenario.id, /^[NA]-\d{3}-[a-z0-9-]+$/);
    assert.equal(ids.has(scenario.id), false, `duplicate scenario id ${scenario.id}`);
    ids.add(scenario.id);
    assert.equal(typeof scenario.prompt, 'string');
    assert.equal(typeof scenario.expectedBehavior, 'string');
    assert.ok(LITE_OPERATIONS.includes(scenario.expectedLiteOperation), `${scenario.id} expects unknown lite operation`);
    if (scenario.kind === 'natural') {
      natural++;
      assert.doesNotMatch(scenario.prompt, /\bzmux_[a-z0-9_]+\b/u, `${scenario.id} prompt should not name Pi tools`);
    } else if (scenario.kind === 'adversarial') {
      adversarial++;
    } else {
      assert.fail(`${scenario.id} has invalid kind ${scenario.kind}`);
    }
  }
  assert.ok(natural >= 8, `expected at least 8 natural scenarios, got ${natural}`);
  assert.ok(adversarial >= 4, `expected at least 4 adversarial scenarios, got ${adversarial}`);

  console.log(`pi-zmux-lite tests passed: currentTools=${current.registeredTools.length} currentTokens≈${currentTokens} liteTools=1 liteTokens≈${liteTokens}`);
} finally {
  rmSync(outDir, { recursive: true, force: true });
}
