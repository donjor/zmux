export const ZMUX_OPERATIONS = [
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
	"peer_handoff",
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

export type ZmuxOperation = (typeof ZMUX_OPERATIONS)[number];

const operationSet = new Set<string>(ZMUX_OPERATIONS);

export function isZmuxOperation(value: string): value is ZmuxOperation {
	return operationSet.has(value);
}
