#!/usr/bin/env node
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadDoctrine } from '../../../agent-doctrine/generate.mjs';

const here = fileURLToPath(new URL('.', import.meta.url));
const root = join(here, '..', '..', '..');

for (const retired of ['skills/zmux/references/testing', 'pi-zmux/references/testing']) {
  assert.equal(existsSync(join(root, retired)), false, `retired package-local testing tree must stay removed: ${retired}`);
}

const doctrineCheck = spawnSync(process.execPath, [join(root, 'agent-doctrine/generate.mjs'), '--check'], {
  cwd: root,
  encoding: 'utf8',
});
assert.equal(
  doctrineCheck.status,
  0,
  ['agent doctrine generation/validation failed', doctrineCheck.stdout, doctrineCheck.stderr].filter(Boolean).join('\n'),
);

function read(rel) {
  return readFileSync(join(root, rel), 'utf8');
}

const dispatcherSource = read('pi-zmux/src/dispatcher.ts');
const operationsSource = read('pi-zmux/src/operations.ts');
const toolNames = new Set([...dispatcherSource.matchAll(/name:\s*"(zmux)"/g)].map((match) => match[1]));
assert.deepEqual([...toolNames], ['zmux'], 'Pi must expose one canonical dispatcher tool');
const operationBlock = /export const ZMUX_OPERATIONS = \[([\s\S]*?)\] as const;/u.exec(operationsSource)?.[1] ?? '';
const operations = new Set([...operationBlock.matchAll(/"([a-z0-9_]+)"/g)].map((match) => match[1]));
assert.equal(operations.size, 40, `expected 40 dispatcher operations, got ${operations.size}`);

const doctrineManifest = JSON.parse(read('pi-zmux/doctrine-manifest.generated.json'));
assert.equal(doctrineManifest.schema, 'zmux.doctrine-manifest.v1');
assert.deepEqual(doctrineManifest.dispatcherOperations, [...operations].sort(), 'doctrine manifest operation inventory drifted');
assert.equal(new Set(doctrineManifest.piRuleIds).size, doctrineManifest.piRuleIds.length, 'Pi doctrine manifest contains duplicate rule ids');

