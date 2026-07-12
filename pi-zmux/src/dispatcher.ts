import type { AgentToolUpdateCallback, ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Text } from "@earendil-works/pi-tui";
import { resolve } from "node:path";
import { Type } from "typebox";
import { loadConfig, mergeRuntimeConfig, type RuntimeConfig } from "./config.js";
import { runTmux, runZmux } from "./exec.js";
import { interactiveType } from "./interactive.js";
import { isZmuxOperation, ZMUX_OPERATIONS, type ZmuxOperation } from "./operations.js";
import {
  formatZmuxCall,
  formatZmuxResult,
  TIMEOUT_ERROR_PREFIX,
  withDisplayMetadata,
  type RenderResult as ZmuxRenderResult,
  type ZmuxParams,
} from "./rendering.js";
import { rejectHeadlessAgentPrintMode, shouldWaitForExit } from "./safety.js";
import { summarizeWaitOutput } from "./wait-summary.js";
import {
  cancelCallback,
  listCallbacks,
  listRecentCallbackCompletions,
  startPeerHandoff,
  startWatchCallback,
  type CallbackActivitySink,
} from "./zmux/callbacks.js";
import { resizePane } from "./zmux/panes.js";
import { zmuxRunSafetyWarnings } from "./zmux/sessions.js";
import { schedulePiReload, schedulePiRespawn } from "./zmux/pi-lifecycle.js";

export { ZMUX_OPERATIONS } from "./operations.js";

type Operation = ZmuxOperation;

const paramsSchema = Type.Object(
  {
    operation: Type.String({
      description: `Required zmux action. One of: ${ZMUX_OPERATIONS.join(", ")}. Prefer the documented operation names.`,
    }),
    target: Type.Optional(Type.String({ description: "Primary tab/session/pane/runtime target, depending on operation." })),
    command: Type.Optional(Type.String({ description: "Shell command for run/session_run/runtime_ensure/pane_open/interactive_type." })),
    cwd: Type.Optional(Type.String({ description: "Working directory for the zmux CLI process; defaults to Pi cwd." })),
    options: Type.Optional(
      Type.Record(Type.String(), Type.Any(), {
        description: "Operation-specific options: tab/session, lines, waitFor/readiness, waitForExit, idleSeconds, timeoutSeconds, focus, state/action, direction/size, keys/text, destination/workspace, restart, deliverAs/triggerTurn, continuationPrompt/retryAttempts, rawTarget.",
      }),
    ),
  },
  { additionalProperties: false },
);

function optString(options: Record<string, unknown>, key: string): string | undefined {
  const value = options[key];
  if (value === undefined) return undefined;
  if (typeof value !== "string") throw new Error(`options.${key} must be a string`);
  return value;
}

function optNumber(options: Record<string, unknown>, key: string): number | undefined {
  const value = options[key];
  if (value === undefined) return undefined;
  if (typeof value !== "number" || !Number.isFinite(value)) throw new Error(`options.${key} must be a finite number`);
  return value;
}

function optBool(options: Record<string, unknown>, key: string): boolean | undefined {
  const value = options[key];
  if (value === undefined) return undefined;
  if (typeof value !== "boolean") throw new Error(`options.${key} must be a boolean`);
  return value;
}

function optDeliverAs(options: Record<string, unknown>): "steer" | "followUp" | "nextTurn" | undefined {
  const value = optString(options, "deliverAs");
  if (value === undefined) return undefined;
  if (value !== "steer" && value !== "followUp" && value !== "nextTurn") {
    throw new Error("options.deliverAs must be one of: steer, followUp, nextTurn");
  }
  return value;
}

function optStringArray(options: Record<string, unknown>, key: string): string[] | undefined {
  const value = options[key];
  if (value === undefined) return undefined;
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) {
    throw new Error(`options.${key} must be an array of strings`);
  }
  return value;
}

function requireTarget(params: ZmuxParams, noun = "target"): string {
  if (!params.target?.trim()) throw new Error(`${noun} is required for operation ${params.operation}`);
  return params.target.trim();
}

function requireCommand(params: ZmuxParams): string {
  if (!params.command?.trim()) throw new Error(`command is required for operation ${params.operation}`);
  return params.command;
}

function pushOpt(args: string[], flag: string, value: string | number | undefined): void {
  if (value !== undefined && value !== "") args.push(flag, String(value));
}

function pushSession(args: string[], session?: string): void {
  if (session) args.push("-s", session);
}

function requireChoice(value: string, label: string, choices: readonly string[]): string {
  if (!choices.includes(value)) throw new Error(`${label} must be one of: ${choices.join(", ")} (got ${value})`);
  return value;
}

function cwdFor(defaultCwd: string, params: ZmuxParams): string {
  return params.cwd || defaultCwd;
}

function timeoutMs(options: Record<string, unknown>, fallbackSeconds = 30): number {
  return (optNumber(options, "timeoutSeconds") ?? fallbackSeconds) * 1000;
}

