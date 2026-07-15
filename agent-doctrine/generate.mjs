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
and render shared plus Pi-only natural-prompt scenarios from agent-doctrine/scenarios/**/*.md.

Modes:
  --write  validate sources and rewrite committed runtime projections
  --check  validate sources and fail when a committed projection is stale; never writes
  --render <artifact> [--tier atomic|workflow|resilience] [--ids ZS-001,ZS-013]
           validate sources and print one maintainer live-test artifact to stdout;
           --ids stays caller-ordered and composes with an optional tier filter
  -h, --help

Committed projections (--write only):
  skills/zmux/references/shared-doctrine.generated.md
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
const SCENARIO_FRONTMATTER = new Set(["id", "title", "tier", "doctrineRefs", "sharedRefs", "applicability", "uatEligible", "legacyRefs", "knownFailureRefs", "divergenceReason"]);
const SCENARIO_SECTIONS = new Set(["Host setup", "Host perturbations", "Verdict", "Evidence", "Cleanup", "Claude answer key", "Pi answer key"]);
const TIERS = ["atomic", "workflow", "resilience"];
const VERDICT_LENSES = ["outcome", "orchestration", "responsiveness", "presentation", "cleanup"];
const INTERNAL_PROMPT_VOCABULARY = new Set(["callback", "observer", "lease", "inbox", "queue", "epoch", "tombstone", "generation", "receipt", "dispatcher"]);

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
    return { kind, name, path, value: buildValue(fields, sections, at) };
  });
}

