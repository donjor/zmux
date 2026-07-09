import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawn, type ChildProcess } from "node:child_process";
import { Type } from "typebox";
import { runTmux, runZmux, shellQuote, tmuxSocketArgs, zmuxBin } from "./exec.js";

export const LITE_OPERATIONS = [
  "current",
  "tabs",
  "sessions",
  "panes",
  "run",
  "session_run",
  "session_kill",
  "runtime_ensure",
  "runtime_logs",
  "runtime_stop",
  "tab_state",
  "tab_peer",
  "tab_status",
  "tab_inspect",
  "tab_label",
  "tab_move",
  "tab_place",
  "tab_kill",
  "tab_focus",
  "send_keys",
  "type_text",
  "peer_ensure",
  "pane_open",
  "pane_close",
  "pane_resize",
  "pane_focus",
  "pane_send_keys",
  "pane_type",
  "log",
  "snapshot",
  "wait",
  "callback_watch",
  "callback_list",
  "callback_cancel",
  "interactive_type",
  "terminal_current",
  "zmux_reload",
  "pi_reload",
  "pi_respawn",
] as const;

type Operation = (typeof LITE_OPERATIONS)[number];

type LiteParams = {
  operation: string;
  target?: string;
  command?: string;
  cwd?: string;
  options?: Record<string, unknown>;
};

type CallbackRecord = {
  id: string;
  target: string;
  startedAt: string;
  process: ChildProcess;
};

const callbacks = new Map<string, CallbackRecord>();

const paramsSchema = Type.Object(
  {
    operation: Type.String({
      description: `Required zmux action. One of: ${LITE_OPERATIONS.join(", ")}. Prefer outcome names over legacy tool names.`,
    }),
    target: Type.Optional(Type.String({ description: "Primary tab/session/pane/runtime target, depending on operation." })),
    command: Type.Optional(Type.String({ description: "Shell command for run/session_run/runtime_ensure/pane_open/interactive_type." })),
    cwd: Type.Optional(Type.String({ description: "Working directory for the zmux CLI process; defaults to Pi cwd." })),
    options: Type.Optional(
      Type.Record(Type.String(), Type.Any(), {
        description: "Operation-specific small options: tab, session, lines, waitFor, idleSeconds, timeoutSeconds, focus, state, action, direction, size, keys, text, destination, workspace, restart.",
      }),
    ),
  },
  { additionalProperties: false },
);

function isOperation(value: string): value is Operation {
  return (LITE_OPERATIONS as readonly string[]).includes(value);
}

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

function optStringArray(options: Record<string, unknown>, key: string): string[] | undefined {
  const value = options[key];
  if (value === undefined) return undefined;
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) {
    throw new Error(`options.${key} must be an array of strings`);
  }
  return value;
}

function requireTarget(params: LiteParams, noun = "target"): string {
  if (!params.target?.trim()) throw new Error(`${noun} is required for operation ${params.operation}`);
  return params.target.trim();
}

function requireCommand(params: LiteParams): string {
  if (!params.command?.trim()) throw new Error(`command is required for operation ${params.operation}`);
  return params.command;
}

function pushOpt(args: string[], flag: string, value: string | number | undefined): void {
  if (value !== undefined && value !== "") args.push(flag, String(value));
}

function pushSession(args: string[], session?: string): void {
  if (session) args.push("-s", session);
}

function cwdFor(defaultCwd: string, params: LiteParams): string {
  return params.cwd || defaultCwd;
}

function timeoutMs(options: Record<string, unknown>, fallbackSeconds = 30): number {
  return (optNumber(options, "timeoutSeconds") ?? fallbackSeconds) * 1000;
}