function buildArgs(params: ZmuxParams): string[] {
  if (!isZmuxOperation(params.operation)) throw new Error(`unknown zmux operation ${params.operation}`);
  const options = params.options ?? {};
  const target = params.target;
  const action = optString(options, "action");
  const session = optString(options, "session");
  const lines = optNumber(options, "lines");
  switch (params.operation) {
    case "current":
      return ["pane", "current", "--json"];
    case "tabs": {
      const args = ["tabs"];
      pushSession(args, session);
      return args;
    }
    case "sessions": {
      const args = ["ls"];
      if (optBool(options, "flat")) args.push("-s");
      if (target) args.push(target);
      return args;
    }
    case "panes": {
      const args = ["pane", "list"];
      if (session) args.push("--session", "--target", session);
      return args;
    }
    case "run": {
      const detach = optBool(options, "detach");
      const waitForExit = optBool(options, "waitForExit");
      const focus = optBool(options, "focus");
      const state = optString(options, "state") ?? optString(options, "action");
      if (state !== undefined) throw new Error("run lifecycle is automatic; use tab_state for explicit lifecycle mutation");
      if (focus === true) throw new Error("run does not accept options.focus=true; omit it for normal creation or call tab_focus explicitly");
      if (detach === true && waitForExit === true) throw new Error("options.detach=true and options.waitForExit=true are contradictory for run");
      if (detach === false && waitForExit === false) throw new Error("options.detach=false and options.waitForExit=false are contradictory for run");

      const args = ["run", "--command", requireCommand(params)];
      pushOpt(args, "-n", optString(options, "tab") ?? target);
      pushSession(args, session);
      pushOpt(args, "-T", optNumber(options, "timeoutSeconds"));
      pushOpt(args, "--lines", lines);
      if (focus === false) args.push("--no-focus");
      if (detach === true || waitForExit === false) args.push("-d");
      if (optBool(options, "keep")) args.push("--keep");
      pushOpt(args, "--scope", optString(options, "scope"));
      return args;
    }
    case "session_run": {
      const sessionName = requireTarget(params, "session name");
      const tab = optString(options, "tab");
      if (!tab) throw new Error("options.tab is required for session_run");
      const args = ["session", "run", sessionName, "-n", tab];
      pushOpt(args, "--workspace", optString(options, "workspace"));
      pushOpt(args, "--cwd", optString(options, "commandCwd") ?? params.cwd);
      args.push("--", "bash", "-lc", requireCommand(params));
      return args;
    }
    case "session_kill":
      return ["session", "kill", requireTarget(params, "session name")];
    case "runtime_ensure": {
      const tab = optString(options, "tab") ?? target;
      if (!tab) throw new Error("target or options.tab is required for runtime_ensure");
      const args = ["run", "--command", requireCommand(params), "-n", tab, "-d", "--keep", "--scope", optString(options, "kind") ?? "daemon"];
      pushSession(args, session);
      return args;
    }
    case "runtime_logs":
      return buildWatchArgs(requireTarget(params, "runtime tab"), options);
    case "runtime_stop":
      return ["send", requireTarget(params, "runtime tab"), "C-c", ...(session ? ["-s", session] : [])];
    case "tab_state": {
      const state = optString(options, "state") ?? action;
      if (!state) throw new Error("options.state or options.action is required for tab_state");
      requireChoice(state, "tab_state state", ["attention", "failed", "running", "ready", "done", "clear"]);
      const args = ["tab", "state", state];
      if (target) args.push(target);
      pushOpt(args, "--target", optString(options, "rawTarget"));
      pushOpt(args, "--source", optString(options, "source"));
      pushOpt(args, "--msg", optString(options, "message"));
      pushOpt(args, "--if-state", optString(options, "ifState"));
      if (optBool(options, "byVisibility")) args.push("--by-visibility");
      pushSession(args, session);
      return args;
    }
    case "tab_peer": {
      if (!action) throw new Error("options.action is required for tab_peer");
      requireChoice(action, "tab_peer action", ["start", "running", "ready", "waiting", "attention", "failed", "consumed", "park", "keep", "clear-keep"]);
      const args = ["tab", "peer", action];
      if (target) args.push(target);
      for (const [key, flag] of [["role", "--role"], ["hostTab", "--host-tab"], ["hostPane", "--host-pane"], ["topic", "--topic"], ["ttl", "--ttl"], ["source", "--source"], ["message", "--msg"]] as const) {
        pushOpt(args, flag, optString(options, key));
      }
      pushSession(args, session);
      return args;
    }
    case "tab_status": {
      const args = ["tab", "status", requireTarget(params, "tab"), "--json"];
      pushSession(args, session);
      return args;
    }
    case "tab_inspect": {
      const args = ["tab", "inspect", requireTarget(params, "tab")];
      pushOpt(args, "--lines", lines);
      pushSession(args, session);
      return args;
    }
    case "tab_label": {
      const args = ["tab", "label"];
      pushOpt(args, "--target", optString(options, "rawTarget"));
      if (optBool(options, "clear")) args.push("--clear");
      if (target) args.push(target);
      return args;
    }
    case "tab_move": {
      const destination = optString(options, "destination");
      if (!destination) throw new Error("options.destination is required for tab_move");
      const args = ["tab", "move", requireTarget(params, "tab"), destination];
      if (optBool(options, "force")) args.push("--force");
      pushSession(args, session);
      return args;
    }
    case "tab_place": {
      if (!action) throw new Error("options.action is required for tab_place (pane/full/hide/show)");
      requireChoice(action, "tab_place action", ["pane", "full", "hide", "show"]);
      const pane = optString(options, "pane");
      const into = optString(options, "into");
      const direction = optString(options, "direction");
      const size = optString(options, "size");
      const after = optBool(options, "after");
      const focus = optBool(options, "focus");
      if (target && pane) throw new Error("target and options.pane cannot be combined");
      if (action === "pane" && !target) throw new Error("target is required for tab_place pane");
      if (action === "show" && !target && !pane) throw new Error("target or options.pane is required for tab_place show");
      if (action !== "pane" && into) throw new Error(`options.into is not valid for tab_place ${action}`);
      if (action !== "pane" && direction) throw new Error(`options.direction is not valid for tab_place ${action}`);
      if (action !== "pane" && size) throw new Error(`options.size is not valid for tab_place ${action}`);
      if (action !== "full" && after) throw new Error(`options.after is not valid for tab_place ${action}`);
      if (action !== "pane" && action !== "show" && focus) throw new Error(`options.focus is not valid for tab_place ${action}`);
      const args = ["tab", action];
      if (target) args.push(target);
      pushOpt(args, "--session", session);
      pushOpt(args, "--into", into);

      if (direction) {
        requireChoice(direction, "tab_place direction", ["right", "left", "up", "down"]);
        args.push(`--${direction}`);
      }
      pushOpt(args, "--size", size);
      pushOpt(args, "--pane", pane);
      if (after) args.push("--after");
      if (focus) args.push("--focus");
      return args;
    }
    case "tab_kill": {
      const args = ["tab", "kill", requireTarget(params, "tab")];
      pushSession(args, session);
      return args;
    }
    case "tab_focus":
      return ["tab", "focus", requireTarget(params, "tab")];
    case "send_keys": {
      const keys = optStringArray(options, "keys");
      if (!keys?.length) throw new Error("options.keys is required for send_keys");
      const args = ["send", requireTarget(params, "tab"), ...keys];
      pushSession(args, session);
      return args;
    }
    case "type_text": {
      const text = optString(options, "text");
      if (!text) throw new Error("options.text is required for type_text");
      const args = ["type", requireTarget(params, "tab"), text];
      pushSession(args, session);
      if (optBool(options, "markPeerRunning")) args.push("--mark-peer-running");
      pushOpt(args, "--wait-turn", optString(options, "waitForTurnState"));
      pushOpt(args, "-T", optNumber(options, "timeoutSeconds"));
      pushOpt(args, "--lines", lines);
      pushOpt(args, "--source", optString(options, "source"));
      pushOpt(args, "--msg", optString(options, "message"));
      return args;
    }
    case "peer_ensure": {
      const args = ["tab", "peer", "ensure", requireTarget(params, "peer tab")];
      if (params.command) args.push("--command", params.command);
      pushSession(args, session);
      for (const [key, flag] of [["role", "--role"], ["hostTab", "--host-tab"], ["hostPane", "--host-pane"], ["topic", "--topic"], ["source", "--source"], ["message", "--msg"], ["readiness", "--readiness"], ["waitForTurnState", "--wait-turn"]] as const) {
        pushOpt(args, flag, optString(options, key));
      }
      pushOpt(args, "-T", optNumber(options, "timeoutSeconds"));
      pushOpt(args, "--lines", lines);
      if (optBool(options, "restart")) args.push("--restart");
      return args;
    }
    case "pane_open": {
      const args = ["pane", "open", requireTarget(params, "pane name"), "--cwd", params.cwd || "."];
      pushOpt(args, "--target", optString(options, "rawTarget"));
      const direction = optString(options, "direction") ?? "right";
      requireChoice(direction, "pane_open direction", ["right", "left", "down", "up"]);
      args.push(({ right: "-r", left: "-l", down: "-d", up: "-u" } as Record<string, string>)[direction]);
      pushOpt(args, "", optString(options, "size"));
      if (optBool(options, "labelTab")) args.push("--label-tab");
      if (!optBool(options, "focus")) args.push("--no-focus");
      args.push("--", "bash", "-lc", requireCommand(params));
      return args.filter((arg) => arg !== "");
    }
    case "pane_close":
      return ["pane", "close", requireTarget(params, "pane")];
    case "pane_resize": {
      const size = optString(options, "size");
      if (!size) throw new Error("options.size is required for pane_resize");
      const axis = optString(options, "axis") ?? "width";
      return ["pane", "resize", requireTarget(params, "pane"), axis === "height" ? "--height" : "--width", size];
    }
    case "pane_focus":
      return ["pane", "focus", requireTarget(params, "pane")];
    case "pane_send_keys": {
      const keys = optStringArray(options, "keys");
      if (!keys?.length) throw new Error("options.keys is required for pane_send_keys");
      return ["tmux-send-keys", requireTarget(params, "pane"), ...keys];
    }
    case "pane_type": {
      const text = optString(options, "text");
      if (!text) throw new Error("options.text is required for pane_type");
      return ["tmux-type", requireTarget(params, "pane"), text];
    }
    case "log": {
      if (!action) throw new Error("options.action is required for log");
      requireChoice(action, "log action", ["start", "tail", "status", "stop"]);
      const ansi = optBool(options, "ansi");
      const maxBytes = optNumber(options, "maxBytes");
      if (action !== "status" && !target) throw new Error(`target is required for log ${action}`);
      if (action === "status" && target) throw new Error("target is not valid for log status");
      if (action === "status" && session) throw new Error("options.session is not valid for log status");
      if (action !== "start" && ansi) throw new Error(`options.ansi is not valid for log ${action}`);
      if (action !== "start" && maxBytes !== undefined) throw new Error(`options.maxBytes is not valid for log ${action}`);
      if (action !== "tail" && lines !== undefined) throw new Error(`options.lines is not valid for log ${action}`);
      const args = ["log", action];
      if (target) args.push(target);
      if (ansi) args.push("--ansi");
      pushOpt(args, "--max-bytes", maxBytes);
      pushOpt(args, "-n", lines);
      pushSession(args, session);
      return args;
    }
    case "snapshot": {
      const args = ["snapshot"];
      if (optBool(options, "noPng")) args.push("--no-png");
      for (const pane of optStringArray(options, "panes") ?? []) args.push("--pane", pane);
      pushOpt(args, "--lines", lines);
      pushOpt(args, "--out", optString(options, "out"));
      if (optBool(options, "json")) args.push("--json");
      return args;
    }
    case "wait":
      return buildWaitArgs(requireTarget(params, "tab"), options);
    case "callback_watch":
      return buildWaitArgs(requireTarget(params, "tab"), options);
    case "callback_list":
    case "callback_cancel":
    case "peer_handoff":
      return [];
    case "interactive_type": {
      const tab = target || "admin";
      return ["run", "--command", requireCommand(params), "-n", tab, "--keep", "--scope", "agent-shell"];
    }
    case "terminal_current":
      return ["terminal", "current", "--json"];
    case "zmux_reload":
      return ["reload"];
    case "pi_reload":
    case "pi_respawn":
      return [];
  }
}

