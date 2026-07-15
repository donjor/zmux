import assert from "node:assert/strict";
import { cpSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";

const doctrine = dirname(fileURLToPath(import.meta.url));
const committedPaths = [
  "skills/zmux/references/shared-doctrine.generated.md",
  "docs/reference/agent-doctrine-matrix.generated.md",
];

const RULE = `---
id: "ZD-001"
title: "Rule"
applicability: ["claude","pi"]
---

## Invariant

Do the useful thing.

## Shared instruction

Keep it observable.

## Claude mechanism

Host setup owns isolation.

## Claude enforcement

instruction

## Claude prompt guideline

_None._

## Claude caveats

_None._

## Pi mechanism

Host setup owns isolation.

## Pi enforcement

instruction

## Pi prompt guideline

_None._

## Pi caveats

_None._

## Verify

- fixture test
`;

function scenario({ id, tier = "atomic", prompts = ["Please show the result."], perturbations = "_None._" } = {}) {
  const promptMarkdown = prompts.map((prompt, i) => `## Prompt ${i + 1}\n\n${prompt}`).join("\n\n");
  return `---
id: "${id}"
title: "${id} test"
tier: "${tier}"
doctrineRefs: ["ZD-001"]
applicability: ["claude","pi"]
uatEligible: ${tier === "resilience" ? "false" : "true"}
---

${promptMarkdown}

## Host setup

- Start a disposable fixture.

## Host perturbations

${perturbations}

## Verdict

- outcome: The requested result is visible.
- orchestration: One correct route is used.
- responsiveness: Focus stays usable.
- presentation: Output is clear.
- cleanup: The fixture returns to baseline.

## Evidence

- Inspect the visible result.

## Cleanup

- Remove the fixture.

## Claude answer key

- Host performs Claude setup.

## Pi answer key

- Host performs Pi setup.
`;
}

// Shared-only registry: harness-neutral rules/scenarios that project to both
// Claude and Pi. The Pi extension package, Pi-only records, and the campaign
// migration/known-failure ledgers are deferred and not exercised here.
function fixture() {
  const root = mkdtempSync(join(tmpdir(), "zmux-doctrine-v2-"));
  const ad = join(root, "agent-doctrine");
  mkdirSync(join(ad, "rules/shared"), { recursive: true });
  mkdirSync(join(ad, "scenarios/shared"), { recursive: true });
  mkdirSync(join(root, "pi-zmux/src"), { recursive: true });
  cpSync(join(doctrine, "generate.mjs"), join(ad, "generate.mjs"));
  const operations = ["run", "wait", "log", "runtime_ensure", "peer_ensure"];
  writeFileSync(join(root, "pi-zmux/src/operations.ts"), `export const ZMUX_OPERATIONS = ${JSON.stringify(operations)} as const;\n`);
  writeFileSync(join(root, "pi-zmux/doctrine-manifest.generated.json"), `${JSON.stringify({ dispatcherOperations: operations }, null, 2)}\n`);
  writeFileSync(join(ad, "rules/shared/ZD-001-rule.md"), RULE);
  const records = [
    ["ZS-001", { prompts: ["Run this exactly:\n\n```sh\necho hello\n\n# still fenced\n```\n\nThen tell me what it printed."] }],
    ["ZS-002", { tier: "workflow", prompts: ["Start the sample.", "Now show its result."] }],
    ["ZS-003", { tier: "resilience", prompts: ["Start the sample."], perturbations: "- Host interrupts after submission." }],
    ["ZS-004", { prompts: ["Show a second atomic result."] }],
  ];
  for (const [id, options] of records) writeFileSync(join(ad, `scenarios/shared/${id}.md`), scenario({ id, ...options }));
  return root;
}

function productionFixture() {
  const root = mkdtempSync(join(tmpdir(), "zmux-doctrine-production-"));
  const ad = join(root, "agent-doctrine");
  mkdirSync(ad, { recursive: true });
  cpSync(join(doctrine, "generate.mjs"), join(ad, "generate.mjs"));
  for (const name of ["rules", "scenarios"]) cpSync(join(doctrine, name), join(ad, name), { recursive: true });
  mkdirSync(join(root, "pi-zmux/src"), { recursive: true });
  cpSync(join(doctrine, "../pi-zmux/src/operations.ts"), join(root, "pi-zmux/src/operations.ts"));
  cpSync(join(doctrine, "../pi-zmux/doctrine-manifest.generated.json"), join(root, "pi-zmux/doctrine-manifest.generated.json"));
  return root;
}

function run(root, ...args) { return spawnSync(process.execPath, [join(root, "agent-doctrine/generate.mjs"), ...args], { cwd: root, encoding: "utf8" }); }
function replace(root, relative, before, after) { const path = join(root, relative); const source = readFileSync(path, "utf8"); assert.ok(source.includes(before), `${relative} replacement target`); writeFileSync(path, source.replace(before, after)); }
function withFixture(fn) { const root = fixture(); try { return fn(root); } finally { rmSync(root, { recursive: true, force: true }); } }

test("shared fixture preserves numbered multiline fenced prompt bytes and omits session preamble", () => withFixture((root) => {
  const result = run(root, "--render", "pi-prompts", "--ids", "ZS-001");
  assert.equal(result.status, 0, result.stderr);
  const expected = "Run this exactly:\n\n```sh\necho hello\n\n# still fenced\n```\n\nThen tell me what it printed.";
  assert.equal(result.stdout, expected);
  const claude = run(root, "--render", "claude-prompts", "--ids", "ZS-001");
  assert.equal(claude.status, 0, claude.stderr);
  assert.equal(claude.stdout, result.stdout, "shared prompt bytes must be identical across harnesses");
  assert.doesNotMatch(result.stdout, /Session contract|ordinary Pi worker|Host setup|Verdict|ZS-001/);
}));

test("tier selection composes with caller ordered ids and defaults tier/id order", () => withFixture((root) => {
  const ordered = run(root, "--render", "claude-prompts", "--tier", "atomic", "--ids", "ZS-004,ZS-001");
  assert.equal(ordered.status, 0, ordered.stderr);
  assert.ok(ordered.stdout.indexOf("Show a second atomic") < ordered.stdout.indexOf("Run this exactly"));
  const wrongTier = run(root, "--render", "claude-prompts", "--tier", "atomic", "--ids", "ZS-002");
  assert.equal(wrongTier.status, 1); assert.match(wrongTier.stderr, /not in tier atomic/);
  const all = run(root, "--render", "claude-prompts");
  assert.equal(all.status, 0, all.stderr);
  assert.ok(all.stdout.indexOf("Run this exactly") < all.stdout.indexOf("Show a second atomic") && all.stdout.indexOf("Show a second atomic") < all.stdout.indexOf("Start the sample."));
  assert.match(all.stdout, /BEGIN HOST TURN ZS-002 Prompt 1/);
  assert.match(all.stdout, /BEGIN HOST TURN ZS-002 Prompt 2/);
  assert.doesNotMatch(run(root, "--render", "claude-prompts", "--ids", "ZS-001").stdout, /HOST TURN/);
}));

test("tier, turn, and five-lens verdict contracts reject malformed records", () => withFixture((root) => {
  replace(root, "agent-doctrine/scenarios/shared/ZS-001.md", 'tier: "atomic"', 'tier: "workflow"');
  let result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /workflow scenarios require at least two prompts/);
  replace(root, "agent-doctrine/scenarios/shared/ZS-001.md", 'tier: "workflow"', 'tier: "atomic"');
  replace(root, "agent-doctrine/scenarios/shared/ZS-001.md", "- cleanup: The fixture returns to baseline.\n", "");
  result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /requires exactly outcome/);
}));

