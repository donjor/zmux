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

const USAGE = `Usage: node agent-doctrine/generate.mjs <--write|--check|--render <artifact>>

Generate committed Claude/Pi doctrine projections from authored Markdown records
in agent-doctrine/rules/*.md and agent-doctrine/scenarios/*.md.

Modes:
  --write  validate sources and rewrite committed runtime projections
  --check  validate sources and fail when a committed projection is stale; never writes
  --render <artifact>
           validate sources and print one maintainer live-test artifact to stdout
  -h, --help

Committed projections (--write only):
  skills/zmux/references/shared-doctrine.generated.md
  pi-zmux/src/generated/doctrine.ts
  pi-zmux/doctrine-manifest.generated.json
  docs/reference/agent-doctrine-matrix.generated.md

Render artifacts:
  claude-prompts | claude-answer-key | pi-prompts | pi-answer-key

Exit codes: 0 success; 1 invalid/stale doctrine; 2 usage error.`;

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const HARNESSES = ["claude", "pi"];
const ENFORCEMENT = new Set(["instruction", "typed-operation", "composite", "guard", "unsupported"]);
const GENERATED_BANNER = "GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`.";
const RENDERED_BANNER = "RENDERED ARTIFACT — edit agent-doctrine/ and rerun this --render command; do not save or commit this output.";

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

function checkedRecordPath(kind, name) {
  const path = join(here, kind, name);
  if (!withinRoot(path)) throw new Error(`${kind}/${name} escapes repository root`);
  if (lstatSync(path).isSymbolicLink()) throw new Error(`${kind}/${name} must be a committed file, not a symlink`);
  const real = realpathSync(path);
  if (!withinRoot(real)) throw new Error(`${kind}/${name} resolves outside repository root`);
  return path;
}

const RULE_FRONTMATTER = new Set(["id", "title", "applicability", "divergenceReason"]);
const RULE_SECTIONS = new Set([
  "Invariant", "Shared instruction", "Claude mechanism", "Claude enforcement", "Claude prompt guideline", "Claude caveats",
  "Pi mechanism", "Pi enforcement", "Pi prompt guideline", "Pi caveats", "Verify",
]);
const SCENARIO_FRONTMATTER = new Set(["id", "title", "doctrineRefs", "applicability", "divergenceReason"]);
const SCENARIO_SECTIONS = new Set(["Prompt", "Setup", "Expected outcome", "Evidence", "Safety", "Cleanup", "Claude answer key", "Pi answer key"]);

function parseJsonFrontmatter(source, at, allowedFields) {
  const fields = {};
  for (const [index, line] of source.split("\n").entries()) {
    if (!line.trim()) continue;
    const match = /^([A-Za-z][A-Za-z0-9]*):\s+(.+)$/u.exec(line);
    expect(match, `${at} frontmatter line ${index + 1} must be key: JSON-value`);
    const [, key, encoded] = match;
    expect(allowedFields.has(key), `${at} frontmatter contains unknown field ${key}`);
    expect(fields[key] === undefined, `${at} frontmatter contains duplicate field ${key}`);
    try {
      fields[key] = JSON.parse(encoded);
    } catch (error) {
      throw new Error(`${at} frontmatter field ${key} is not valid inline JSON: ${error.message}`);
    }
  }
  return fields;
}

function parseSections(source, at, allowedSections) {
  const sections = {};
  const heading = /^## (.+)$/gmu;
  const matches = [...source.matchAll(heading)];
  expect(matches.length > 0, `${at} must contain Markdown sections`);
  expect(source.slice(0, matches[0].index).trim() === "", `${at} must not contain content before its first section`);
  for (const [index, match] of matches.entries()) {
    const name = match[1].trim();
    expect(allowedSections.has(name), `${at} contains unknown section ${name}`);
    expect(sections[name] === undefined, `${at} contains duplicate section ${name}`);
    const start = match.index + match[0].length;
    const end = matches[index + 1]?.index ?? source.length;
    sections[name] = source.slice(start, end).trim();
  }
  return sections;
}

function paragraphSection(sections, name, at) {
  const value = sections[name];
  string(value, `${at} section ${name}`);
  expect(!value.includes("\n- "), `${at} section ${name} must be prose, not a list`);
  return value.replace(/\s*\n\s*/gu, " ");
}