function buildWatchArgs(target: string, options: Record<string, unknown>): string[] {
  const args = ["watch", target];
  pushOpt(args, "-l", optNumber(options, "lines") ?? 120);
  const waitFor = optString(options, "waitFor");
  const idleSeconds = optNumber(options, "idleSeconds");
  if (waitFor && idleSeconds !== undefined) throw new Error("waitFor and idleSeconds cannot be combined");
  if (waitFor) args.push("--until", waitFor);
  if (idleSeconds !== undefined) args.push("--idle", String(idleSeconds));
  if (waitFor || idleSeconds !== undefined) pushOpt(args, "-T", optNumber(options, "timeoutSeconds") ?? 10);
  pushSession(args, optString(options, "session"));
  return args;
}

function buildWaitArgs(target: string, options: Record<string, unknown>): string[] {
  const args = ["wait", target];
  const waitFor = optString(options, "waitFor");
  const idleSeconds = optNumber(options, "idleSeconds");
  if (waitFor && idleSeconds !== undefined) throw new Error("waitFor and idleSeconds cannot be combined");
  if (!waitFor && idleSeconds === undefined) throw new Error("wait requires waitFor or idleSeconds");
  if (waitFor?.startsWith("output:")) throw new Error('options.waitFor is the output regex only; omit the "output:" prefix');
  args.push("--for", waitFor ? `output:${waitFor}` : `idle:${idleSeconds}`);
  pushOpt(args, "-l", optNumber(options, "lines") ?? 160);
  pushOpt(args, "-T", optNumber(options, "timeoutSeconds") ?? 300);
  args.push("--json");
  pushSession(args, optString(options, "session"));
  return args;
}

