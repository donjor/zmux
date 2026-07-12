import { createHash } from "node:crypto";
import {
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  realpathSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const USAGE = `Usage: node agent-doctrine/generate.mjs <--write|--check>

Generate committed Claude/Pi doctrine projections from agent-doctrine/rules/*.json
and validate agent-doctrine/scenarios/*.json.

Modes:
  --write  validate sources and rewrite changed generated outputs
  --check  validate sources and fail when a generated output is stale
  -h, --help

Writes (--write only):
  skills/zmux/references/shared-doctrine.generated.md
  pi-zmux/src/generated/doctrine.ts
  pi-zmux/doctrine-manifest.generated.json
  docs/domains/agent-doctrine-matrix.generated.md
  skills/zmux/references/testing/prompts.md
  skills/zmux/references/testing/answer-key.generated.md
  pi-zmux/references/testing/prompts.md
  pi-zmux/references/testing/answer-key.generated.md

Exit codes: 0 success; 1 invalid/stale doctrine; 2 usage error.`;

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const HARNESSES = ["claude", "pi"];
const ENFORCEMENT = new Set(["instruction", "typed-operation", "composite", "guard", "unsupported"]);
const GENERATED_BANNER = "GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`.";

function fail(message, code = 1) {
  console.error(`agent-doctrine: ${message}`);
  process.exitCode = code;
}

function usageError(message) {
  console.error(`agent-doctrine: ${message}\n\n${USAGE}`);
  process.exitCode = 2;
}

function withinRoot(path) {
  const rel = relative(root, path);
  return rel !== "" && !rel.startsWith(`..${sep}`) && rel !== ".." && !resolve(path).startsWith(`${root}${sep}..${sep}`);
}

function readJsonRecords(kind) {
  const dir = join(here, kind);
  const names = readdirSync(dir).filter((name) => name.endsWith(".json")).sort();
  if (names.length === 0) throw new Error(`${kind}/ contains no JSON records`);
  return names.map((name) => {
    const path = join(dir, name);
    if (!withinRoot(path)) throw new Error(`${kind}/${name} escapes repository root`);
    if (lstatSync(path).isSymbolicLink()) throw new Error(`${kind}/${name} must be a committed file, not a symlink`);
    const real = realpathSync(path);
    if (!withinRoot(real)) throw new Error(`${kind}/${name} resolves outside repository root`);
    let value;
    try {
      value = JSON.parse(readFileSync(path, "utf8"));
    } catch (error) {
      throw new Error(`${kind}/${name}: invalid JSON: ${error.message}`);
    }
    return { name, path, value };
  });
}

function expect(condition, message) {
  if (!condition) throw new Error(message);
}

function string(value, field) {
  expect(typeof value === "string" && value.trim() !== "", `${field} must be a non-empty string`);
}

function stringArray(value, field, { allowEmpty = false } = {}) {
  expect(Array.isArray(value), `${field} must be an array`);
  expect(allowEmpty || value.length > 0, `${field} must not be empty`);
  for (const [index, entry] of value.entries()) string(entry, `${field}[${index}]`);
}

function exactHarnesses(value, field) {
  stringArray(value, field);
  const unique = [...new Set(value)];
  expect(unique.length === value.length, `${field} contains duplicates`);
  expect(unique.every((harness) => HARNESSES.includes(harness)), `${field} contains an unknown harness`);
  expect([...unique].sort().join(",") === unique.join(","), `${field} must be sorted`);
  return unique;
}

function validateRule(record) {
  const rule = record.value;
  const at = `rules/${record.name}`;
  expect(rule && typeof rule === "object" && !Array.isArray(rule), `${at} must contain an object`);
  expect(/^ZD-\d{3}$/.test(rule.id), `${at}.id must match ZD-###`);
  expect(record.name.startsWith(`${rule.id}-`), `${at} filename must start with ${rule.id}-`);
  for (const field of ["title", "invariant", "sharedInstruction"]) string(rule[field], `${at}.${field}`);
  const appliesTo = exactHarnesses(rule.appliesTo, `${at}.appliesTo`);
  if (appliesTo.length !== HARNESSES.length) string(rule.divergenceReason, `${at}.divergenceReason`);
  expect(rule.projection && typeof rule.projection === "object", `${at}.projection must be an object`);
  expect(rule.caveats && typeof rule.caveats === "object", `${at}.caveats must be an object`);
  for (const harness of HARNESSES) {
    const projection = rule.projection[harness];
    const caveats = rule.caveats[harness];
    if (!appliesTo.includes(harness)) {
      expect(projection === undefined, `${at}.projection.${harness} must be omitted when not applicable`);
      expect(caveats === undefined, `${at}.caveats.${harness} must be omitted when not applicable`);
      continue;
    }
    expect(projection && typeof projection === "object", `${at}.projection.${harness} is required`);
    string(projection.mechanism, `${at}.projection.${harness}.mechanism`);
    expect(ENFORCEMENT.has(projection.enforcement), `${at}.projection.${harness}.enforcement is invalid`);
    expect(
      projection.promptGuideline === null || (typeof projection.promptGuideline === "string" && projection.promptGuideline.trim() !== ""),
      `${at}.projection.${harness}.promptGuideline must be null or a non-empty string`,
    );
    stringArray(caveats, `${at}.caveats.${harness}`, { allowEmpty: true });
  }
  stringArray(rule.verifyRefs, `${at}.verifyRefs`);
  return rule;
}

function validateScenario(record, ruleIds) {
  const scenario = record.value;
  const at = `scenarios/${record.name}`;
  expect(scenario && typeof scenario === "object" && !Array.isArray(scenario), `${at} must contain an object`);
  expect(/^ZS-\d{3}$/.test(scenario.id), `${at}.id must match ZS-###`);
  expect(record.name.startsWith(`${scenario.id}-`), `${at} filename must start with ${scenario.id}-`);
  for (const field of ["title", "prompt", "expectedOutcome"]) string(scenario[field], `${at}.${field}`);
  stringArray(scenario.doctrineRefs, `${at}.doctrineRefs`);
  for (const ref of scenario.doctrineRefs) expect(ruleIds.has(ref), `${at}.doctrineRefs references missing ${ref}`);
  const applicability = exactHarnesses(scenario.applicability, `${at}.applicability`);
  if (applicability.length !== HARNESSES.length) string(scenario.divergenceReason, `${at}.divergenceReason`);
  for (const field of ["setup", "evidence", "safety", "cleanup"]) {
    stringArray(scenario[field], `${at}.${field}`, { allowEmpty: field === "cleanup" });
  }
  expect(scenario.answerKey && typeof scenario.answerKey === "object", `${at}.answerKey must be an object`);
  for (const harness of HARNESSES) {
    if (applicability.includes(harness)) stringArray(scenario.answerKey[harness], `${at}.answerKey.${harness}`);
    else expect(scenario.answerKey[harness] === undefined, `${at}.answerKey.${harness} must be omitted when not applicable`);
  }
  return scenario;
}

function uniqueById(records, kind) {
  const seen = new Set();
  for (const record of records) {
    expect(!seen.has(record.id), `duplicate ${kind} id ${record.id}`);
    seen.add(record.id);
  }
  return records.sort((a, b) => a.id.localeCompare(b.id));
}

function declaredOperations() {
  const source = readFileSync(join(root, "pi-zmux/src/operations.ts"), "utf8");
  const block = /export const ZMUX_OPERATIONS = \[([\s\S]*?)\] as const;/u.exec(source)?.[1];
  expect(block, "cannot parse pi-zmux/src/operations.ts ZMUX_OPERATIONS");
  return new Set([...block.matchAll(/"([a-z0-9_]+)"/g)].map((match) => match[1]));
}

function validatePiOperationMentions(rules, scenarios) {
  const operations = declaredOperations();
  const allowed = new Set(["needs_user_input"]);
  const texts = [];
  for (const rule of rules) {
    if (!rule.appliesTo.includes("pi")) continue;
    texts.push([rule.id, rule.projection.pi.mechanism, rule.projection.pi.promptGuideline ?? ""]);
  }
  for (const scenario of scenarios) {
    if (!scenario.applicability.includes("pi")) continue;
    texts.push([scenario.id, ...scenario.answerKey.pi]);
  }
  for (const [id, ...parts] of texts) {
    for (const match of parts.join("\n").matchAll(/\b[a-z]+_[a-z0-9_]+\b/g)) {
      expect(operations.has(match[0]) || allowed.has(match[0]), `${id} mentions undeclared Pi operation ${match[0]}`);
    }
  }
  return [...operations].sort();
}

function markdownText(value) {
  return value.replaceAll("|", "\\|").replaceAll("\n", " ");
}

function renderClaudeReference(rules) {
  const body = rules
    .filter((rule) => rule.appliesTo.includes("claude"))
    .map((rule) => {
      const caveats = rule.caveats.claude.length > 0 ? rule.caveats.claude.map((item) => `  - Caveat: ${item}`).join("\n") : "  - Caveat: none.";
      return `### ${rule.id} · ${rule.title}\n\n- **Invariant:** ${rule.invariant}\n- **Instruction:** ${rule.sharedInstruction}\n- **Claude mechanism:** ${rule.projection.claude.mechanism} (${rule.projection.claude.enforcement})\n${caveats}\n- **Verify:** ${rule.verifyRefs.map((ref) => `\`${ref}\``).join(", ")}`;
    })
    .join("\n\n");
  return `<!-- ${GENERATED_BANNER} -->\n\n# Shared zmux doctrine\n\nThese are harness-neutral outcomes projected for the Claude skill. Claude-specific command sequences and hooks remain in the handwritten references.\n\n${body}\n`;
}

function renderPiModule(rules) {
  const piRules = rules.filter((rule) => rule.appliesTo.includes("pi"));
  const guidelines = piRules.flatMap((rule) => rule.projection.pi.promptGuideline ? [rule.projection.pi.promptGuideline] : []);
  const ids = piRules.map((rule) => rule.id);
  return `// ${GENERATED_BANNER}\n\nexport const SHARED_ZMUX_PROMPT_GUIDELINES = ${JSON.stringify(guidelines, null, 2)} as const;\n\nexport const PI_DOCTRINE_RULE_IDS = ${JSON.stringify(ids, null, 2)} as const;\n`;
}

function manifestPayload(rules) {
  return rules
    .filter((rule) => rule.appliesTo.includes("pi"))
    .map((rule) => ({
      id: rule.id,
      enforcement: rule.projection.pi.enforcement,
      mechanism: rule.projection.pi.mechanism,
      promptProjected: Boolean(rule.projection.pi.promptGuideline),
    }));
}

function renderManifest(rules, scenarios, operations) {
  const piRules = manifestPayload(rules);
  const digestInput = JSON.stringify({ piRules, piScenarioIds: scenarios.filter((item) => item.applicability.includes("pi")).map((item) => item.id) });
  const manifest = {
    schema: "zmux.doctrine-manifest.v1",
    generated: true,
    coverageComplete: true,
    digest: `sha256:${createHash("sha256").update(digestInput).digest("hex")}`,
    piRuleIds: piRules.map((rule) => rule.id),
    promptRuleIds: piRules.filter((rule) => rule.promptProjected).map((rule) => rule.id),
    piRules,
    piScenarioIds: scenarios.filter((item) => item.applicability.includes("pi")).map((item) => item.id),
    dispatcherOperations: operations,
  };
  return `${JSON.stringify(manifest, null, 2)}\n`;
}

function renderMatrix(rules) {
  const rows = [];
  for (const rule of rules) {
    for (const harness of HARNESSES) {
      if (!rule.appliesTo.includes(harness)) {
        rows.push(`| ${rule.id} | ${markdownText(rule.title)} | ${harness} | unsupported | — | ${markdownText(rule.divergenceReason)} |`);
        continue;
      }
      const projection = rule.projection[harness];
      const caveat = rule.caveats[harness].join(" ") || "—";
      rows.push(`| ${rule.id} | ${markdownText(rule.title)} | ${harness} | ${projection.enforcement} | ${markdownText(projection.mechanism)} | ${markdownText(caveat)} |`);
    }
  }
  return `<!-- ${GENERATED_BANNER} -->\n\n# Agent doctrine capability matrix\n\n| Rule | Outcome | Harness | Enforcement | Mechanism | Caveat / divergence |\n|---|---|---|---|---|---|\n${rows.join("\n")}\n`;
}

function renderPrompts(scenarios, harness) {
  const contract = harness === "claude"
    ? "You are an ordinary Claude worker exercising branch-local zmux doctrine against isolated zzmux. Complete each supplied terminal task directly and safely through documented zmux CLI verbs, invoking the isolated binary as `zzmux` rather than live `zmux`. Bounded repository inspection may use your shell; do not use raw tmux, hidden jobs, or ad-hoc polling. Inspect real state before asserting success, pin the intended session, keep focus unchanged unless explicitly asked, and report concise concrete evidence after each task."
    : "You are an ordinary Pi worker exercising the branch-local canonical zmux dispatcher against isolated zzmux. Complete each supplied terminal task directly and safely through the zmux tool, which the host has bound to `PI_ZMUX_BIN=zzmux`. Bounded repository inspection may use Bash; do not shell out to zmux or raw tmux, bypass the Bash guard, create hidden jobs, or poll. Inspect real state before asserting success, pin the intended session, keep focus unchanged unless explicitly asked, and report concise concrete evidence after each task.";
  const body = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .map((scenario) => `## ${scenario.id} · ${scenario.title}\n\n> ${scenario.prompt}`)
    .join("\n\n");
  const title = harness === "claude" ? "Claude zmux worker prompts" : "Pi zmux worker prompts";
  return `<!-- ${GENERATED_BANNER} -->\n\n# ${title}\n\nThe host sends the session contract once, then sends one scenario prompt at a time. Headings and answer keys stay host-side.\n\n## Session contract\n\n> ${contract}\n\n${body}\n`;
}

function renderAnswerKey(scenarios, harness) {
  const rows = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .map((scenario) => `### ${scenario.id} · ${scenario.title}\n\n- **Expected outcome:** ${scenario.expectedOutcome}\n- **${harness === "claude" ? "Claude" : "Pi"} mechanics:** ${scenario.answerKey[harness].join("; ")}\n- **Evidence:** ${scenario.evidence.join("; ")}\n- **Safety:** ${scenario.safety.join("; ")}\n- **Cleanup:** ${scenario.cleanup.length > 0 ? scenario.cleanup.join("; ") : "none"}${scenario.divergenceReason ? `\n- **Divergence:** ${scenario.divergenceReason}` : ""}`)
    .join("\n\n");
  return `<!-- ${GENERATED_BANNER} -->\n\n# ${harness === "claude" ? "Claude" : "Pi"} host answer key\n\nHost-only expected mechanics and evidence. Never send this file or its operation/verb hints to the worker.\n\n${rows}\n`;
}

function outputs(rules, scenarios, operations) {
  return new Map([
    ["skills/zmux/references/shared-doctrine.generated.md", renderClaudeReference(rules)],
    ["pi-zmux/src/generated/doctrine.ts", renderPiModule(rules)],
    ["pi-zmux/doctrine-manifest.generated.json", renderManifest(rules, scenarios, operations)],
    ["docs/domains/agent-doctrine-matrix.generated.md", renderMatrix(rules)],
    ["skills/zmux/references/testing/prompts.md", renderPrompts(scenarios, "claude")],
    ["skills/zmux/references/testing/answer-key.generated.md", renderAnswerKey(scenarios, "claude")],
    ["pi-zmux/references/testing/prompts.md", renderPrompts(scenarios, "pi")],
    ["pi-zmux/references/testing/answer-key.generated.md", renderAnswerKey(scenarios, "pi")],
  ]);
}

function writeOutputs(rendered) {
  let changed = 0;
  for (const [rel, content] of rendered) {
    const path = join(root, rel);
    expect(withinRoot(path), `output path escapes root: ${rel}`);
    let current;
    try { current = readFileSync(path, "utf8"); } catch { current = undefined; }
    if (current === content) continue;
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, content, "utf8");
    console.log(`wrote ${rel}`);
    changed += 1;
  }
  console.log(`agent-doctrine: ${changed === 0 ? "already current" : `wrote ${changed} output(s)`}`);
}

function checkOutputs(rendered) {
  const stale = [];
  for (const [rel, content] of rendered) {
    let current;
    try { current = readFileSync(join(root, rel), "utf8"); } catch { current = undefined; }
    if (current !== content) stale.push(rel);
  }
  if (stale.length > 0) throw new Error(`stale generated output(s): ${stale.join(", ")}; run make gen-doctrine`);
  console.log(`agent-doctrine: ${rendered.size} generated outputs current`);
}

function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && ["-h", "--help"].includes(args[0])) {
    console.log(USAGE);
    return;
  }
  if (args.length !== 1 || !["--write", "--check"].includes(args[0])) {
    usageError("choose exactly one mode: --write or --check");
    return;
  }
  try {
    const rules = uniqueById(readJsonRecords("rules").map(validateRule), "rule");
    const ruleIds = new Set(rules.map((rule) => rule.id));
    const scenarios = uniqueById(readJsonRecords("scenarios").map((record) => validateScenario(record, ruleIds)), "scenario");
    const operations = validatePiOperationMentions(rules, scenarios);
    const rendered = outputs(rules, scenarios, operations);
    if (args[0] === "--write") writeOutputs(rendered);
    else checkOutputs(rendered);
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }
}

main();
