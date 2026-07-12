import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const sourceDoctrine = dirname(fileURLToPath(import.meta.url));
const sourceRoot = join(sourceDoctrine, "..");
const generatedPaths = [
  "skills/zmux/references/shared-doctrine.generated.md",
  "pi-zmux/src/generated/doctrine.ts",
  "pi-zmux/doctrine-manifest.generated.json",
  "docs/domains/agent-doctrine-matrix.generated.md",
  "skills/zmux/references/testing/prompts.md",
  "skills/zmux/references/testing/answer-key.generated.md",
  "pi-zmux/references/testing/prompts.md",
  "pi-zmux/references/testing/answer-key.generated.md",
];

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

function run(root, mode) {
  return spawnSync(process.execPath, [join(root, "agent-doctrine/generate.mjs"), mode], { cwd: root, encoding: "utf8" });
}

function json(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function writeJson(path, value) {
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`);
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
  const claude = readFileSync(join(root, "skills/zmux/references/testing/prompts.md"), "utf8");
  const pi = readFileSync(join(root, "pi-zmux/references/testing/prompts.md"), "utf8");
  const scenarioDir = join(root, "agent-doctrine/scenarios");
  for (const name of readdirSync(scenarioDir).filter((entry) => entry.endsWith(".json"))) {
    const scenario = json(join(scenarioDir, name));
    const renderedPrompt = `> ${scenario.prompt}`;
    if (scenario.applicability.includes("claude")) assert.ok(claude.includes(renderedPrompt), `${scenario.id} missing from Claude prompts`);
    else assert.ok(!claude.includes(renderedPrompt), `${scenario.id} leaked into Claude prompts`);
    if (scenario.applicability.includes("pi")) assert.ok(pi.includes(renderedPrompt), `${scenario.id} missing from Pi prompts`);
    else assert.ok(!pi.includes(renderedPrompt), `${scenario.id} leaked into Pi prompts`);
  }
}));

test("check names stale generated output and never rewrites it", () => withFixture((root) => {
  assert.equal(run(root, "--write").status, 0);
  const path = join(root, generatedPaths[0]);
  writeFileSync(path, "stale\n");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /skills\/zmux\/references\/shared-doctrine\.generated\.md/);
  assert.equal(readFileSync(path, "utf8"), "stale\n");
}));

test("invalid JSON reports its source record", () => withFixture((root) => {
  const name = readdirSync(join(root, "agent-doctrine/rules"))[0];
  writeFileSync(join(root, "agent-doctrine/rules", name), "{ nope");
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, new RegExp(name.replaceAll(".", "\\.")));
  assert.match(result.stderr, /invalid JSON/);
}));

test("duplicate rule ids fail before generation", () => withFixture((root) => {
  const dir = join(root, "agent-doctrine/rules");
  const original = join(dir, "ZD-001-routing.json");
  cpSync(original, join(dir, "ZD-001-routing-copy.json"));
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /duplicate rule id ZD-001/);
}));

test("missing applicable projection fails", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-001-routing.json");
  const rule = json(path);
  delete rule.projection.pi;
  writeJson(path, rule);
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /projection\.pi is required/);
}));

test("undeclared Pi operation mention fails", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-001-routing.json");
  const rule = json(path);
  rule.projection.pi.promptGuideline += " Use invented_operation.";
  writeJson(path, rule);
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /undeclared Pi operation invented_operation/);
}));

test("one-harness records require a divergence reason", () => withFixture((root) => {
  const path = join(root, "agent-doctrine/rules/ZD-012-pi-lifecycle.json");
  const rule = json(path);
  delete rule.divergenceReason;
  writeJson(path, rule);
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /divergenceReason must be a non-empty string/);
}));

test("symlinked registry records are rejected", { skip: process.platform === "win32" }, () => withFixture((root) => {
  const dir = join(root, "agent-doctrine/rules");
  symlinkSync(join(dir, "ZD-001-routing.json"), join(dir, "ZD-099-linked.json"));
  const result = run(root, "--check");
  assert.equal(result.status, 1);
  assert.match(result.stderr, /must be a committed file, not a symlink/);
}));

test("unknown arguments return usage exit 2", () => withFixture((root) => {
  const result = run(root, "--unknown");
  assert.equal(result.status, 2);
  assert.match(result.stderr, /choose exactly one mode/);
  assert.match(result.stderr, /Usage:/);
}));