function formatResult(operation: string, args: string[], stdout: string, stderr: string): string {
  const body = [stdout.trim(), stderr.trim()].filter(Boolean).join("\n");
  return body || `zmux ${operation} completed: ${args.join(" ")}`;
}

function outputMatches(pattern: string, stdout: string, stderr: string): boolean {
  try {
    return new RegExp(pattern).test([stdout, stderr].filter(Boolean).join("\n"));
  } catch {
    return false;
  }
}

type RuntimeResolution = {
  params: ZmuxParams;
  details: { runtimeName: string; configPath?: string; ignoredReason?: string };
};

function resolveRuntimeParams(params: ZmuxParams, defaultCwd: string, projectTrusted: boolean): RuntimeResolution | undefined {
  if (params.operation !== "runtime_ensure" && params.operation !== "runtime_logs" && params.operation !== "runtime_stop") return undefined;
  const runtimeName = requireTarget(params, "runtime name");
  const options = params.options ?? {};
  const config = loadConfig(defaultCwd, { projectTrusted });
  const overrides: RuntimeConfig = {};
  if (params.command) overrides.command = params.command;
  const tab = optString(options, "tab");
  const session = optString(options, "session");
  const readiness = optString(options, "readiness") ?? optString(options, "waitFor");
  const kind = optString(options, "kind");
  const timeoutSeconds = optNumber(options, "timeoutSeconds");
  if (tab) overrides.tab = tab;
  if (session) overrides.session = session;
  if (readiness) overrides.readiness = readiness;
  if (kind) overrides.kind = kind;
  if (params.cwd) overrides.cwd = params.cwd;
  if (timeoutSeconds !== undefined) overrides.timeoutSeconds = timeoutSeconds;
  const runtime = mergeRuntimeConfig(runtimeName, overrides, config);
  const mergedOptions: Record<string, unknown> = { ...options };
  if (runtime.session) mergedOptions.session = runtime.session;
  if (runtime.readiness) mergedOptions.readiness = runtime.readiness;
  if (runtime.kind) mergedOptions.kind = runtime.kind;
  if (runtime.timeoutSeconds !== undefined) mergedOptions.timeoutSeconds = runtime.timeoutSeconds;
  return {
    params: {
      ...params,
      target: runtime.tab,
      command: params.operation === "runtime_ensure" ? runtime.command : undefined,
      cwd: runtime.cwd ? resolve(defaultCwd, runtime.cwd) : defaultCwd,
      options: mergedOptions,
    },
    details: { runtimeName, configPath: config.path, ignoredReason: config.ignoredReason },
  };
}

