import { spawn } from "node:child_process";
import { basename } from "node:path";

export interface CommandResult {
  stdout: string;
  stderr: string;
  code: number | null;
  signal: NodeJS.Signals | null;
  timedOut: boolean;
}

export function zmuxBin(): string {
  return process.env.PI_ZMUX_BIN?.trim() || "zmux";
}

export function tmuxSocketArgs(): string[] {
  const explicit = process.env.PI_ZMUX_TMUX_SOCKET?.trim();
  if (explicit) return ["-L", explicit];
  if (basename(zmuxBin()) === "zzmux") return ["-L", "zzmux"];
  return [];
}

export function runFile(
  command: string,
  args: string[],
  options: { cwd?: string; timeoutMs?: number; signal?: AbortSignal } = {},
): Promise<CommandResult> {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      stdio: ["ignore", "pipe", "pipe"],
      signal: options.signal,
    });
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    const timer = options.timeoutMs
      ? setTimeout(() => {
          timedOut = true;
          child.kill("SIGTERM");
        }, options.timeoutMs)
      : undefined;
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      stdout += String(chunk);
    });
    child.stderr.on("data", (chunk) => {
      stderr += String(chunk);
    });
    child.on("error", (error) => {
      if (timer) clearTimeout(timer);
      reject(error);
    });
    child.on("close", (code, signal) => {
      if (timer) clearTimeout(timer);
      resolve({ stdout, stderr, code, signal, timedOut });
    });
  });
}

export async function runZmux(args: string[], options: { cwd?: string; timeoutMs?: number; signal?: AbortSignal } = {}) {
  return runFile(zmuxBin(), args, options);
}

export async function runTmux(args: string[], options: { cwd?: string; timeoutMs?: number; signal?: AbortSignal } = {}) {
  return runFile("tmux", [...tmuxSocketArgs(), ...args], options);
}

export function shellQuote(value: string): string {
  return `'${value.replace(/'/gu, `"'"'"'`)}'`;
}
