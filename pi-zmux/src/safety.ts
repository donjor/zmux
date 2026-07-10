import { HEADLESS_AGENT_PRINT_PATTERN, HEADLESS_AGENT_SUGGESTION, stripQuotedSegments } from "./classify.js";

export function shouldWaitForExit(command: string): boolean {
	const trimmed = command.trim();
	if (/^sudo\s+(-i|-s|su\b)/u.test(trimmed)) return false;
	if (/^(ssh|psql|mysql|sqlite3|redis-cli|python|node|irb|pry|bash|zsh|fish)(\s+.*)?$/u.test(trimmed) && !/^sudo\b/u.test(trimmed)) {
		return false;
	}
	return /(^|[;&|]\s*)(sudo|su)\b/u.test(trimmed);
}

export function hasHeadlessAgentPrintMode(command: string): boolean {
	return HEADLESS_AGENT_PRINT_PATTERN.test(stripQuotedSegments(command.trim()));
}

export function rejectHeadlessAgentPrintMode(command: string): string | undefined {
	if (!hasHeadlessAgentPrintMode(command)) return undefined;
	return HEADLESS_AGENT_SUGGESTION;
}