function buildArgs(params: LiteParams): string[] {
  if (!isOperation(params.operation)) throw new Error(`unknown zmux_lite operation ${params.operation}`);
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
      const args = ["run", "--command", requireCommand(params)];
      pushOpt(args, "-n", optString(options, "tab") ?? target);
      pushSession(args, session);
      pushOpt(args, "-T", optNumber(options, "timeoutSeconds"));
      pushOpt(args, "--lines", lines);
      if (optBool(options, "detach")) args.push("-d");
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
      const args = ["tab", action];
      if (target) args.push(target);
      pushOpt(args, "--session", session);
      pushOpt(args, "--into", optString(options, "into"));
      const direction = optString(options, "direction");
      if (direction) args.push(`--${direction}`);
      pushOpt(args, "--size", optString(options, "size"));
      pushOpt(args, "--pane", optString(options, "pane"));
      if (optBool(options, "after")) args.push("--after");
      if (optBool(options, "focus")) args.push("--focus");
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
      args.push(({ right: "-r", left: "-l", down: "-d", up: "-u" } as Record<string, string>)[direction] ?? "-r");
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
      const args = ["log", action];
      if (target) args.push(target);
      if (optBool(options, "ansi")) args.push("--ansi");
      pushOpt(args, "--max-bytes", optNumber(options, "maxBytes"));
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
      return [];
    case "interactive_type": {
      const tab = target || "admin";
      return ["run", "--command", requireCommand(params), "-n", tab, "--keep", "--scope", "manual"];
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
  if (waitFor) args.push("--until", waitFor);
  if (idleSeconds !== undefined) args.push("--idle", String(idleSeconds));
  pushOpt(args, "-T", optNumber(options, "timeoutSeconds") ?? 10);
  pushSession(args, optString(options, "session"));
  return args;
}

function buildWaitArgs(target: string, options: Record<string, unknown>): string[] {
  const args = ["wait", target];
  const waitFor = optString(options, "waitFor");
  const idleSeconds = optNumber(options, "idleSeconds");
  if (waitFor) args.push("--for", `output:${waitFor}`);
  if (idleSeconds !== undefined) args.push("--for", `idle:${idleSeconds}`);
  pushOpt(args, "-l", optNumber(options, "lines") ?? 160);
  pushOpt(args, "-T", optNumber(options, "timeoutSeconds") ?? 300);
  args.push("--json");
  pushSession(args, optString(options, "session"));
  return args;
}

function formatResult(operation: string, args: string[], stdout: string, stderr: string): string {
  const body = [stdout.trim(), stderr.trim()].filter(Boolean).join("\n");
  return body || `zmux_lite ${operation} completed: ${args.join(" ")}`;
}

function content(text: string, details: Record<string, unknown> = {}) {
  const maxBytes = 50 * 1024;
  const maxLines = 2_000;
  const lines = text.split("\n");
  let selected = lines.slice(Math.max(0, lines.length - maxLines)).join("\n");
  while (Buffer.byteLength(selected, "utf8") > maxBytes) selected = selected.slice(Math.ceil(selected.length * 0.1));
  const truncated = selected !== text;
  return {
    content: [{ type: "text" as const, text: truncated ? `${selected}\n\n[pi-zmux-lite: output truncated]` : selected }],
    details: truncated ? { ...details, truncated: true } : details,
  };
}

async function executeSpecial(pi: ExtensionAPI, params: LiteParams, defaultCwd: string, signal?: AbortSignal) {
  const options = params.options ?? {};
  const cwd = cwdFor(defaultCwd, params);
  if (params.operation === "callback_list") {
    return content(
      callbacks.size
        ? [...callbacks.values()].map((record) => `- ${record.id}: ${record.target} started ${record.startedAt}`).join("\n")
        : "no active zmux_lite callbacks",
      { callbacks: [...callbacks.keys()] },
    );
  }
  if (params.operation === "callback_cancel") {
    const id = params.target || optString(options, "id");
    if (!id) throw new Error("target or options.id is required for callback_cancel");
    const record = callbacks.get(id);
    if (!record) return content(`no active zmux_lite callback ${id}`, { id, cancelled: false });
    record.process.kill("SIGTERM");
    callbacks.delete(id);
    return content(`cancelled zmux_lite callback ${id}`, { id, cancelled: true });
  }
  if (params.operation === "callback_watch") {
    const args = buildArgs(params);
    const id = optString(options, "id") ?? `zmux-lite-${Date.now()}`;
    const child = spawn(zmuxBin(), args, { cwd, stdio: ["ignore", "pipe", "pipe"], signal });
    let stdout = "";
    let stderr = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      stdout += String(chunk);
    });
    child.stderr.on("data", (chunk) => {
      stderr += String(chunk);
    });
    callbacks.set(id, { id, target: requireTarget(params, "tab"), startedAt: new Date().toISOString(), process: child });
    child.on("close", (code, exitSignal) => {
      callbacks.delete(id);
      pi.sendMessage(
        {
          customType: "pi-zmux-lite-callback",
          content: formatResult("callback_watch", args, stdout, stderr),
          display: true,
          details: { id, args, code, signal: exitSignal, target: params.target },
        },
        { deliverAs: (optString(options, "deliverAs") as "steer" | "followUp" | "nextTurn" | undefined) ?? "steer", triggerTurn: optBool(options, "triggerTurn") ?? true },
      );
    });
    return content(`started zmux_lite callback ${id} for ${params.target}`, { id, args, target: params.target });
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
    const pane = params.target || optString(options, "paneId") || "%";
    const delayMs = optNumber(options, "delayMs") ?? 12_000;
    const tmuxCommand = ["tmux", ...tmuxSocketArgs()].map(shellQuote).join(" ");
    const command = `cd ${shellQuote(cwd)}; sleep ${delayMs / 1000}; ${tmuxCommand} send-keys -t ${shellQuote(pane)} /reload Enter`;
    await runZmux(["run", "--command", command, "-n", optString(options, "tab") ?? "pi-reload", "-d", "--keep", "--scope", "task"], { cwd, signal, timeoutMs: 5_000 });
    return content(`scheduled Pi reload for pane ${pane}`, { pane, delayMs });
  }
  if (params.operation === "pi_respawn") {
    const pane = params.target || optString(options, "paneId") || "%";
    const restartCommand = params.command || optString(options, "restartCommand") || "pi -c";
    await runTmux(["respawn-pane", "-k", "-t", pane, "-c", cwd, restartCommand], { cwd, signal, timeoutMs: 5_000 });
    return content(`respawned Pi pane ${pane}`, { pane, command: restartCommand });
  }
  return undefined;
}