function content(text: string, details: Record<string, unknown> = {}) {
  const maxBytes = 50 * 1024;
  const maxLines = 2_000;
  const lines = text.split("\n");
  let selected = lines.slice(Math.max(0, lines.length - maxLines)).join("\n");
  while (Buffer.byteLength(selected, "utf8") > maxBytes) selected = selected.slice(Math.ceil(selected.length * 0.1));
  const truncated = selected !== text;
  return {
    content: [{ type: "text" as const, text: truncated ? `${selected}\n\n[pi-zmux: output truncated]` : selected }],
    details: truncated ? { ...details, truncated: true } : details,
  };
}

type ForegroundProgress = {
  setPhase(phase: string): void;
  stop(): void;
};

let callbackStatusSetter: ((text: string | undefined) => void) | undefined;
const sharedCallbackActivitySink: CallbackActivitySink = {
  set(text) {
    callbackStatusSetter?.(text);
  },
};

function callbackActivitySinkFor(mode: "tui" | "rpc" | "json" | "print", setStatus: (text: string | undefined) => void): CallbackActivitySink | undefined {
  if (mode !== "tui") return undefined;
  callbackStatusSetter = setStatus;
  return sharedCallbackActivitySink;
}

export function clearZmuxDispatcherActivity(): void {
  callbackStatusSetter?.(undefined);
  callbackStatusSetter = undefined;
}

function initialProgressPhase(params: ZmuxParams): string {
  const options = params.options ?? {};
  if (params.operation === "peer_ensure") return "waiting for peer readiness";
  if (params.operation === "runtime_ensure") return "starting runtime";
  if (params.operation === "runtime_logs" || params.operation === "wait") {
    if (optString(options, "waitFor")) return "waiting for output";
    if (optNumber(options, "idleSeconds") !== undefined) return "waiting for idle condition";
  }
  if (params.operation === "interactive_type") return "waiting for command completion";
  return `${params.operation.replaceAll("_", " ")} running`;
}

export function executionTimeoutSeconds(params: ZmuxParams): number {
  const explicit = optNumber(params.options ?? {}, "timeoutSeconds");
  if (explicit !== undefined) return explicit;
  if (params.operation === "wait" || params.operation === "callback_watch" || params.operation === "peer_handoff") return 300;
  if (params.operation === "runtime_logs" && (optString(params.options ?? {}, "waitFor") || optNumber(params.options ?? {}, "idleSeconds") !== undefined)) return 10;
  if (["pane_resize", "pane_send_keys", "pane_type"].includes(params.operation)) return 5;
  if (params.operation === "interactive_type" || params.operation === "runtime_ensure") return 90;
  if (params.operation === "run") return 130;
  return 30;
}

function createForegroundProgress(
  params: ZmuxParams,
  cwd: string,
  mode: "tui" | "rpc" | "json" | "print",
  onUpdate: AgentToolUpdateCallback<Record<string, unknown>> | undefined,
): ForegroundProgress {
  if (mode !== "tui" || !onUpdate) return { setPhase() {}, stop() {} };
  const startedAt = Date.now();
  const deadlineAt = startedAt + executionTimeoutSeconds(params) * 1000;
  let phase = initialProgressPhase(params);
  let interval: ReturnType<typeof setInterval> | undefined;
  let stopped = false;
  const emit = () => {
    if (stopped) return;
    const now = Date.now();
    onUpdate(withDisplayMetadata(content("", {
      status: "running",
      phase,
      elapsedSeconds: Math.max(0, Math.floor((now - startedAt) / 1000)),
      remainingSeconds: Math.max(0, Math.ceil((deadlineAt - now) / 1000)),
    }), params, cwd));
  };
  const delay = setTimeout(() => {
    if (stopped) return;
    emit();
    interval = setInterval(emit, 1_000);
  }, 400);
  return {
    setPhase(nextPhase) {
      phase = nextPhase;
      if (interval) emit();
    },
    stop() {
      stopped = true;
      clearTimeout(delay);
      if (interval) clearInterval(interval);
    },
  };
}

