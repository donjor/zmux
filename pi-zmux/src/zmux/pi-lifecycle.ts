import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { writeReloadContinuation } from "../reload-continuation.js";
import { writeRespawnContinuation } from "../respawn-continuation.js";
import { spawnDetached } from "../shell.js";
import { currentPane } from "./context.js";
import { shellQuote, tmuxPrefix } from "./shared.js";

export async function schedulePiReload(params: {
	cwd: string;
	paneId?: string;
	delayMs?: number;
	retryAttempts?: number;
	retryDelayMs?: number;
	continuationPrompt?: string;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = params.paneId ?? (await currentPane(params.cwd))?.paneId;
	if (!pane) throw new Error("cannot resolve current pane for Pi reload");
	const prompt = params.continuationPrompt?.trim() || "Pi runtime reload complete. Continue the work from before reload; first verify the reloaded tool/extension surface if that was the reason for reload.";
	const continuationPath = writeReloadContinuation(params.cwd, {
		createdAt: new Date().toISOString(),
		prompt,
	});
	const delayMs = params.delayMs ?? 12_000;
	const retryAttempts = params.retryAttempts ?? 3;
	const retryDelayMs = params.retryDelayMs ?? 10_000;
	const script = buildPiReloadScript({ cwd: params.cwd, pane, delayMs, retryAttempts, retryDelayMs });
	spawnDetached("bash", ["-lc", script], { cwd: params.cwd });
	return {
		text: `scheduled Pi /reload for ${pane}`,
		details: { pane, delayMs, retryAttempts, retryDelayMs, continuationPath, method: "tmux send-keys /reload Enter with warning retry" },
	};
}

export function buildPiReloadScript(params: { cwd: string; pane: string; delayMs: number; retryAttempts?: number; retryDelayMs?: number }): string {
	const delay = Math.max(0, params.delayMs) / 1000;
	const retryAttempts = Math.max(1, Math.floor(params.retryAttempts ?? 3));
	const retryDelay = Math.max(0, params.retryDelayMs ?? 10_000) / 1000;
	const warning = "Wait for the current response to finish before reloading.";
	const tmuxBase = ["tmux", ...tmuxPrefix()];
	const sendArgs = [...tmuxBase, "send-keys", "-t", params.pane, "/reload", "Enter"].map(shellQuote).join(" ");
	const captureArgs = [...tmuxBase, "capture-pane", "-t", params.pane, "-p", "-S", "-", "-J"].map(shellQuote).join(" ");
	return [
		`cd ${shellQuote(params.cwd)}`,
		`warning=${shellQuote(warning)}`,
		`count_warning() { ${captureArgs} 2>/dev/null | grep -F -c -- "$warning" || true; }`,
		`before=$(count_warning)`,
		`sleep ${delay}`,
		`attempt=1`,
		`while [ "$attempt" -le ${retryAttempts} ]; do`,
		`  ${sendArgs}`,
		`  sleep 2`,
		`  after=$(count_warning)`,
		`  if [ "$after" -le "$before" ]; then exit 0; fi`,
		`  before="$after"`,
		`  attempt=$((attempt + 1))`,
		`  if [ "$attempt" -le ${retryAttempts} ]; then sleep ${retryDelay}; fi`,
		`done`,
	].join("\n");
}

export async function schedulePiRespawn(params: {
	cwd: string;
	paneId?: string;
	command?: string;
	delayMs?: number;
	continuationPrompt?: string;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = params.paneId ?? (await currentPane(params.cwd))?.paneId;
	if (!pane) throw new Error("cannot resolve current pane for Pi respawn");
	if (params.command && params.continuationPrompt) throw new Error("continuationPrompt cannot be combined with a custom restart command");
	const details: Record<string, unknown> = { pane, delayMs: params.delayMs ?? 300, method: "tmux respawn-pane -k" };
	let command = params.command ?? "pi -c";
	if (params.continuationPrompt) {
		const handoffDir = join(params.cwd, ".dump", "pi-zmux", "respawn-handoffs");
		await mkdir(handoffDir, { recursive: true });
		const handoffPath = join(handoffDir, `${new Date().toISOString().replace(/[:.]/gu, "-")}.md`);
		await writeFile(handoffPath, `${params.continuationPrompt.trim()}\n`);
		const continuationPath = writeRespawnContinuation(params.cwd, {
			createdAt: new Date().toISOString(),
			prompt: params.continuationPrompt.trim(),
			handoffPath,
		});
		details.continuationHandoff = handoffPath;
		details.continuationPath = continuationPath;
	}
	const script = buildTmuxRespawnScript({
		cwd: params.cwd,
		pane,
		command,
		delayMs: params.delayMs ?? 300,
	});
	spawnDetached("bash", ["-lc", script], { cwd: params.cwd });
	details.command = command;
	return { text: `scheduled Pi pane respawn for ${pane} using ${command}`, details };
}

export function buildTmuxRespawnScript(params: { cwd: string; pane: string; command: string; delayMs: number }): string {
	const delay = Math.max(0, params.delayMs) / 1000;
	const tmuxArgs = ["tmux", ...tmuxPrefix(), "respawn-pane", "-k", "-t", params.pane, "-c", params.cwd, params.command];
	return [
		`cd ${shellQuote(params.cwd)}`,
		`sleep ${delay}`,
		tmuxArgs.map(shellQuote).join(" "),
	].join("; ");
}