export function registerLiteDispatcher(pi: ExtensionAPI): void {
  pi.registerTool({
    name: "zmux_lite",
    label: "zmux lite",
    description:
      "One dispatcher for zmux terminal/session work. It is a WIP A/B test surface: choose an operation, provide a primary target/command, and keep focus-moving options false unless the user explicitly asked.",
    promptSnippet: "Dispatch zmux terminal/session/runtime operations through one compact tool",
    promptGuidelines: [
      "Use zmux_lite for persistent runtimes, visible command tabs, tab/pane/session management, waits, and Pi reload/respawn instead of raw tmux or hidden long-running bash.",
      "For zmux_lite, start with operation=current or tabs when the target is ambiguous; never set focus:true unless the user explicitly wants terminal focus moved.",
      "For zmux_lite, prefer runtime_ensure/runtime_logs/runtime_stop for dev servers and watchers; do not start duplicate long-running commands in bash.",
    ],
    parameters: paramsSchema,
    async execute(_id, params: LiteParams, signal, _onUpdate, ctx) {
      const special = await executeSpecial(pi, params, ctx.cwd, signal);
      if (special) return special;
      const cwd = cwdFor(ctx.cwd, params);
      const args = buildArgs(params);
      const result = await runZmux(args, { cwd, signal, timeoutMs: timeoutMs(params.options ?? {}) });
      const failed = result.timedOut || result.code !== 0;
      const text = formatResult(params.operation, args, result.stdout, result.stderr);
      if (failed) throw new Error(text || `zmux_lite ${params.operation} failed`);
      return content(text, { operation: params.operation, args, cwd, exitCode: result.code });
    },
  });

  pi.on("session_shutdown", () => {
    for (const record of callbacks.values()) record.process.kill("SIGTERM");
    callbacks.clear();
  });
}