function listSection(sections, name, at, { allowEmpty = false, required = true } = {}) {
  const source = sections[name];
  if (source === undefined) {
    expect(!required, `${at} section ${name} is required`);
    return undefined;
  }
  if (source === "_None._") {
    expect(allowEmpty, `${at} section ${name} must not be empty`);
    return [];
  }
  const lines = source.split("\n");
  const values = lines.map((line, index) => {
    const match = /^- (.+)$/u.exec(line);
    expect(match, `${at} section ${name} line ${index + 1} must be a Markdown bullet`);
    return match[1];
  });
  stringArray(values, `${at} section ${name}`, { allowEmpty });
  return values;
}

function nullableParagraphSection(sections, name, at) {
  const value = sections[name];
  string(value, `${at} section ${name}`);
  if (value === "_None._") return null;
  return value.replace(/\s*\n\s*/gu, " ");
}

function readMarkdownRecords(kind, frontmatterFields, sectionNames, buildValue) {
  const dir = join(here, kind);
  const entries = readdirSync(dir).sort();
  const legacy = entries.filter((name) => name.endsWith(".json"));
  expect(legacy.length === 0, `${kind}/ contains legacy JSON records: ${legacy.join(", ")}; convert them to Markdown`);
  const names = entries.filter((name) => name.endsWith(".md"));
  if (names.length === 0) throw new Error(`${kind}/ contains no Markdown records`);
  return names.map((name) => {
    const path = checkedRecordPath(kind, name);
    const source = readFileSync(path, "utf8").replaceAll("\r\n", "\n");
    const match = /^---\n([\s\S]*?)\n---\n([\s\S]*)$/u.exec(source);
    expect(match, `${kind}/${name} must contain JSON-valued YAML frontmatter followed by Markdown sections`);
    const at = `${kind}/${name}`;
    const fields = parseJsonFrontmatter(match[1], at, frontmatterFields);
    const sections = parseSections(match[2], at, sectionNames);
    return { name, path, value: buildValue(fields, sections, at) };
  });
}

function readRuleRecords() {
  return readMarkdownRecords("rules", RULE_FRONTMATTER, RULE_SECTIONS, (fields, sections, at) => {
    const projection = {};
    const caveats = {};
    for (const [harness, label] of [["claude", "Claude"], ["pi", "Pi"]]) {
      const mechanism = sections[`${label} mechanism`];
      const enforcement = sections[`${label} enforcement`];
      const promptGuideline = sections[`${label} prompt guideline`];
      const harnessCaveats = sections[`${label} caveats`];
      const present = [mechanism, enforcement, promptGuideline, harnessCaveats].filter((value) => value !== undefined).length;
      expect(present === 0 || present === 4, `${at} must include all or none of the ${label} projection sections`);
      if (present === 0) continue;
      projection[harness] = {
        mechanism: paragraphSection(sections, `${label} mechanism`, at),
        enforcement: paragraphSection(sections, `${label} enforcement`, at),
        promptGuideline: nullableParagraphSection(sections, `${label} prompt guideline`, at),
      };
      caveats[harness] = listSection(sections, `${label} caveats`, at, { allowEmpty: true });
    }
    return {
      id: fields.id,
      title: fields.title,
      appliesTo: fields.applicability,
      ...(fields.divergenceReason ? { divergenceReason: fields.divergenceReason } : {}),
      invariant: paragraphSection(sections, "Invariant", at),
      sharedInstruction: paragraphSection(sections, "Shared instruction", at),
      projection,
      caveats,
      verifyRefs: listSection(sections, "Verify", at),
    };
  });
}

