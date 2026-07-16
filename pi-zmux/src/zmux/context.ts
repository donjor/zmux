import { trimOutput } from "../shell.js";
import { tmux, withSession, zmux } from "./shared.js";

export interface CurrentPane {
	session?: string;
	paneId?: string;
	index?: number;
	windowIndex?: number;
	command?: string;
	dir?: string;
	title?: string;
}

export async function currentPane(cwd: string): Promise<CurrentPane | undefined> {
	try {
		const result = await zmux(["pane", "current", "--json"], { cwd, timeoutMs: 5_000 });
		return JSON.parse(result.stdout) as CurrentPane;
	} catch {
		return undefined;
	}
}

export async function listTabs(cwd: string, session?: string): Promise<string> {
	try {
		const result = await zmux(withSession(["tabs"], session), { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function capabilities(cwd: string): Promise<string> {
	try {
		const result = await zmux(["terminal", "capabilities"], { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function reloadZmux(cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(["reload"], { cwd, timeoutMs: 15_000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	return { text: output || "reloaded zmux", details: { command: "zmux reload" } };
}

export async function focusTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = await currentPane(cwd);
	if (!pane?.session) throw new Error("cannot resolve current zmux session for tab focus");
	await tmux(["select-window", "-t", `${pane.session}:${tab}`], { cwd, timeoutMs: 5_000 });
	return { text: `focused tab ${tab}`, details: { tab, session: pane.session } };
}
