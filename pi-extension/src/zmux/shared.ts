import { basename } from "node:path";
import { runFile } from "../shell.js";

export function zmuxBin(): string {
	return process.env.PI_ZMUX_BIN?.trim() || "zmux";
}

export function tmuxPrefix(): string[] {
	const explicitSocket = process.env.PI_ZMUX_TMUX_SOCKET?.trim();
	if (explicitSocket) return ["-L", explicitSocket];
	if (basename(zmuxBin()) === "zzmux") return ["-L", "zzmux"];
	return [];
}

export function withSession(args: string[], session?: string): string[] {
	return session ? [...args, "-s", session] : args;
}

export function zmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile(zmuxBin(), args, options);
}

export function tmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile("tmux", [...tmuxPrefix(), ...args], options);
}

export function safeJson(value: string): unknown {
	try {
		return JSON.parse(value);
	} catch {
		return undefined;
	}
}

export function delay(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

export function shellQuote(value: string): string {
	return `'${value.replace(/'/gu, `"'"'"'`)}'`;
}