function readScenarioRecords() {
  return readMarkdownRecords("scenarios", SCENARIO_FRONTMATTER, SCENARIO_SECTIONS, (fields, sections, at) => {
    const answerKey = {};
    const claude = listSection(sections, "Claude answer key", at, { required: false });
    const pi = listSection(sections, "Pi answer key", at, { required: false });
    if (claude) answerKey.claude = claude;
    if (pi) answerKey.pi = pi;
    return {
      id: fields.id,
      title: fields.title,
      doctrineRefs: fields.doctrineRefs,
      applicability: fields.applicability,
      ...(fields.divergenceReason ? { divergenceReason: fields.divergenceReason } : {}),
      prompt: paragraphSection(sections, "Prompt", at),
      setup: listSection(sections, "Setup", at),
      expectedOutcome: paragraphSection(sections, "Expected outcome", at),
      evidence: listSection(sections, "Evidence", at),
      safety: listSection(sections, "Safety", at),
      cleanup: listSection(sections, "Cleanup", at, { allowEmpty: true }),
      answerKey,
    };
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
    : "You are an ordinary Pi worker exercising the installed canonical zmux dispatcher on native zmux. Complete each supplied terminal task directly and safely through the zmux tool. Bounded repository inspection may use Bash; do not shell out to zmux or raw tmux, bypass the Bash guard, create hidden jobs, or poll. Touch only doctrine-test state, inspect real state before asserting success, pin the intended session, keep focus unchanged unless explicitly asked, and report concise concrete evidence after each task.";
  const body = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .map((scenario) => `## ${scenario.id} · ${scenario.title}\n\n> ${scenario.prompt}`)
    .join("\n\n");
  const title = harness === "claude" ? "Claude zmux worker prompts" : "Pi zmux worker prompts";
  return `<!-- ${RENDERED_BANNER} -->\n\n# ${title}\n\nThe host sends the session contract once, then sends one scenario prompt at a time. Headings and answer keys stay host-side.\n\n## Session contract\n\n> ${contract}\n\n${body}\n`;
}

function renderAnswerKey(scenarios, harness) {
  const rows = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .map((scenario) => `### ${scenario.id} · ${scenario.title}\n\n- **Expected outcome:** ${scenario.expectedOutcome}\n- **${harness === "claude" ? "Claude" : "Pi"} mechanics:** ${scenario.answerKey[harness].join("; ")}\n- **Evidence:** ${scenario.evidence.join("; ")}\n- **Safety:** ${scenario.safety.join("; ")}\n- **Cleanup:** ${scenario.cleanup.length > 0 ? scenario.cleanup.join("; ") : "none"}${scenario.divergenceReason ? `\n- **Divergence:** ${scenario.divergenceReason}` : ""}`)
    .join("\n\n");
  return `<!-- ${RENDERED_BANNER} -->\n\n# ${harness === "claude" ? "Claude" : "Pi"} host answer key\n\nHost-only expected mechanics and evidence. Never send this file or its operation/verb hints to the worker.\n\n${rows}\n`;
}

function outputs(rules, scenarios, operations) {
  return new Map([
    ["skills/zmux/references/shared-doctrine.generated.md", renderClaudeReference(rules)],
    ["pi-zmux/src/generated/doctrine.ts", renderPiModule(rules)],
    ["pi-zmux/doctrine-manifest.generated.json", renderManifest(rules, scenarios, operations)],
    ["docs/reference/agent-doctrine-matrix.generated.md", renderMatrix(rules)],
  ]);
}

function renderTestingArtifact(name, scenarios) {
  const artifacts = {
    "claude-prompts": renderPrompts(scenarios, "claude"),
    "claude-answer-key": renderAnswerKey(scenarios, "claude"),
    "pi-prompts": renderPrompts(scenarios, "pi"),
    "pi-answer-key": renderAnswerKey(scenarios, "pi"),
  };
  expect(Object.hasOwn(artifacts, name), `unknown render artifact ${name}; choose one of: ${Object.keys(artifacts).join(", ")}`);
  return artifacts[name];
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
  if (stale.length > 0) throw new Error(`stale committed projection(s): ${stale.join(", ")}; run make gen-doctrine`);
  console.log(`agent-doctrine: ${rendered.size} committed projections current; Markdown records valid`);
}

export function loadDoctrine() {
  const rules = uniqueById(readRuleRecords().map(validateRule), "rule");
  const ruleIds = new Set(rules.map((rule) => rule.id));
  const scenarios = uniqueById(readScenarioRecords().map((record) => validateScenario(record, ruleIds)), "scenario");
  const operations = validatePiOperationMentions(rules, scenarios);
  return { rules, scenarios, operations };
}

function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && ["-h", "--help"].includes(args[0])) {
    console.log(USAGE);
    return;
  }
  const renderMode = args[0] === "--render" && args.length === 2;
  const projectionMode = args.length === 1 && ["--write", "--check"].includes(args[0]);
  if (!renderMode && !projectionMode) {
    usageError("choose --write, --check, or --render <artifact>");
    return;
  }
  try {
    const { rules, scenarios, operations } = loadDoctrine();
    if (renderMode) {
      process.stdout.write(renderTestingArtifact(args[1], scenarios));
      return;
    }
    const rendered = outputs(rules, scenarios, operations);
    if (args[0] === "--write") writeOutputs(rendered);
    else checkOutputs(rendered);
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) main();
