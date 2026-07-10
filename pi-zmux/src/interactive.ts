import { runZmux, type CommandResult } from "./exec.js";

export type InteractiveOptions = {
  waitFor?: string;
  waitForExit?: boolean;
  timeoutSeconds?: number;
  lines?: number;
  focus?: boolean;
  session?: string;
};

type CommandStatus = {
  cmdState?: unknown;
  cmdSeq?: unknown;
  lastExit?: unknown;
  command?: unknown;
};

function delay(ms: number): Promise<void> {
  return new Promise((resolvePromise) => setTimeout(resolvePromise, ms));
}

function withSession(args: string[], session?: string): string[] {
  return session ? [...args, "-s", session] : args;
}

function failed(result: CommandResult): boolean {
  return result.timedOut || result.code !== 0;
}

async function checked(args: string[], cwd: string, timeoutMs: number): Promise<CommandResult> {
  const result = await runZmux(args, { cwd, timeoutMs });
  if (failed(result)) throw new Error(result.stderr || result.stdout || `zmux ${args.join(" ")} failed`);
  return result;
}

async function readStatus(tab: string, cwd: string, session?: string): Promise<CommandStatus | undefined> {
  const result = await checked(withSession(["tab", "status", tab, "--json"], session), cwd, 5_000);
  const output = (result.stdout || result.stderr).trim();
  if (!output) return undefined;
  try {
    const parsed = JSON.parse(output) as CommandStatus;
    return parsed && typeof parsed === "object" ? parsed : undefined;
  } catch {
    return undefined;
  }
}

async function tabExists(tab: string, cwd: string, session?: string): Promise<boolean> {
  try {
    await readStatus(tab, cwd, session);
    return true;
  } catch {
    return false;
  }
}

function parseNumber(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string" || !/^\d+$/u.test(value.trim())) return undefined;
  return Number(value.trim());
}

function commandState(status?: CommandStatus): string {
  return typeof status?.cmdState === "string" ? status.cmdState : "";
}

function commandSeq(status?: CommandStatus): number | undefined {
  return parseNumber(status?.cmdSeq);
}

function exitCode(status?: CommandStatus): number {
  return parseNumber(status?.lastExit) ?? (commandState(status) === "failed" ? 1 : 0);
}

function outputAfterBaseline(latest: string, baseline: string): string {
  if (!baseline || latest === baseline) return baseline ? "" : latest;
  if (latest.startsWith(baseline)) return latest.slice(baseline.length).replace(/^\n/u, "");
  const before = baseline.split("\n");
  const after = latest.split("\n");
  for (let count = Math.min(before.length, after.length); count > 0; count--) {
    if (before.slice(-count).join("\n") === after.slice(0, count).join("\n")) return after.slice(count).join("\n");
  }
  return latest;
}

export type UserInputPrompt = { kind: "sudo_password" | "password" | "ssh_confirm"; line: string };

export function detectUserInputPrompt(output: string): UserInputPrompt | undefined {
  const lines = output.split("\n").map((line) => line.trim()).filter(Boolean);
  for (let index = lines.length - 1; index >= 0; index--) {
    const line = lines[index];
    if (/\[sudo\]\s+password\s+for\s+.+:\s*$/iu.test(line)) return { kind: "sudo_password", line };
    if (/(password|passphrase).*:\s*$/iu.test(line)) return { kind: "password", line };
    if (/are you sure you want to continue connecting.*\?\s*$/iu.test(line)) return { kind: "ssh_confirm", line };
  }
  return undefined;
}

async function logs(tab: string, cwd: string, lines: number, session?: string): Promise<string> {
  const result = await checked(withSession(["watch", tab, "-l", String(lines), "-T", "10"], session), cwd, 12_000);
  return (result.stdout || result.stderr).trimEnd();
}

