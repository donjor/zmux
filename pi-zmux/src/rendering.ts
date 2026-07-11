import type { ZmuxOperation } from "./operations.js";

export type ZmuxParams = {
  operation: string;
  target?: string;
  command?: string;
  cwd?: string;
  options?: Record<string, unknown>;
};

export type Destination = {
  workspace?: string;
  session?: string;
  tab?: string;
  paneLabel?: string;
  paneId?: string;
};

export type DisplayInput = {
  kind: "command" | "text" | "keys";
  value: string;
  length: number;
  lines: number;
  sensitive?: boolean;
};

export type DisplayLifecycle = {
  status?: "running" | "ready" | "done" | "failed" | "attention";
  evidence?: string;
  elapsedSeconds?: number;
  focusChanged?: boolean;
};

export type DisplayMetadata = {
  operation: string;
  verb: string;
  destination: Destination;
  input?: DisplayInput;
  lifecycle?: DisplayLifecycle;
  raw: {
    cwd: string;
    args?: string[];
    exitCode?: number | null;
    output?: string;
  };
};

export type ZmuxResultDetails = Record<string, unknown> & { display?: DisplayMetadata; failed?: boolean };

export type ThemeLike = {
  fg(color: string, text: string): string;
  bold(text: string): string;
  italic(text: string): string;
};

export type RenderResult = {
  content: Array<{ type: string; text?: string }>;
  details?: ZmuxResultDetails;
};

type DestinationKind = "none" | "workspace" | "session" | "tab" | "pane";
type InputKind = "none" | "command" | "text" | "keys";

type OperationDescriptor = {
  icon: string;
  verb: string;
  destination: DestinationKind;
  input: InputKind;
};

export const OPERATION_DESCRIPTORS: Record<ZmuxOperation, OperationDescriptor> = {
  current: { icon: "󰋼", verb: "current pane", destination: "none", input: "none" },
  tabs: { icon: "󰓩", verb: "list tabs", destination: "session", input: "none" },
  sessions: { icon: "", verb: "list sessions", destination: "workspace", input: "none" },
  panes: { icon: "󰏤", verb: "list panes", destination: "session", input: "none" },
  run: { icon: "󰆍", verb: "run command", destination: "tab", input: "command" },
  session_run: { icon: "󰆍", verb: "start session", destination: "session", input: "command" },
  session_kill: { icon: "󰅖", verb: "kill session", destination: "session", input: "none" },
  runtime_ensure: { icon: "󰆍", verb: "ensure runtime", destination: "tab", input: "command" },
  runtime_logs: { icon: "󰆍", verb: "read runtime", destination: "tab", input: "none" },
  runtime_stop: { icon: "󰓛", verb: "stop runtime", destination: "tab", input: "none" },
  tab_state: { icon: "󰓩", verb: "set tab state", destination: "tab", input: "none" },
  tab_peer: { icon: "󰘬", verb: "set peer state", destination: "tab", input: "none" },
  tab_status: { icon: "󰋼", verb: "tab status", destination: "tab", input: "none" },
  tab_inspect: { icon: "󰋼", verb: "inspect tab", destination: "tab", input: "none" },
  tab_label: { icon: "󰌕", verb: "label tab", destination: "tab", input: "none" },
  tab_move: { icon: "󰁔", verb: "move tab", destination: "tab", input: "none" },
  tab_place: { icon: "󰏤", verb: "place tab", destination: "tab", input: "none" },
  tab_kill: { icon: "󰅖", verb: "kill tab", destination: "tab", input: "none" },
  tab_focus: { icon: "󰍉", verb: "focus tab", destination: "tab", input: "none" },
  send_keys: { icon: "󰌌", verb: "send keys", destination: "tab", input: "keys" },
  type_text: { icon: "󰅐", verb: "type text", destination: "tab", input: "text" },
  peer_ensure: { icon: "󰘬", verb: "ensure peer", destination: "tab", input: "command" },
  peer_handoff: { icon: "󰘬", verb: "peer handoff", destination: "tab", input: "text" },
  pane_open: { icon: "󰆍", verb: "open pane", destination: "pane", input: "command" },
  pane_close: { icon: "󰅖", verb: "close pane", destination: "pane", input: "none" },
  pane_resize: { icon: "󰩨", verb: "resize pane", destination: "pane", input: "none" },
  pane_focus: { icon: "󰍉", verb: "focus pane", destination: "pane", input: "none" },
  pane_send_keys: { icon: "󰌌", verb: "send pane keys", destination: "pane", input: "keys" },
  pane_type: { icon: "󰅐", verb: "type into pane", destination: "pane", input: "text" },
  log: { icon: "󰆍", verb: "manage log", destination: "tab", input: "none" },
  snapshot: { icon: "󰄄", verb: "capture snapshot", destination: "none", input: "none" },
  wait: { icon: "󰔟", verb: "wait", destination: "tab", input: "none" },
  callback_watch: { icon: "󰑓", verb: "watch callback", destination: "tab", input: "none" },
  callback_list: { icon: "󰑓", verb: "list callbacks", destination: "none", input: "none" },
  callback_cancel: { icon: "󰅖", verb: "cancel callback", destination: "none", input: "none" },
  interactive_type: { icon: "󰆍", verb: "interactive command", destination: "tab", input: "command" },
  terminal_current: { icon: "", verb: "current terminal", destination: "none", input: "none" },
  zmux_reload: { icon: "󰑓", verb: "reload zmux", destination: "none", input: "none" },
  pi_reload: { icon: "󰑓", verb: "reload Pi", destination: "pane", input: "none" },
  pi_respawn: { icon: "󰑐", verb: "respawn Pi", destination: "pane", input: "command" },
};