test("prompt leakage rejects internal names outside fences but permits ordinary words", () => withFixture((root) => {
  const path = "agent-doctrine/scenarios/shared/ZS-002.md";
  replace(root, path, "Start the sample.", "Start with runtime_ensure.");
  let result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /leaks snake_case operation runtime_ensure/);
  replace(root, path, "runtime_ensure", "run, wait, and log");
  result = run(root, "--render", "claude-prompts"); assert.equal(result.status, 0, result.stderr);
  replace(root, path, "run, wait, and log", "run, wait, and log.\n\n```sh\nruntime_ensure callback\n```");
  result = run(root, "--render", "claude-prompts"); assert.equal(result.status, 0, result.stderr);
  replace(root, path, "run, wait, and log", "use a callback");
  result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /internal vocabulary callback/);
}));

test("answer keys retain every host-only section while worker render cannot leak them", () => withFixture((root) => {
  const answer = run(root, "--render", "pi-answer-key", "--ids", "ZS-003");
  assert.equal(answer.status, 0, answer.stderr);
  for (const field of ["Host setup", "Host perturbations", "outcome", "orchestration", "responsiveness", "presentation", "cleanup", "Evidence", "Cleanup", "Pi mechanics"]) assert.match(answer.stdout, new RegExp(field));
  const worker = run(root, "--render", "pi-prompts", "--ids", "ZS-003");
  assert.equal(worker.status, 0, worker.stderr); assert.doesNotMatch(worker.stdout, /Host interrupts|Host setup|Verdict/);
}));