function readRuleRecords(kind) {
  return readMarkdownRecords(kind, RULE_FRONTMATTER, RULE_SECTIONS, (fields, sections, at) => {
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

function parseScenarioSections(source, at) {
  const sections = {};
  const prompts = [];
  const heading = /^## (.+)$/gmu;
  // A fenced command payload can itself contain a Markdown-looking h2.
  const matches = [...source.matchAll(heading)].filter((match) => (source.slice(0, match.index).match(/^```/gmu) ?? []).length % 2 === 0);
  expect(matches.length > 0, `${at} must contain Markdown sections`);
  expect(source.slice(0, matches[0].index).trim() === "", `${at} must not contain content before its first section`);
  for (const [index, match] of matches.entries()) {
    const name = match[1].trim();
    const prompt = /^Prompt (\d+)$/u.exec(name);
    const start = match.index + match[0].length;
    const end = matches[index + 1]?.index ?? source.length;
    if (prompt) {
      const number = Number(prompt[1]);
      expect(number === prompts.length + 1, `${at} prompt headings must be sequential Prompt 1, Prompt 2, …`);
      let body = source.slice(start, end);
      expect(body.startsWith("\n\n"), `${at} ${name} must contain a blank line before its body`);
      body = body.slice(2);
      if (index + 1 < matches.length) {
        expect(body.endsWith("\n\n"), `${at} ${name} must be separated from the next section by a blank line`);
        body = body.slice(0, -2);
      } else if (body.endsWith("\n")) body = body.slice(0, -1);
      string(body, `${at} ${name}`);
      prompts.push(body);
      continue;
    }
    expect(SCENARIO_SECTIONS.has(name), `${at} contains unknown section ${name}`);
    expect(sections[name] === undefined, `${at} contains duplicate section ${name}`);
    sections[name] = source.slice(start, end).trim();
  }
  expect(prompts.length > 0, `${at} requires at least one numbered Prompt section`);
  return { sections, prompts };
}

function parseVerdict(sections, at) {
  const source = sections.Verdict;
  string(source, `${at} section Verdict`);
  const verdict = {};
  for (const [index, line] of source.split("\n").entries()) {
    const match = /^- ([a-z]+): (.+)$/u.exec(line);
    expect(match, `${at} section Verdict line ${index + 1} must be a named Markdown bullet`);
    const [, lens, value] = match;
    expect(VERDICT_LENSES.includes(lens), `${at} section Verdict contains invalid lens ${lens}`);
    expect(verdict[lens] === undefined, `${at} section Verdict contains duplicate lens ${lens}`);
    string(value, `${at} section Verdict.${lens}`);
    verdict[lens] = value;
  }
  expect(Object.keys(verdict).length === VERDICT_LENSES.length, `${at} section Verdict requires exactly ${VERDICT_LENSES.join(", ")}`);
  return verdict;
}

function readScenarioRecords(kind) {
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
    const fields = parseJsonFrontmatter(match[1], at, SCENARIO_FRONTMATTER);
    const { sections, prompts } = parseScenarioSections(match[2], at);
    const answerKey = {};
    const claude = listSection(sections, "Claude answer key", at, { required: false });
    const pi = listSection(sections, "Pi answer key", at, { required: false });
    if (claude) answerKey.claude = claude;
    if (pi) answerKey.pi = pi;
    return { kind, name, path, value: {
      id: fields.id, title: fields.title, tier: fields.tier, doctrineRefs: fields.doctrineRefs,
      ...(fields.sharedRefs ? { sharedRefs: fields.sharedRefs } : {}), applicability: fields.applicability,
      uatEligible: fields.uatEligible, legacyRefs: fields.legacyRefs ?? [], knownFailureRefs: fields.knownFailureRefs ?? [],
      ...(fields.divergenceReason ? { divergenceReason: fields.divergenceReason } : {}),
      prompts, hostSetup: listSection(sections, "Host setup", at),
      hostPerturbations: listSection(sections, "Host perturbations", at, { allowEmpty: true }),
      verdict: parseVerdict(sections, at), evidence: listSection(sections, "Evidence", at),
      cleanup: listSection(sections, "Cleanup", at, { allowEmpty: true }), answerKey,
    }};
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

function validateRule(record, { sharedOnly = false, piOnly = false } = {}) {
  const rule = record.value;
  const at = `${record.kind}/${record.name}`;
  expect(rule && typeof rule === "object" && !Array.isArray(rule), `${at} must contain an object`);
  expect(/^ZD-\d{3}$/.test(rule.id), `${at}.id must match ZD-###`);
  expect(record.name.startsWith(`${rule.id}-`), `${at} filename must start with ${rule.id}-`);
  for (const field of ["title", "invariant", "sharedInstruction"]) string(rule[field], `${at}.${field}`);
  const appliesTo = exactHarnesses(rule.appliesTo, `${at}.appliesTo`);
  if (sharedOnly) expect(appliesTo.length === HARNESSES.length, `${at}.applicability must be ["claude","pi"]`);
  if (piOnly) expect(appliesTo.length === 1 && appliesTo[0] === "pi", `${at}.applicability must be ["pi"]`);
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

function validateScenario(record, ruleIds, { idPattern = /^ZS-\d{3}$/, idShape = "ZS-###", sharedOnly = false, piOnly = false, sharedScenarioIds } = {}) {
  const scenario = record.value;
  const at = `${record.kind}/${record.name}`;
  expect(scenario && typeof scenario === "object" && !Array.isArray(scenario), `${at} must contain an object`);
  expect(idPattern.test(scenario.id), `${at}.id must match ${idShape}`);
  expect(record.name === `${scenario.id}.md` || record.name.startsWith(`${scenario.id}-`), `${at} filename must be ${scenario.id}.md or start with ${scenario.id}-`);
  for (const field of ["title"]) string(scenario[field], `${at}.${field}`);
  expect(TIERS.includes(scenario.tier), `${at}.tier must be one of: ${TIERS.join(", ")}`);
  expect(typeof scenario.uatEligible === "boolean", `${at}.uatEligible must be a boolean`);
  if (scenario.tier === "resilience") expect(!scenario.uatEligible, `${at}.uatEligible must be false for resilience scenarios`);
  stringArray(scenario.prompts, `${at}.prompts`);
  if (scenario.tier === "atomic") expect(scenario.prompts.length === 1, `${at} atomic scenarios require exactly one prompt`);
  if (scenario.tier === "workflow") expect(scenario.prompts.length >= 2, `${at} workflow scenarios require at least two prompts`);
  if (scenario.tier === "atomic") expect(scenario.hostPerturbations.length === 0, `${at} atomic scenarios require _None._ Host perturbations`);
  if (scenario.tier === "resilience") expect(scenario.hostPerturbations.length > 0, `${at} resilience scenarios require Host perturbations`);
  stringArray(scenario.legacyRefs, `${at}.legacyRefs`, { allowEmpty: true });
  stringArray(scenario.knownFailureRefs, `${at}.knownFailureRefs`, { allowEmpty: true });
  stringArray(scenario.doctrineRefs, `${at}.doctrineRefs`);
  for (const ref of scenario.doctrineRefs) expect(ruleIds.has(ref), `${at}.doctrineRefs references missing ${ref}`);
  if (sharedOnly) expect(scenario.sharedRefs === undefined, `${at}.sharedRefs belongs only on Pi-specific scenarios`);
  if (scenario.sharedRefs !== undefined) {
    stringArray(scenario.sharedRefs, `${at}.sharedRefs`);
    expect(sharedScenarioIds instanceof Set, `${at}.sharedRefs cannot be validated without shared scenarios`);
    for (const ref of scenario.sharedRefs) expect(sharedScenarioIds.has(ref), `${at}.sharedRefs references missing ${ref}`);
  }
  const applicability = exactHarnesses(scenario.applicability, `${at}.applicability`);
  if (sharedOnly) expect(applicability.length === HARNESSES.length, `${at}.applicability must be ["claude","pi"]`);
  if (piOnly) expect(applicability.length === 1 && applicability[0] === "pi", `${at}.applicability must be ["pi"]`);
  if (applicability.length !== HARNESSES.length) string(scenario.divergenceReason, `${at}.divergenceReason`);
  for (const field of ["hostSetup", "hostPerturbations", "evidence", "cleanup"]) {
    stringArray(scenario[field], `${at}.${field}`, { allowEmpty: field === "hostPerturbations" || field === "cleanup" });
  }
  expect(scenario.verdict && typeof scenario.verdict === "object", `${at}.verdict must be an object`);
  expect(Object.keys(scenario.verdict).length === VERDICT_LENSES.length, `${at}.verdict must contain five lenses`);
  for (const lens of VERDICT_LENSES) string(scenario.verdict[lens], `${at}.verdict.${lens}`);
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

function validateFrozenPiManifestOperations(operations) {
  const path = join(root, "pi-zmux/doctrine-manifest.generated.json");
  let manifest;
  try { manifest = JSON.parse(readFileSync(path, "utf8")); }
  catch (error) { throw new Error(`invalid frozen Pi doctrine manifest: ${error instanceof Error ? error.message : String(error)}`); }
  const projected = manifest?.dispatcherOperations;
  expect(Array.isArray(projected) && projected.every((operation) => typeof operation === "string"), "frozen Pi doctrine manifest dispatcherOperations must be a string array");
  expect(JSON.stringify([...projected].sort()) === JSON.stringify(operations), "frozen Pi doctrine manifest dispatcherOperations drift from pi-zmux/src/operations.ts");
}

function unfencedMarkdown(value) {
  let fenced = false;
  const outside = [];
  for (const line of value.split("\n")) {
    if (/^```/u.test(line)) { fenced = !fenced; continue; }
    if (!fenced) outside.push(line);
  }
  return outside.join("\n");
}

function validatePromptLeakage(scenarios) {
  for (const scenario of scenarios) for (const [index, prompt] of scenario.prompts.entries()) {
    const text = unfencedMarkdown(prompt);
    const snake = /\b[a-z]+_[a-z0-9_]+\b/gu.exec(text);
    expect(!snake, `${scenario.id} Prompt ${index + 1} leaks snake_case operation ${snake?.[0]}`);
    for (const word of INTERNAL_PROMPT_VOCABULARY) {
      expect(!new RegExp(`\\b${word}\\b`, "iu").test(text), `${scenario.id} Prompt ${index + 1} leaks internal vocabulary ${word}`);
    }
    // Snake-case operation identifiers are internal. Single-word operation names such
    // as run, wait, log, current, or snapshot remain valid ordinary user language.
  }
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
  const turns = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .flatMap((scenario) => scenario.prompts.map((prompt, index) => ({ id: scenario.id, number: index + 1, prompt })));
  // A single selected turn is exact copy/paste worker input. Multi-turn/tier bundles
  // carry host-only HTML boundaries so the conductor can send each body separately.
  if (turns.length === 1) return turns[0].prompt;
  return turns.map(({ id, number, prompt }) => `<!-- BEGIN HOST TURN ${id} Prompt ${number} -->\n${prompt}\n<!-- END HOST TURN ${id} Prompt ${number} -->`).join("\n\n");
}

function renderAnswerKey(scenarios, harness) {
  const rows = scenarios
    .filter((scenario) => scenario.applicability.includes(harness))
    .map((scenario) => `### ${scenario.id} · ${scenario.title}\n\n- **Host setup:** ${scenario.hostSetup.join("; ")}\n- **Host perturbations:** ${scenario.hostPerturbations.length ? scenario.hostPerturbations.join("; ") : "none"}\n${VERDICT_LENSES.map((lens) => `- **${lens}:** ${scenario.verdict[lens]}`).join("\n")}\n- **Evidence:** ${scenario.evidence.join("; ")}\n- **Cleanup:** ${scenario.cleanup.length ? scenario.cleanup.join("; ") : "none"}\n- **${harness === "claude" ? "Claude" : "Pi"} mechanics:** ${scenario.answerKey[harness].join("; ")}${scenario.divergenceReason ? `\n- **Divergence:** ${scenario.divergenceReason}` : ""}`)
    .join("\n\n");
  return `<!-- ${RENDERED_BANNER} -->\n\n# ${harness === "claude" ? "Claude" : "Pi"} host answer key\n\nHost-only expected mechanics and evidence. Never send this file or its operation/verb hints to the worker.\n\n${rows}\n`;
}

function outputs(rules) {
  return new Map([
    ["skills/zmux/references/shared-doctrine.generated.md", renderClaudeReference(rules)],
    ["docs/reference/agent-doctrine-matrix.generated.md", renderMatrix(rules)],
  ]);
}

function scenarioOrder(left, right) {
  const tier = TIERS.indexOf(left.tier) - TIERS.indexOf(right.tier);
  if (tier) return tier;
  const shared = (left.id.startsWith("PZ-") ? 1 : 0) - (right.id.startsWith("PZ-") ? 1 : 0);
  if (shared) return shared;
  return Number(left.id.slice(3)) - Number(right.id.slice(3));
}

function selectScenarios(records, ids, tier, harness) {
  const applicable = records.filter((scenario) => scenario.applicability.includes(harness));
  if (ids.length === 0) return applicable.filter((scenario) => !tier || scenario.tier === tier).sort(scenarioOrder);
  const byId = new Map(applicable.map((scenario) => [scenario.id, scenario]));
  const unique = [...new Set(ids)];
  expect(unique.length === ids.length, "--ids contains duplicates");
  return ids.map((id) => {
    const scenario = byId.get(id);
    expect(scenario, `scenario ${id} is unknown or not applicable to ${harness}`);
    expect(!tier || scenario.tier === tier, `scenario ${id} is not in tier ${tier}`);
    return scenario;
  });
}

function renderTestingArtifact(name, scenarios, ids = [], tier) {
  const specs = {
    "claude-prompts": ["claude", renderPrompts],
    "claude-answer-key": ["claude", renderAnswerKey],
    "pi-prompts": ["pi", renderPrompts],
    "pi-answer-key": ["pi", renderAnswerKey],
  };
  expect(Object.hasOwn(specs, name), `unknown render artifact ${name}; choose one of: ${Object.keys(specs).join(", ")}`);
  const [harness, renderer] = specs[name];
  return renderer(selectScenarios(scenarios, ids, tier, harness), harness);
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
  const rules = uniqueById(
    readRuleRecords("rules/shared").map((record) => validateRule(record, { sharedOnly: true })),
    "rule",
  );
  const ruleIds = new Set(rules.map((rule) => rule.id));
  const scenarios = uniqueById(
    readScenarioRecords("scenarios/shared").map((record) => validateScenario(record, ruleIds, { sharedOnly: true })),
    "shared scenario",
  );
  // Shared rules/scenarios keep both harness projections; this still validates their Pi answer keys.
  const operations = validatePiOperationMentions(rules, scenarios);
  validateFrozenPiManifestOperations(operations);
  validatePromptLeakage(scenarios);
  return { rules, scenarios };
}

function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && ["-h", "--help"].includes(args[0])) { console.log(USAGE); return; }
  const projectionMode = args.length === 1 && ["--write", "--check"].includes(args[0]);
  const renderMode = args[0] === "--render" && typeof args[1] === "string";
  if (!renderMode && !projectionMode) { usageError("choose --write, --check, or --render <artifact>"); return; }
  let ids = [];
  let tier;
  if (renderMode) {
    for (let index = 2; index < args.length; index += 2) {
      const option = args[index]; const value = args[index + 1];
      if (!value || !["--ids", "--tier"].includes(option)) { usageError("--render accepts only --ids and --tier values"); return; }
      if (option === "--ids") { if (ids.length) { usageError("--ids may be supplied once"); return; } ids = value.split(",").map((id) => id.trim()).filter(Boolean); }
      else { if (tier !== undefined || !TIERS.includes(value)) { usageError(`--tier must be one of: ${TIERS.join(", ")}`); return; } tier = value; }
    }
    if (args.length % 2 !== 0) { usageError("--render options require values"); return; }
  }
  try {
    const { rules, scenarios } = loadDoctrine();
    if (renderMode) {
      expect(!args.includes("--ids") || ids.length > 0, "--ids requires at least one scenario id");
      process.stdout.write(renderTestingArtifact(args[1], scenarios, ids, tier));
      return;
    }
    const rendered = outputs(rules);
    if (args[0] === "--write") writeOutputs(rendered); else checkOutputs(rendered);
  } catch (error) { fail(error instanceof Error ? error.message : String(error)); }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) main();
