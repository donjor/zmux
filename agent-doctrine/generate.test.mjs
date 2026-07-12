import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const sourceDoctrine = dirname(fileURLToPath(import.meta.url));
const sourceRoot = join(sourceDoctrine, "..");
const committedPaths = [
  "skills/zmux/references/shared-doctrine.generated.md",
  "pi-zmux/src/generated/doctrine.ts",
  "pi-zmux/doctrine-manifest.generated.json",
  "docs/reference/agent-doctrine-matrix.generated.md",
];
const generatedPaths = [...committedPaths];

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "zmux-doctrine-"));
  mkdirSync(join(root, "agent-doctrine"), { recursive: true });
  cpSync(join(sourceDoctrine, "generate.mjs"), join(root, "agent-doctrine/generate.mjs"));
  cpSync(join(sourceDoctrine, "rules"), join(root, "agent-doctrine/rules"), { recursive: true });
  cpSync(join(sourceDoctrine, "scenarios"), join(root, "agent-doctrine/scenarios"), { recursive: true });
  mkdirSync(join(root, "pi-zmux/src"), { recursive: true });
  cpSync(join(sourceRoot, "pi-zmux/src/operations.ts"), join(root, "pi-zmux/src/operations.ts"));
  return root;
}

function run(root, ...args) {
  return spawnSync(process.execPath, [join(root, "agent-doctrine/generate.mjs"), ...args], { cwd: root, encoding: "utf8" });
}

function replaceFile(path, oldText, newText) {
  const source = readFileSync(path, "utf8");
  assert.ok(source.includes(oldText), `${path} must contain replacement target`);
  writeFileSync(path, source.replace(oldText, newText));
}

function withFixture(fn) {
  const root = fixture();
  try { return fn(root); } finally { rmSync(root, { recursive: true, force: true }); }
}

test("write and check are byte-deterministic", () => withFixture((root) => {
  const first = run(root, "--write");
  assert.equal(first.status, 0, first.stderr);
  const before = Object.fromEntries(generatedPaths.map((path) => [path, readFileSync(join(root, path), "utf8")]));
  const second = run(root, "--write");
  assert.equal(second.status, 0, second.stderr);
  assert.match(second.stdout, /already current/);
  const check = run(root, "--check");
  assert.equal(check.status, 0, check.stderr);
  for (const path of generatedPaths) assert.equal(readFileSync(join(root, path), "utf8"), before[path]);
}));

test("shared scenario prompts are identical and harness-only prompts stay local", () => withFixture((root) => {
  assert.equal(run(root, "--write").status, 0);
  const claudeResult = run(root, "--render", "claude-prompts");
  const piResult = run(root, "--render", "pi-prompts");
  assert.equal(claudeResult.status, 0, claudeResult.stderr);
  assert.equal(piResult.status, 0, piResult.stderr);
  const claude = claudeResult.stdout;
  const pi = piResult.stdout;
  for (const [harness, prompts] of [["Claude", claude], ["Pi", pi]]) {
    assert.ok(!prompts.includes(`**${harness} mechanics:**`), `${harness} worker prompts must not leak host answer-key mechanics`);
    assert.ok(!prompts.includes("# Claude host answer key") && !prompts.includes("# Pi host answer key"), `${harness} worker prompts must not leak a host answer key`);
  }
  const scenarioDir = join(root, "agent-doctrine/scenarios");
  for (const name of readdirSync(scenarioDir).filter((entry) => entry.endsWith(".md"))) {
    const source = readFileSync(join(scenarioDir, name), "utf8");
    const id = /^id: "([^"]+)"$/mu.exec(source)?.[1];
    const applicability = JSON.parse(/^applicability: (.+)$/mu.exec(source)?.[1] ?? "[]");
    const prompt = /^## Prompt\n\n([\s\S]*?)\n\n## Setup$/mu.exec(source)?.[1];
    assert.ok(id && prompt, `${name} must expose readable Markdown id and prompt sections`);
    const renderedPrompt = `> ${prompt}`;
    if (applicability.includes("claude")) assert.ok(claude.includes(renderedPrompt), `${id} missing from Claude prompts`);
    else assert.ok(!claude.includes(renderedPrompt), `${id} leaked into Claude prompts`);
    if (applicability.includes("pi")) assert.ok(pi.includes(renderedPrompt), `${id} missing from Pi prompts`);
    else assert.ok(!pi.includes(renderedPrompt), `${id} leaked into Pi prompts`);
  }
}));

test("check names stale committed output and never rewrites it", () => withFixture((root) => {
  assert.equal(run(root, "--write").status, 0);
  const path = join(root, committedPaths[0]);
  writeFileSync(path, "stale\n");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /skills\/zmux\/references\/shared-doctrine\.generated\.md/);
  assert.equal(readFileSync(path, "utf8"), "stale\n");
}));

test("live-test rendering is stdout-only", () => withFixture((root) => {
  const result = run(root, "--render", "pi-answer-key");
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /# Pi host answer key/);
  assert.equal(existsSync(join(root, ".dump")), false);
}));

test("invalid rule Markdown reports its source record", () => withFixture((root) => {
  const name = readdirSync(join(root, "agent-doctrine/rules"))[0];
  writeFileSync(join(root, "agent-doctrine/rules", name), "# not a rule record\n");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, new RegExp(name.replaceAll(".", "\\.")));
  assert.match(result.stderr, /frontmatter/);
}));

test("invalid scenario Markdown reports its source record", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/scenarios/ZS-001-runtime-start.md");
  writeFileSync(path, "# not a scenario record\n");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /ZS-001-runtime-start\.md/);
  assert.match(result.stderr, /frontmatter/);
}));

test("legacy scenario JSON is rejected", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/scenarios/ZS-999-legacy.json");
  writeFileSync(path, '{"id":"ZS-999"}\n');
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /legacy JSON records/);
}));

test("duplicate rule ids fail before generation", () => withFixture((root) => {
  const dir = join(root, "agent-doctrine/rules");
  const original = join(dir, "ZD-001-routing.md");
  cpSync(original, join(dir, "ZD-001-routing-copy.md"));
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /duplicate rule id ZD-001/);
}));

test("missing applicable projection fails", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-001-routing.md");
  const source = readFileSync(path, "utf8");
  writeFileSync(path, source.replace(/\n## Pi mechanism[\s\S]*?(?=\n## Verify)/u, ""));
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /projection\.pi is required/);
}));

test("undeclared Pi operation mention fails", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-001-routing.md");
  replaceFile(path, "and Pi lifecycle; never background long-running commands.", "and Pi lifecycle; never background long-running commands. Use invented_operation.");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /undeclared Pi operation invented_operation/);
}));

test("one-harness records require a divergence reason", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-012-pi-lifecycle.md");
  replaceFile(path, 'divergenceReason: "Claude does not expose Pi extension reload/respawn lifecycle operations."\n', "");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /divergenceReason must be a non-empty string/);
}));

test("symlinked registry records are rejected", { skip: process.platform === "win32" }, () => withFixture((root) => {
  const dir = join(root, "agent-doctrine/rules");
  symlinkSync(join(dir, "ZD-001-routing.md"), join(dir, "ZD-099-linked.md"));
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /must be a committed file, not a symlink/);
}));

test("unknown arguments return usage exit 2", () => withFixture((root) => {
  const result = run(root, "--unknown");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /choose --write, --check, or --render/);
  assert.match(result.stderr, /Usage:/);
}));

test("unknown render artifacts fail without output", () => withFixture((root) => {
  const result = run(root, "--render", "unknown");
  assert.equal(result.status, 1);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /unknown render artifact unknown/);
}));