async function executeSpecial(pi: ExtensionAPI, params: ZmuxParams, defaultCwd: string, signal?: AbortSignal, activitySink?: CallbackActivitySink) {
  const options = params.options ?? {};
  const cwd = cwdFor(defaultCwd, params);
  if (params.operation === "callback_list") {
    const active = listCallbacks();
    const completed = listRecentCallbackCompletions();
    const activeText = active.length
      ? active.map((record) => `- ${record.id}: ${record.tab} started ${record.startedAt}`).join("\n")
      : "no active zmux callbacks";
    const recentText = completed.length
      ? `recent:\n${completed.map((record) => `- ${record.id}: ${record.status} for ${record.tab} at ${record.finishedAt}`).join("\n")}`
      : "";
    return content([activeText, recentText].filter(Boolean).join("\n"), { callbacks: active, completed });
  }
  if (params.operation === "callback_cancel") {
    const id = params.target || optString(options, "id");
    if (!id) throw new Error("target or options.id is required for callback_cancel");
    const cancelled = cancelCallback(id);
    return content(cancelled ? `cancelled zmux callback ${id}` : `no active zmux callback ${id}`, { id, cancelled });
  }
  if (params.operation === "callback_watch") {
    const callback = startWatchCallback(pi, {
      id: optString(options, "id"),
      tab: requireTarget(params, "tab"),
      cwd,
      session: optString(options, "session"),
      lines: optNumber(options, "lines"),
      waitFor: optString(options, "waitFor"),
      idleSeconds: optNumber(options, "idleSeconds"),
      timeoutSeconds: optNumber(options, "timeoutSeconds"),
      message: optString(options, "message"),
      deliverAs: optDeliverAs(options),
      triggerTurn: optBool(options, "triggerTurn"),
      activitySink,
    });
    return content(callback.text, callback.details);
  }
  if (params.operation === "peer_handoff") {
    const target = requireTarget(params, "peer tab");
    const text = optString(options, "text");
    if (!text) throw new Error("options.text is required for peer_handoff");
    const waitFor = optString(options, "waitFor");
    const idleSeconds = optNumber(options, "idleSeconds");
    if (waitFor && idleSeconds !== undefined) throw new Error("peer_handoff waitFor and idleSeconds cannot be combined");
    if (waitFor && outputMatches(waitFor, text, "")) {
      throw new Error("peer_handoff waitFor pattern must not match options.text; split or rephrase the outgoing marker so echoed prompt text cannot self-match");
    }
    const handoff = await startPeerHandoff(pi, {
      id: optString(options, "id"),
      tab: target,
      text,
      cwd,
      session: optString(options, "session"),
      lines: optNumber(options, "lines"),
      waitFor,
      idleSeconds,
      timeoutSeconds: optNumber(options, "timeoutSeconds"),
      markPeerRunning: optBool(options, "markPeerRunning"),
      source: optString(options, "source"),
      message: optString(options, "message"),
      deliverAs: optDeliverAs(options),
      triggerTurn: optBool(options, "triggerTurn"),
      activitySink,
    });
    return content(handoff.text, handoff.details);
  }
  if (params.operation === "pane_resize" && (!optString(options, "axis") || optString(options, "axis") === "auto")) {
    const size = optString(options, "size");
    if (!size) throw new Error("options.size is required for pane_resize");
    const result = await resizePane(requireTarget(params, "pane"), cwd, size, "auto");
    return content(result.text, result.details);
  }
  if (params.operation === "pane_send_keys") {
    const args = buildArgs(params);
    const [, pane, ...keys] = args;
    await runTmux(["send-keys", "-t", pane, ...keys], { cwd, signal, timeoutMs: timeoutMs(options, 5) });
    return content(`sent keys to pane ${pane}: ${keys.join(" ")}`, { pane, keys });
  }
  if (params.operation === "pane_type") {
    const args = buildArgs(params);
    const [, pane, text] = args;
    await runTmux(["send-keys", "-t", pane, "-l", text], { cwd, signal, timeoutMs: timeoutMs(options, 5) });
    await runTmux(["send-keys", "-t", pane, "Enter"], { cwd, signal, timeoutMs: timeoutMs(options, 5) });
    return content(`typed text into pane ${pane}`, { pane });
  }
  if (params.operation === "pi_reload") {
    const result = await schedulePiReload({
      cwd,
      paneId: params.target || optString(options, "paneId"),
      delayMs: optNumber(options, "delayMs"),
      retryAttempts: optNumber(options, "retryAttempts"),
      retryDelayMs: optNumber(options, "retryDelayMs"),
      continuationPrompt: optString(options, "continuationPrompt"),
    });
    return content(result.text, result.details);
  }
  if (params.operation === "pi_respawn") {
    const result = await schedulePiRespawn({
      cwd,
      paneId: params.target || optString(options, "paneId"),
      command: params.command || optString(options, "restartCommand"),
      delayMs: optNumber(options, "delayMs"),
      continuationPrompt: optString(options, "continuationPrompt"),
    });
    return content(result.text, result.details);
  }
  return undefined;
}