const { scenarios: scenarioRecords } = loadDoctrine();
// The Pi doctrine harness (agent-doctrine/harnesses/pi) and its scenario
// projection are deferred with the Pi extension; only the Claude harness ships
// on this branch, so cross-check the shared scenario inventory against it alone.
const claudeHostFlow = read('agent-doctrine/harnesses/claude/host-flow.md');
for (const scenario of scenarioRecords) {
  assert.equal(claudeHostFlow.includes(scenario.id), scenario.applicability.includes('claude'), `${scenario.id} claude host flow inventory drifted`);
}
assert.match(dispatcherSource, /import \{ SHARED_ZMUX_PROMPT_GUIDELINES \} from "\.\/generated\/doctrine\.js";/);
assert.match(dispatcherSource, /promptGuidelines:\s*\[\s*\.\.\.SHARED_ZMUX_PROMPT_GUIDELINES,/);

const skillFiles = [
  'skills/zmux/SKILL.md',
  'skills/zmux/references/run-observe.md',
  'skills/zmux/references/guard-and-tab-states.md',
  'skills/zmux/references/agent-peer.md',
  'skills/zmux/references/agent-worker.md',
  'skills/zmux/references/cli-catalog.md',
  'skills/zmux/references/shared-doctrine.generated.md',
  'docs/reference/agent-doctrine-matrix.generated.md',
  'docs/domains/pi-zmux-extension.md',
  'docs/dev/agent-grounding.md',
  'docs/dev/test-prompts/README.md',
  'docs/dev/test-prompts/zmux-agent-pi-zmux-testing-prompt.md',
  'docs/dev/test-prompts/zmux-agent-skill-testing-prompt.md',
  'agent-doctrine/harnesses/claude/README.md',
  'agent-doctrine/harnesses/claude/host-prompt.md',
  'agent-doctrine/harnesses/claude/host-flow.md',
];
const docs = Object.fromEntries(skillFiles.map((file) => [file, read(file)]));
const combined = Object.values(docs).join('\n');
// Whitespace-normalized view so doctrine assertions bind to wording, not to
// where Markdown/prose happens to line-wrap. Use this for any multi-word phrase.
const normalizeWs = (text) => text.replace(/\s+/g, ' ');
const flatDocs = Object.fromEntries(Object.entries(docs).map(([file, text]) => [file, normalizeWs(text)]));

const criticalOperations = [
  'run',
  'runtime_ensure',
  'runtime_logs',
  'interactive_type',
  'tab_status',
  'tab_peer',
  'type_text',
  'session_run',
  'pi_reload',
];
for (const name of criticalOperations) {
  assert.ok(operations.has(name), `critical dispatcher operation missing from source: ${name}`);
  assert.ok(combined.includes(name), `critical dispatcher operation missing from docs/skill doctrine: ${name}`);
}

const allowedNonToolMentions = new Set(['zmux_born', 'zmux_keep', 'zmux_label', 'zmux_tab']);
for (const [file, text] of Object.entries(docs)) {
  for (const match of text.matchAll(/\bzmux_[a-z0-9_]+\b/g)) {
    const name = match[0];
    if (allowedNonToolMentions.has(name) || operations.has(name)) continue;
    assert.ok(toolNames.has(name), `${file} mentions ${name}, but no Pi tool by that name is registered`);
  }
}

assert.ok(!combined.includes(':::AGENT_DONE'), 'skill/docs must not revive AGENT_DONE sentinel doctrine');
assert.match(docs['skills/zmux/references/run-observe.md'], /does not print completion sentinels/i);
assert.match(docs['skills/zmux/references/run-observe.md'], /Do not use `watch` as lifecycle truth/i);
assert.match(docs['skills/zmux/references/agent-peer.md'], /not the primary completion signal/i);
assert.match(docs['skills/zmux/references/agent-peer.md'], /Never start peer agents through headless\/print one-shot modes/i);
assert.ok(
  !/zmux\s+(run|tab peer ensure)[^\n]*(claude|codex|pi|agy)[^\n]*(\s-p\b|\s--print\b)/.test(combined),
  'docs must not show peer launch examples using agent -p/--print; type into visible peers instead',
);
assert.match(docs['skills/zmux/SKILL.md'], /references\/shared-doctrine\.generated\.md/);
for (const file of [
  'skills/zmux/references/run-observe.md',
  'skills/zmux/references/guard-and-tab-states.md',
  'skills/zmux/references/agent-peer.md',
  'skills/zmux/references/agent-worker.md',
]) {
  assert.match(docs[file], /shared-doctrine\.generated\.md/, `${file} must route shared outcomes to the generated reference`);
}
assert.match(docs['skills/zmux/references/agent-peer.md'], /-s <session>/);
assert.match(docs['skills/zmux/references/agent-peer.md'], /`options\.session`/);
assert.match(docs['skills/zmux/references/guard-and-tab-states.md'], /legacy `waiting` means `ready`|Legacy `waiting` means `ready`|waiting` aliases to `ready`/i);
assert.match(docs['skills/zmux/SKILL.md'], /remote-<host>2/i);
assert.ok(
  flatDocs['skills/zmux/references/guard-and-tab-states.md'].includes('opaque encoded or obfuscated payload'),
  'guard-and-tab-states.md must keep the opaque-payload audit rule',
);
assert.match(docs['docs/domains/pi-zmux-extension.md'], /numbered `remote-<host>N` tab sprawl/i);
assert.match(docs['skills/zmux/references/shared-doctrine.generated.md'], /avoid numbered tab sprawl|stable admin or remote-host/i);

const devSh = read('dev.sh');
assert.ok(
  devSh.includes('if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_SHELL_SETUP:-0}" != "1" ]; then'),
  'dev.sh must not update live shell integration for TARGET=zzmux by default',
);
assert.ok(
  flatDocs['docs/dev/agent-grounding.md'].includes('./dev.sh zzmux # build + install the edge binary (binary only'),
  'agent-grounding.md must document zzmux as a binary-only edge install',
);

console.log(`zmux skill doctor passed (${toolNames.size} Pi tools, ${doctrineManifest.piRuleIds.length} Pi doctrine rules, ${scenarioRecords.length} scenarios, ${skillFiles.length} docs checked)`);