test("applicability and declared-operation guarantees remain enforced", () => withFixture((root) => {
  replace(root, "agent-doctrine/scenarios/shared/ZS-001.md", 'applicability: ["claude","pi"]', 'applicability: ["pi"]');
  let result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /applicability must be \["claude","pi"\]/);
  const root2 = fixture();
  try {
    replace(root2, "agent-doctrine/scenarios/shared/ZS-001.md", "Host performs Pi setup.", "Host performs invented_operation.");
    result = run(root2, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /undeclared Pi operation invented_operation/);
  } finally { rmSync(root2, { recursive: true, force: true }); }
}));

test("frozen Pi manifest inventory stays aligned with the package-owned operation source", () => withFixture((root) => {
  const path = join(root, "pi-zmux/doctrine-manifest.generated.json");
  const manifest = JSON.parse(readFileSync(path, "utf8"));
  manifest.dispatcherOperations.pop();
  writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /frozen Pi doctrine manifest dispatcherOperations drift from pi-zmux\/src\/operations\.ts/);
}));

test("write/check remain deterministic and render remains stdout-only", () => withFixture((root) => {
  const write = run(root, "--write"); assert.equal(write.status, 0, write.stderr);
  const before = Object.fromEntries(committedPaths.map((path) => [path, readFileSync(join(root, path), "utf8")]));
  const second = run(root, "--write"); assert.equal(second.status, 0, second.stderr); assert.match(second.stdout, /already current/);
  const check = run(root, "--check"); assert.equal(check.status, 0, check.stderr);
  for (const path of committedPaths) assert.equal(readFileSync(join(root, path), "utf8"), before[path]);
  const render = run(root, "--render", "claude-prompts", "--tier", "atomic");
  assert.equal(render.status, 0, render.stderr); assert.equal(render.stdout.includes("Host setup"), false);
  assert.equal(existsSync(join(root, ".dump")), false);
}));

test("wait doctrine still projects bounded post-hoc waiting guidance", () => {
  const root = productionFixture();
  try {
    assert.equal(run(root, "--write").status, 0);
    const shared = readFileSync(join(root, committedPaths[0]), "utf8");
    assert.match(shared, /register.*callback.*before/is);
    assert.match(shared, /post-hoc blind wait/is);
    assert.match(shared, /10.*30.*60/is);
    assert.match(shared, /inspect.*reassess/is);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("check names stale committed output and never rewrites it", () => withFixture((root) => {
  assert.equal(run(root, "--write").status, 0);
  const path = join(root, committedPaths[0]); writeFileSync(path, "stale\n");
  const result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /shared-doctrine\.generated\.md/);
  assert.equal(readFileSync(path, "utf8"), "stale\n");
}));

test("invalid and duplicate scenario records name their source", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/scenarios/shared/ZS-001.md"); writeFileSync(path, "# invalid\n");
  let result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /ZS-001\.md.*frontmatter/);
  const root2 = fixture();
  try {
    cpSync(join(root2, "agent-doctrine/scenarios/shared/ZS-001.md"), join(root2, "agent-doctrine/scenarios/shared/ZS-001-copy.md"));
    result = run(root2, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /duplicate shared scenario id ZS-001/);
  } finally { rmSync(root2, { recursive: true, force: true }); }
}));

test("legacy JSON records are rejected", () => withFixture((root) => {
  writeFileSync(join(root, "agent-doctrine/scenarios/shared/ZS-999.json"), "{}\n");
  const result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /legacy JSON records/);
}));

test("rule projection and uniqueness guarantees remain enforced", () => withFixture((root) => {
  const rule = "agent-doctrine/rules/shared/ZD-001-rule.md";
  cpSync(join(root, rule), join(root, "agent-doctrine/rules/shared/ZD-001-copy.md"));
  let result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /duplicate rule id ZD-001/);
  const root2 = fixture();
  try {
    replace(root2, rule, /\n## Pi mechanism[\s\S]*?(?=\n## Verify)/u.exec(readFileSync(join(root2, rule), "utf8"))[0], "");
    result = run(root2, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /projection\.pi is required/);
  } finally { rmSync(root2, { recursive: true, force: true }); }
}));

test("symlinked registry records remain rejected", { skip: process.platform === "win32" }, () => withFixture((root) => {
  symlinkSync(join(root, "agent-doctrine/rules/shared/ZD-001-rule.md"), join(root, "agent-doctrine/rules/shared/ZD-099-linked.md"));
  const result = run(root, "--check"); assert.equal(result.status, 1); assert.match(result.stderr, /must be a committed file, not a symlink/);
}));

test("unknown arguments and render artifacts fail with stable exit contracts", () => withFixture((root) => {
  let result = run(root, "--unknown"); assert.equal(result.status, 2); assert.match(result.stderr, /Usage:/);
  result = run(root, "--render", "unknown"); assert.equal(result.status, 1); assert.equal(result.stdout, ""); assert.match(result.stderr, /unknown render artifact/);
}));