const LARGE_TEXT_CHARACTERS = 1_500;
const LARGE_TEXT_LINES = 20;
const SENSITIVE_PATTERN = /(?:pass(?:word|phrase)?|secret|token|api[-_ ]?key|authorization)\s*[:=]/iu;

function descriptor(operation: string): OperationDescriptor {
  return OPERATION_DESCRIPTORS[operation as ZmuxOperation] ?? { icon: "󰆍", verb: operation || "zmux", destination: "none", input: "none" };
}

function stringOption(params: ZmuxParams, key: string): string | undefined {
  const value = params.options?.[key];
  return typeof value === "string" && value ? value : undefined;
}

function boolOption(params: ZmuxParams, key: string): boolean | undefined {
  const value = params.options?.[key];
  return typeof value === "boolean" ? value : undefined;
}

function numberOption(params: ZmuxParams, key: string): number | undefined {
  const value = params.options?.[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function arrayOption(params: ZmuxParams, key: string): string[] | undefined {
  const value = params.options?.[key];
  return Array.isArray(value) && value.every((item) => typeof item === "string") ? value : undefined;
}

function friendlyPane(value: string | undefined): Pick<Destination, "paneLabel" | "paneId"> {
  if (!value) return {};
  return value.startsWith("%") ? { paneId: value } : { paneLabel: value };
}

export function destinationFor(params: ZmuxParams): Destination {
  const spec = descriptor(params.operation);
  const session = stringOption(params, "session");
  const workspace = stringOption(params, "workspace");
  if (params.operation === "session_run") {
    return { workspace, session: params.target, tab: stringOption(params, "tab") };
  }
  if (params.operation === "tab_place" && stringOption(params, "action") === "pane") {
    return { session, tab: stringOption(params, "into"), paneLabel: params.target };
  }
  if (params.operation === "tab_label") {
    return { session, tab: params.target, ...friendlyPane(stringOption(params, "rawTarget")) };
  }
  if (params.operation === "pane_open") {
    const pane = friendlyPane(params.target);
    return { session, ...pane, paneId: stringOption(params, "rawTarget") ?? pane.paneId };
  }
  if (spec.destination === "workspace") return { workspace: params.target };
  if (spec.destination === "session") return { session: params.target ?? session };
  if (spec.destination === "tab") return { session, tab: params.target };
  if (spec.destination === "pane") return { session, ...friendlyPane(params.target) };
  return {};
}

function inputFor(params: ZmuxParams): DisplayInput | undefined {
  const kind = descriptor(params.operation).input;
  if (kind === "none") return undefined;
  let value: string | undefined;
  if (kind === "command") value = params.command;
  if (kind === "text") value = stringOption(params, "text");
  if (kind === "keys") value = arrayOption(params, "keys")?.join("  ");
  if (!value) return undefined;
  const sensitive = boolOption(params, "sensitive") === true || SENSITIVE_PATTERN.test(value);
  return { kind, value, length: value.length, lines: value.split("\n").length, sensitive };
}

function mergeDestination(base: Destination, parsed: Record<string, unknown> | undefined): Destination {
  if (!parsed) return base;
  const pick = (...keys: string[]) => {
    for (const key of keys) {
      const value = parsed[key];
      if (typeof value === "string" && value) return value;
    }
    return undefined;
  };
  const pane = pick("pane", "paneId", "paneID", "ID");
  return {
    workspace: base.workspace ?? pick("workspace", "Workspace"),
    session: base.session ?? pick("session", "Session", "sessionName"),
    tab: base.tab ?? pick("tab", "tabName", "windowName", "WindowName"),
    paneLabel: base.paneLabel ?? pick("paneLabel", "paneName"),
    paneId: base.paneId ?? (pane?.startsWith("%") ? pane : undefined),
  };
}

function parseObject(text: string | undefined): Record<string, unknown> | undefined {
  if (!text?.trim().startsWith("{")) return undefined;
  try {
    const value = JSON.parse(text);
    return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
  } catch {
    return undefined;
  }
}

function inferLifecycle(details: Record<string, unknown>, failed: boolean): DisplayLifecycle {
  const statusValue = details.status ?? details.turnState ?? details.cmdState;
  const status = failed
    ? "failed"
    : details.ready === true || statusValue === "ready"
      ? "ready"
      : statusValue === "running"
        ? "running"
        : statusValue === "attention"
          ? "attention"
          : "done";
  return {
    status,
    evidence: typeof details.readinessBasis === "string"
      ? details.readinessBasis
      : typeof details.evidenceBasis === "string"
        ? details.evidenceBasis
        : undefined,
    elapsedSeconds: typeof details.elapsedSeconds === "number" ? details.elapsedSeconds : undefined,
    focusChanged: typeof details.focus === "boolean" ? details.focus : undefined,
  };
}

export function buildDisplayMetadata(
  params: ZmuxParams,
  cwd: string,
  details: Record<string, unknown> = {},
  raw: { args?: string[]; exitCode?: number | null; output?: string } = {},
): DisplayMetadata {
  const output = raw.output;
  const parsed = parseObject(output);
  const detailArgs = Array.isArray(details.args) && details.args.every((item) => typeof item === "string") ? details.args as string[] : undefined;
  const detailExitCode = typeof details.exitCode === "number" || details.exitCode === null ? details.exitCode : undefined;
  const resolvedRaw = { cwd, ...raw, args: raw.args ?? detailArgs, exitCode: raw.exitCode ?? detailExitCode, output };
  const failed = details.failed === true || (typeof resolvedRaw.exitCode === "number" && resolvedRaw.exitCode !== 0);
  const lifecycle = inferLifecycle(details, failed);
  if (!failed && ["callback_watch", "peer_handoff"].includes(params.operation)) lifecycle.status = "running";
  if (!failed && params.operation === "runtime_ensure" && details.ready !== true) lifecycle.status = "running";
  return {
    operation: params.operation,
    verb: descriptor(params.operation).verb,
    destination: mergeDestination(destinationFor(params), parsed),
    input: inputFor(params),
    lifecycle,
    raw: resolvedRaw,
  };
}

export function withDisplayMetadata<T extends RenderResult>(
  result: T,
  params: ZmuxParams,
  cwd: string,
  raw: { args?: string[]; exitCode?: number | null; output?: string } = {},
): T {
  const details = result.details ?? {};
  return {
    ...result,
    details: { ...details, display: buildDisplayMetadata(params, cwd, details, raw) },
  };
}

function destinationTree(destination: Destination, theme: ThemeLike): string[] {
  const nodes: string[] = [];
  if (destination.workspace) nodes.push(`󱂬 ${destination.workspace}`);
  if (destination.session) nodes.push(` ${destination.session}`);
  if (destination.tab) nodes.push(`󰓩 ${destination.tab}`);
  if (destination.paneLabel) nodes.push(`󰏤 ${destination.paneLabel}`);
  else if (destination.paneId) nodes.push("󰏤 pane");
  return nodes.map((node, index) => {
    const prefix = index === 0 ? "" : `${"   ".repeat(index - 1)}└─ `;
    return theme.fg(index === nodes.length - 1 ? "accent" : "muted", `${prefix}${node}`);
  });
}

function renderInput(input: DisplayInput | undefined, expanded: boolean, theme: ThemeLike): string[] {
  if (!input) return [];
  if (input.sensitive) return [theme.italic(theme.fg("muted", "[sensitive input redacted]"))];
  const large = input.length > LARGE_TEXT_CHARACTERS || input.lines > LARGE_TEXT_LINES;
  let value = input.value;
  if (large && !expanded) {
    value = `${value.slice(0, 240).trimEnd()}…`;
  }
  const rendered = input.kind === "command"
    ? theme.fg("toolOutput", `$ ${value}`)
    : input.kind === "text"
      ? theme.italic(theme.fg("muted", value))
      : theme.fg("toolOutput", value);
  const lines = [rendered];
  if (large && !expanded) lines.push(theme.fg("muted", `󰅐 ${input.length.toLocaleString()} characters · ${input.lines} lines · Ctrl+O to show all`));
  return lines;
}

const OPTION_LABELS: Record<string, (value: unknown) => string | undefined> = {
  action: (value) => typeof value === "string" ? value : undefined,
  state: (value) => typeof value === "string" ? value : undefined,
  destination: (value) => typeof value === "string" ? `to ${value}` : undefined,
  direction: (value) => typeof value === "string" ? value : undefined,
  size: (value) => typeof value === "string" ? value : undefined,
  waitFor: (value) => typeof value === "string" ? `wait ${value}` : undefined,
  readiness: (value) => typeof value === "string" ? `ready ${value}` : undefined,
  waitForTurnState: (value) => typeof value === "string" ? `wait for ${value}` : undefined,
  idleSeconds: (value) => typeof value === "number" ? `idle ${value}s` : undefined,
  timeoutSeconds: (value) => typeof value === "number" ? `timeout ${value}s` : undefined,
  lines: (value) => typeof value === "number" ? `${value} lines` : undefined,
  focus: (value) => typeof value === "boolean" ? (value ? "focus moves" : "focus unchanged") : undefined,
  restart: (value) => value === true ? "restart" : undefined,
  detach: (value) => value === true ? "detached" : undefined,
  keep: (value) => value === true ? "kept" : undefined,
  force: (value) => value === true ? "force" : undefined,
  flat: (value) => value === true ? "flat" : undefined,
  clear: (value) => value === true ? "clear" : undefined,
  after: (value) => value === true ? "place after" : undefined,
  byVisibility: (value) => value === true ? "by visibility" : undefined,
  labelTab: (value) => value === true ? "label tab" : undefined,
  ansi: (value) => value === true ? "ANSI" : undefined,
  waitForExit: (value) => typeof value === "boolean" ? (value ? "wait for exit" : "do not wait for exit") : undefined,
  markPeerRunning: (value) => value === true ? "mark peer running" : undefined,
  deliverAs: (value) => typeof value === "string" ? `deliver ${value}` : undefined,
  triggerTurn: (value) => typeof value === "boolean" ? (value ? "trigger turn" : "no turn trigger") : undefined,
  scope: (value) => typeof value === "string" ? value : undefined,
  kind: (value) => typeof value === "string" ? value : undefined,
  role: (value) => typeof value === "string" ? `role ${value}` : undefined,
  topic: (value) => typeof value === "string" ? `topic ${value}` : undefined,
  ttl: (value) => typeof value === "string" ? `TTL ${value}` : undefined,
  hostTab: (value) => typeof value === "string" ? `host ${value}` : undefined,
  ifState: (value) => typeof value === "string" ? `if ${value}` : undefined,
  axis: (value) => typeof value === "string" ? value : undefined,
  panes: (value) => Array.isArray(value) ? `${value.length} panes` : undefined,
  noPng: (value) => value === true ? "no PNG" : undefined,
  json: (value) => value === true ? "JSON" : undefined,
  maxBytes: (value) => typeof value === "number" ? `max ${value.toLocaleString()} bytes` : undefined,
  out: (value) => typeof value === "string" ? `out ${value}` : undefined,
  id: (value) => typeof value === "string" ? `id ${value}` : undefined,
  retryAttempts: (value) => typeof value === "number" ? `${value} retries` : undefined,
  retryDelayMs: (value) => typeof value === "number" ? `retry delay ${value}ms` : undefined,
  delayMs: (value) => typeof value === "number" ? `delay ${value}ms` : undefined,
};

const EXPANDED_OPTION_SKIP = new Set(["session", "workspace", "tab", "pane", "into", "text", "keys", "sensitive"]);

function optionSummary(params: ZmuxParams, expanded: boolean): string[] {
  const parts: string[] = [];
  const commandCwd = stringOption(params, "commandCwd");
  if (params.cwd || commandCwd) parts.push(`cwd ${params.cwd ?? commandCwd}`);
  for (const [key, formatter] of Object.entries(OPTION_LABELS)) {
    const formatted = formatter(params.options?.[key]);
    if (formatted) parts.push(formatted);
  }
  const continuation = stringOption(params, "continuationPrompt");
  if (continuation) parts.push(`continuation ${continuation.length} chars`);
  if (["pane_open", "tab_place", "interactive_type"].includes(params.operation) && boolOption(params, "focus") === undefined) {
    parts.push("focus unchanged");
  }
  if (expanded) {
    for (const [key, value] of Object.entries(params.options ?? {})) {
      if (key in OPTION_LABELS || EXPANDED_OPTION_SKIP.has(key) || key === "continuationPrompt") continue;
      const rendered = /pass|secret|token|key/iu.test(key) ? "[redacted]" : typeof value === "string" ? value : JSON.stringify(value);
      parts.push(`${key} ${rendered}`);
    }
  }
  return parts;
}

function zmuxChip(theme: ThemeLike): string {
  return theme.fg("accent", theme.bold("┫ 󱂬 zmux ┣"));
}

function heading(params: ZmuxParams, theme: ThemeLike): string {
  const spec = descriptor(params.operation);
  const action = theme.fg("toolTitle", theme.bold(`${spec.icon} ${spec.verb}`));
  return `${zmuxChip(theme)}  ${action}`;
}

export function formatZmuxCall(params: ZmuxParams, expanded: boolean, theme: ThemeLike): string {
  const destination = destinationFor(params);
  const blocks: string[] = [heading(params, theme)];
  const tree = destinationTree(destination, theme);
  if (tree.length) blocks.push(tree.join("\n"));
  const input = renderInput(inputFor(params), expanded, theme);
  if (input.length) blocks.push(input.join("\n"));
  const options = optionSummary(params, expanded);
  if (options.length) blocks.push(theme.fg("muted", options.join(" · ")));
  return blocks.join("\n\n");
}

function textOutput(result: RenderResult): string {
  return result.content.filter((item) => item.type === "text" && typeof item.text === "string").map((item) => item.text).join("\n").trim();
}

function statusPresentation(metadata: DisplayMetadata, failed: boolean, partial: boolean): { glyph: string; label: string; color: string } {
  if (failed || metadata.lifecycle?.status === "failed") return { glyph: "✗", label: `${metadata.verb} failed`, color: "error" };
  if (partial || metadata.lifecycle?.status === "running") return { glyph: "◐", label: `${metadata.verb} running`, color: "warning" };
  if (metadata.lifecycle?.status === "ready") return { glyph: "↩", label: `${metadata.verb} ready`, color: "success" };
  if (metadata.lifecycle?.status === "attention") return { glyph: "●", label: `${metadata.verb} needs attention`, color: "warning" };
  return { glyph: "✓", label: `${metadata.verb} done`, color: "success" };
}

function collapsedEvidence(output: string, metadata: DisplayMetadata): string[] {
  const parsed = parseObject(output);
  const evidence: string[] = [];
  if (metadata.lifecycle?.evidence) evidence.push(metadata.lifecycle.evidence.replaceAll("-", " "));
  if (metadata.lifecycle?.elapsedSeconds !== undefined) evidence.push(`${metadata.lifecycle.elapsedSeconds}s`);
  if (metadata.lifecycle?.focusChanged !== undefined) evidence.push(metadata.lifecycle.focusChanged ? "focus moved" : "focus unchanged");
  if (parsed) {
    for (const key of ["status", "cmdState", "turnState", "lastExit", "message"] as const) {
      const value = parsed[key];
      if (typeof value === "string" || typeof value === "number") evidence.push(`${key} ${value}`);
    }
    return evidence;
  }
  const meaningful = output
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line && !/^zmux \S+ completed:/u.test(line));
  if (meaningful.length <= 3 && output.length <= 500) evidence.push(...meaningful);
  else if (meaningful.length) evidence.push(...meaningful.slice(-3));
  return [...new Set(evidence)].slice(0, 3);
}

function listTree(operation: string, output: string, parentDepth: number, theme: ThemeLike): string[] {
  const glyph = operation === "tabs" ? "󰓩" : operation === "sessions" ? "" : operation === "panes" ? "󰏤" : undefined;
  if (!glyph || parseObject(output)) return [];
  const lines = output.split("\n").map((line) => line.trim()).filter(Boolean).slice(0, 10);
  return lines.map((line, index) => {
    if (parentDepth === 0 && operation === "sessions") return theme.fg("toolOutput", `${glyph} ${line}`);
    const branch = index === lines.length - 1 ? "└─" : "├─";
    return theme.fg("toolOutput", `${"   ".repeat(Math.max(0, parentDepth - 1))}${branch} ${glyph} ${line}`);
  });
}

function sanitizeArgs(args: string[] | undefined, input: DisplayInput | undefined): string[] | undefined {
  if (!args || !input?.sensitive) return args;
  return args.map((arg) => arg === input.value ? "[redacted]" : arg);
}

function expandedMetadata(metadata: DisplayMetadata, details: ZmuxResultDetails, output: string): Array<[string, string]> {
  const rows: Array<[string, string | undefined]> = [
    ["operation", metadata.operation],
    ["workspace", metadata.destination.workspace],
    ["session", metadata.destination.session],
    ["tab", metadata.destination.tab],
    ["pane", metadata.destination.paneId],
    ["cwd", metadata.raw.cwd],
    ["exit", metadata.raw.exitCode === undefined ? undefined : String(metadata.raw.exitCode)],
    ["evidence", metadata.lifecycle?.evidence],
  ];
  const args = sanitizeArgs(metadata.raw.args, metadata.input);
  if (args?.length) rows.push(["argv", args.join(" ")]);
  for (const key of ["runtimeName", "configPath", "failureKind", "readinessBasis", "evidenceBasis", "callbackId", "id", "signal", "truncated"] as const) {
    const value = details[key];
    if (["string", "number", "boolean"].includes(typeof value)) rows.push([key, String(value)]);
  }
  const expandedOutput = metadata.raw.output ?? output;
  if (expandedOutput) rows.push(["output", metadata.input?.sensitive ? "[sensitive output redacted]" : expandedOutput]);
  return rows.filter((row): row is [string, string] => row[1] !== undefined && row[1] !== "");
}

export function formatZmuxResult(
  result: RenderResult,
  params: ZmuxParams,
  options: { expanded: boolean; isPartial: boolean },
  isError: boolean,
  theme: ThemeLike,
): string {
  const details = result.details ?? {};
  const output = textOutput(result);
  const metadata = details.display ?? buildDisplayMetadata(params, "", details, { output });
  const failed = isError || details.failed === true;
  const status = statusPresentation(metadata, failed, options.isPartial);
  const statusHeading = theme.fg(status.color, theme.bold(`${status.glyph} ${status.label}`));
  const blocks = [`${zmuxChip(theme)}  ${statusHeading}`];
  const tree = destinationTree(metadata.destination, theme);
  const listed = options.expanded ? [] : listTree(metadata.operation, output, tree.length, theme);
  if (tree.length || listed.length) blocks.push([...tree, ...listed].join("\n"));
  if (options.expanded) {
    const rows = expandedMetadata(metadata, details, output);
    const width = Math.max(9, ...rows.map(([key]) => key.length)) + 2;
    blocks.push(rows.map(([key, value]) => `${theme.fg("muted", key.padEnd(width))}${theme.fg("toolOutput", value)}`).join("\n"));
  } else if (!listed.length) {
    const evidence = collapsedEvidence(output, metadata);
    if (evidence.length) blocks.push(evidence.map((line) => theme.fg(failed ? "error" : "toolOutput", line)).join("\n"));
  }
  return blocks.filter(Boolean).join("\n\n");
}