export async function interactiveType(
  tab: string,
  command: string,
  cwd: string,
  options: InteractiveOptions = {},
): Promise<{ text: string; details: Record<string, unknown> }> {
  const focus = options.focus ?? false;
  const timeoutSeconds = options.timeoutSeconds ?? 90;
  const lines = options.lines ?? 160;
  if (!(await tabExists(tab, cwd, options.session))) {
    await checked(withSession(["run", "--command", "exec bash -l", "-n", tab, "-d"], options.session), cwd, 10_000);
    await delay(300);
  }
  if (focus) await checked(["tab", "focus", tab], cwd, 5_000);

  const baseline = options.waitForExit || options.waitFor ? await logs(tab, cwd, lines, options.session).catch(() => "") : "";
  const baselineStatus = options.waitForExit ? await readStatus(tab, cwd, options.session).catch(() => undefined) : undefined;
  await checked(withSession(["type", tab, command], options.session), cwd, 5_000);

  const output = [`typed command into ${tab}${focus ? " and focused it" : " without changing focus"}; user may need to respond there`];
  const details: Record<string, unknown> = { tab, command, waitForExit: options.waitForExit ?? false, focus, session: options.session };
  const deadline = Date.now() + timeoutSeconds * 1000;

  if (options.waitForExit) {
    const baselineSeq = commandSeq(baselineStatus);
    let latest = "";
    while (Date.now() <= deadline) {
      latest = await logs(tab, cwd, lines, options.session);
      const status = await readStatus(tab, cwd, options.session);
      const seq = commandSeq(status);
      const state = commandState(status);
      if (seq !== undefined && (baselineSeq === undefined || seq > baselineSeq) && (state === "done" || state === "failed")) {
        const scoped = outputAfterBaseline(latest, baseline).trimEnd();
        if (scoped) output.push("", scoped);
        details.completed = true;
        details.exitCode = exitCode(status);
        details.cmdSeq = seq;
        details.cmdState = state;
        if (typeof status?.command === "string") details.observedCommand = status.command;
        return { text: output.join("\n"), details };
      }
      const prompt = detectUserInputPrompt(outputAfterBaseline(latest, baseline));
      if (prompt && !focus) {
        output.push("", `user input required in ${tab}: ${prompt.line}`, "Ask the user before focusing this tab, or ask them to switch there manually.");
        return { text: output.join("\n"), details: { ...details, completed: false, needsUserInput: true, promptKind: prompt.kind, prompt: prompt.line } };
      }
      await delay(500);
    }
    const scoped = outputAfterBaseline(latest, baseline).trimEnd();
    const exitMarkers = [...scoped.matchAll(/\[ble:\s*exit\s+(\d+)\]/giu)];
    const marker = exitMarkers.at(-1);
    if (marker) {
      if (scoped) output.push("", scoped);
      details.completed = true;
      details.exitCode = Number(marker[1]);
      details.evidenceBasis = "shell-output-exit-marker";
      details.warning = "fresh command lifecycle status was unavailable; exit came from retained shell output";
      return { text: output.join("\n"), details };
    }
    if (scoped) output.push("", scoped);
    output.push("", `wait timed out after ${timeoutSeconds}s`);
    details.completed = false;
  } else if (options.waitFor) {
    const pattern = new RegExp(options.waitFor);
    while (Date.now() <= deadline) {
      const latest = await logs(tab, cwd, lines, options.session);
      const scoped = outputAfterBaseline(latest, baseline);
      if (pattern.test(scoped)) {
        output.push("", scoped);
        details.waitFor = options.waitFor;
        details.matched = true;
        return { text: output.join("\n"), details };
      }
      const prompt = detectUserInputPrompt(scoped);
      if (prompt && !focus) {
        output.push("", `user input required in ${tab}: ${prompt.line}`);
        return { text: output.join("\n"), details: { ...details, waitFor: options.waitFor, matched: false, needsUserInput: true, promptKind: prompt.kind } };
      }
      await delay(500);
    }
    output.push("", `wait timed out after ${timeoutSeconds}s`);
    details.waitFor = options.waitFor;
    details.matched = false;
  } else {
    output.push(`Tell the user to complete any prompts in tab ${tab}.`);
  }
  return { text: output.join("\n"), details };
}