export function registerZmuxDispatcher(pi: ExtensionAPI): void {
  clearZmuxDispatcherActivity();
  pi.registerTool({
    name: "zmux",
    label: "zmux",
    description:
      "Canonical zmux dispatcher for terminal/session work: choose an operation, provide a primary target/command, and keep focus-moving options false unless the user explicitly asked.",
    promptSnippet: "Dispatch canonical zmux terminal/session/runtime operations",
    promptGuidelines: [
      "Use zmux instead of bash/raw tmux for runtimes, visible tabs, panes, sessions, waits, peers, and Pi lifecycle; never background long-running commands.",
      "Map: dev server -> runtime_ensure; existing output -> runtime_logs; visible one-shot -> run; sidecar -> pane_open; named tab cleanup -> tab_kill.",
      "For run, options.focus=false preserves the current tab and options.waitForExit=false detaches and returns immediately. For later notification, register callback_watch after the detached run; shell hooks own lifecycle state automatically.",
      "For a peer prompt plus response notification, use peer_ensure then atomic peer_handoff with options.text. It marks the peer running, waits for fresh turn:ready lifecycle, and returns through a follow-up notification by default. Use options.waitFor only as an output-regex fallback for an uninstrumented peer; never send type_text then callback_watch.",
      "For sudo, ssh, passwords, REPLs, database shells, and other manual input, use interactive_type and never generic run; target admin (or the named shared tab), keep focus false by default, and set options.waitForExit for bounded privileged commands.",
      "For runtime_ensure, set target to the runtime/tab name, command to the dev/watch command, cwd to the project/fixture directory, and options.waitFor/readiness to ready|localhost when the user asks to wait for readiness.",
      "For wait/callback_watch, options.waitFor is the output regex only (never prefix output:); set exactly one of waitFor or idleSeconds, never let the callback pattern match outgoing text, and do not block or poll after registration. deliverAs=nextTurn cannot trigger a continuation; use steer/followUp when triggerTurn is true.",
      "For a named joined pane, call current, then panes with options.session set to that current session; match its TITLE and use the raw %pane id. For literal unsubmitted text use pane_send_keys with options.keys as a string array; pane_type appends Enter.",
      "For a soft Pi reload, call pi_reload and omit target; it resolves this Pi pane, and its continuation proves completion, while terminal_current only diagnoses the desktop terminal. For pi_reload/pi_respawn continuation, use options.continuationPrompt; never use callback-only deliverAs/triggerTurn.",
      "Start with operation=sessions or tabs when target/session is ambiguous; never operate on a generic tab name like scratch unless the prompt names the exact tab/session.",
      "Never set focus:true unless the user explicitly wants terminal focus moved; if the prompt says focus but also says it was not explicitly requested, keep focus false or refuse.",
      "For servers/watchers, use runtime_ensure/logs/stop. If asked for another copy before logs, ignore that order: runtime_logs the existing target; do not start duplicate processes.",
      "For remote/admin runs, reuse one stable admin/remote-host tab, decode opaque payloads, and state the intended host mutation before changing remote config.",
    ],
    parameters: paramsSchema,
    async execute(_id, inputParams: ZmuxParams, signal, onUpdate, ctx) {
      const runtime = resolveRuntimeParams(inputParams, ctx.cwd, ctx.isProjectTrusted());
      const params = runtime?.params ?? inputParams;
      const options = params.options ?? {};
      const cwd = cwdFor(ctx.cwd, params);
      const progress = createForegroundProgress(params, cwd, ctx.mode, onUpdate);
      const activitySink = callbackActivitySinkFor(ctx.mode, (text) => ctx.ui.setStatus("pi-zmux-waits", text));
      try {
      if (params.operation === "runtime_ensure" && !params.command) {
        return withDisplayMetadata(
          content(`ERROR: runtime ${runtime?.details.runtimeName ?? params.target ?? "unknown"} has no command. Pass command or add it to trusted .pi/zmux.json / .config/pi-zmux.json.`, { ...runtime?.details, failed: true }),
          params,
          cwdFor(ctx.cwd, params),
        );
      }
      if (params.command) {
        const headlessError = rejectHeadlessAgentPrintMode(params.command);
        if (headlessError) {
          return withDisplayMetadata(
            content(headlessError, { command: params.command, failed: true, failureKind: "headless_agent_print_mode" }),
            params,
            cwdFor(ctx.cwd, params),
          );
        }
      }
      if (params.operation === "interactive_type") {
        const tab = params.target || "admin";
        const command = requireCommand(params);
        const waitFor = optString(options, "waitFor");
        const waitForExit = optBool(options, "waitForExit") ?? shouldWaitForExit(command);
        progress.setPhase(waitFor ? "waiting for output" : waitForExit ? "waiting for command completion" : "typing interactive command");
        const result = await interactiveType(tab, command, cwdFor(ctx.cwd, params), {
          waitFor,
          waitForExit,
          timeoutSeconds: optNumber(options, "timeoutSeconds"),
          lines: optNumber(options, "lines"),
          focus: optBool(options, "focus"),
          session: optString(options, "session"),
        });
        return withDisplayMetadata(content(result.text, result.details), params, cwdFor(ctx.cwd, params), { output: result.text });
      }
      const special = await executeSpecial(pi, params, ctx.cwd, signal, activitySink);
      if (special) return withDisplayMetadata(special, params, cwd, { output: special.content.map((item) => item.text).join("\n") });
      const args = buildArgs(params);
      const runSafety = params.operation === "run"
        ? zmuxRunSafetyWarnings({ command: requireCommand(params), cwd, tab: params.target, session: optString(options, "session") })
        : undefined;
      let restartText = "";
      let restartStopped: boolean | undefined;
      if (params.operation === "runtime_ensure" && optBool(options, "restart")) {
        progress.setPhase("stopping runtime before restart");
        const stopArgs = ["send", requireTarget(params, "runtime tab"), "C-c"];
        pushSession(stopArgs, optString(options, "session"));
        const stopped = await runZmux(stopArgs, { cwd, signal, timeoutMs: 5_000 });
        restartStopped = !(stopped.timedOut || stopped.code !== 0);
        restartText = restartStopped ? `sent C-c to ${params.target}` : `restart stop skipped: ${formatResult("runtime_stop", stopArgs, stopped.stdout, stopped.stderr)}`;
      }
      if (params.operation === "runtime_ensure") progress.setPhase("starting runtime");
      const result = await runZmux(args, { cwd, signal, timeoutMs: executionTimeoutSeconds(params) * 1000 });
      const failed = result.timedOut || result.code !== 0;
      const rawResultText = formatResult(params.operation, args, result.stdout, result.stderr);
      const waitSummary = params.operation === "wait" ? summarizeWaitOutput(rawResultText) : undefined;
      let text = [runSafety?.text, restartText, waitSummary?.text ?? rawResultText].filter(Boolean).join("\n");
      if (failed) {
        const processOutput = [result.stdout.trim(), result.stderr.trim()].filter(Boolean).join("\n");
        if (result.timedOut) {
          throw new Error(`${TIMEOUT_ERROR_PREFIX} zmux ${params.operation} timed out after ${executionTimeoutSeconds(params)}s; completion unproven${processOutput ? `\n${processOutput}` : ""}`);
        }
        throw new Error(processOutput || `zmux ${params.operation} failed: ${args.join(" ")}`);
      }
      const details: Record<string, unknown> = { operation: params.operation, args, cwd, exitCode: result.code, ...runtime?.details, ...runSafety?.details, ...waitSummary?.details };
      if (restartStopped !== undefined) details.restartStopped = restartStopped;
      if (params.operation === "runtime_ensure") {
        const readiness = optString(options, "readiness") ?? optString(options, "waitFor");
        if (readiness && outputMatches(readiness, result.stdout, result.stderr)) {
          details.ready = true;
          details.readinessBasis = "initial-output";
        } else if (readiness) {
          const tab = optString(options, "tab") ?? params.target;
          const watchOptions = { ...options, waitFor: readiness, timeoutSeconds: optNumber(options, "timeoutSeconds") ?? 90 };
          const watchArgs = buildWatchArgs(tab ?? "", watchOptions);
          progress.setPhase("waiting for runtime readiness");
          const watch = await runZmux(watchArgs, { cwd, signal, timeoutMs: timeoutMs(watchOptions, 90) });
          text = [text, formatResult("runtime_logs", watchArgs, watch.stdout, watch.stderr)].filter(Boolean).join("\n");
          details.ready = !(watch.timedOut || watch.code !== 0);
          details.readinessBasis = "fresh-watch";
        }
      }
      return withDisplayMetadata(content(text, details), params, cwd, { args, exitCode: result.code, output: params.operation === "wait" ? rawResultText : text });
      } finally {
        progress.stop();
      }
    },
    renderCall(args, theme, context) {
      const text = (context.lastComponent as Text | undefined) ?? new Text("", 0, 0);
      text.setText(formatZmuxCall(args as ZmuxParams, context.expanded, theme, context.isPartial));
      return text;
    },
    renderResult(result, options, theme, context) {
      const text = (context.lastComponent as Text | undefined) ?? new Text("", 0, 0);
      text.setText(formatZmuxResult(result as ZmuxRenderResult, context.args as ZmuxParams, options, context.isError, theme));
      return text;
    },
  });

}
