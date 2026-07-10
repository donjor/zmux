#!/usr/bin/env node
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = fileURLToPath(new URL('.', import.meta.url));
const root = join(here, '..', '..', '..');

function read(rel) {
  return readFileSync(join(root, rel), 'utf8');
}

const dispatcherSource = read('pi-zmux/src/dispatcher.ts');
const toolNames = new Set([...dispatcherSource.matchAll(/name:\s*"(zmux_[^"]+)"/g)].map((match) => match[1]));
assert.deepEqual([...toolNames], ['zmux_lite'], 'Pi must expose one compact dispatcher tool');
const operationBlock = /export const ZMUX_OPERATIONS = \[([\s\S]*?)\] as const;/u.exec(dispatcherSource)?.[1] ?? '';
const operations = new Set([...operationBlock.matchAll(/"([a-z0-9_]+)"/g)].map((match) => match[1]));
assert.equal(operations.size, 40, `expected 40 dispatcher operations, got ${operations.size}`);

const skillFiles = [
  'skills/zmux/SKILL.md',
  'skills/zmux/references/run-observe.md',
  'skills/zmux/references/guard-and-tab-states.md',
  'skills/zmux/references/agent-peer.md',
  'skills/zmux/references/agent-worker.md',
  'skills/zmux/references/cli-catalog.md',
  'docs/domains/pi-zmux-extension.md',
  'docs/dev/agent-grounding.md',
  'docs/dev/test-prompts/zmux-agent-pi-zmux-testing-prompt.md',
  'docs/dev/test-prompts/zmux-agent-skill-testing-prompt.md',
];
const docs = Object.fromEntries(skillFiles.map((file) => [file, read(file)]));
const combined = Object.values(docs).join('\n');

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
assert.match(docs['skills/zmux/references/agent-peer.md'], /-s <session>/);
assert.match(docs['skills/zmux/references/agent-peer.md'], /`options\.session`/);
assert.match(docs['skills/zmux/references/guard-and-tab-states.md'], /legacy `waiting` means `ready`|Legacy `waiting` means `ready`|waiting` aliases to `ready`/i);
assert.match(docs['skills/zmux/SKILL.md'], /remote-<host>2/i);
assert.match(docs['skills/zmux/references/guard-and-tab-states.md'], /opaque\nencoded or obfuscated payload/i);
assert.match(docs['docs/domains/pi-zmux-extension.md'], /numbered `remote-<host>N` tab sprawl/i);
assert.match(docs['docs/dev/test-prompts/zmux-agent-pi-zmux-testing-prompt.md'], /numbered remote-admin tab names/i);

const devSh = read('dev.sh');
assert.ok(
  devSh.includes('if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_SHELL_SETUP:-0}" != "1" ]; then'),
  'dev.sh must not update live shell integration for TARGET=zzmux by default',
);
assert.match(docs['docs/dev/agent-grounding.md'], /\.\/dev\.sh zzmux\s+# build \+ install the edge binary \(binary only/i);

console.log(`zmux skill doctor passed (${toolNames.size} Pi tools, ${skillFiles.length} docs checked)`);
