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

function toolNamesFromSource() {
  const files = [
    'pi-extension/src/tools/core.ts',
    'pi-extension/src/tools/tabs.ts',
    'pi-extension/src/tools/panes.ts',
    'pi-extension/src/tools/runtimes.ts',
  ];
  const names = new Set();
  for (const file of files) {
    const text = read(file);
    for (const match of text.matchAll(/name:\s*"(zmux_[^"]+)"/g)) {
      names.add(match[1]);
    }
  }
  return names;
}

const toolNames = toolNamesFromSource();
assert.ok(toolNames.size >= 30, `expected the Pi tool surface, got ${toolNames.size} tools`);

const skillFiles = [
  'skills/zmux/SKILL.md',
  'skills/zmux/references/run-observe.md',
  'skills/zmux/references/guard-and-tab-states.md',
  'skills/zmux/references/agent-peer.md',
  'skills/zmux/references/agent-worker.md',
  'skills/zmux/references/cli-catalog.md',
  'docs/domains/pi-zmux-extension.md',
  'docs/dev/agent-grounding.md',
];
const docs = Object.fromEntries(skillFiles.map((file) => [file, read(file)]));
const combined = Object.values(docs).join('\n');

const criticalTools = [
  'zmux_run',
  'zmux_runtime_ensure',
  'zmux_runtime_logs',
  'zmux_interactive_type',
  'zmux_tab_status',
  'zmux_tab_peer',
  'zmux_type',
  'zmux_session_run',
  'zmux_pi_reload',
];
for (const name of criticalTools) {
  assert.ok(toolNames.has(name), `critical Pi tool missing from source: ${name}`);
  assert.ok(combined.includes(name), `critical Pi tool missing from docs/skill doctrine: ${name}`);
}

const allowedNonToolMentions = new Set([
  'zmux_born',
  'zmux_keep',
  'zmux_label',
  'zmux_tab',
]);
for (const [file, text] of Object.entries(docs)) {
  for (const match of text.matchAll(/\bzmux_[a-z0-9_]+\b/g)) {
    const name = match[0];
    if (name.endsWith('_')) continue; // wildcard family, e.g. zmux_tab_*
    if (allowedNonToolMentions.has(name)) continue; // tmux option names, not Pi tools
    assert.ok(toolNames.has(name), `${file} mentions ${name}, but no Pi tool by that name is registered`);
  }
}

assert.ok(!combined.includes(':::AGENT_DONE'), 'skill/docs must not revive AGENT_DONE sentinel doctrine');
assert.match(docs['skills/zmux/references/run-observe.md'], /does not print completion sentinels/i);
assert.match(docs['skills/zmux/references/run-observe.md'], /Do not use `watch` as lifecycle truth/i);
assert.match(docs['skills/zmux/references/agent-peer.md'], /not the primary completion signal/i);
assert.match(docs['skills/zmux/references/agent-peer.md'], /-s <session>/);
assert.match(docs['skills/zmux/references/agent-peer.md'], /`session` parameter/);
assert.match(docs['skills/zmux/references/guard-and-tab-states.md'], /legacy `waiting` means `ready`|Legacy `waiting` means `ready`|waiting` aliases to `ready`/i);

const devSh = read('dev.sh');
assert.ok(
  devSh.includes('if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_SHELL_SETUP:-0}" != "1" ]; then'),
  'dev.sh must not update live shell integration for TARGET=zzmux by default',
);
assert.match(docs['docs/dev/agent-grounding.md'], /\.\/dev\.sh zzmux\s+# build \+ install the edge binary \(binary only/i);

console.log(`zmux skill doctor passed (${toolNames.size} Pi tools, ${skillFiles.length} docs checked)`);
