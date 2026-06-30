import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import {
	closePane,
	focusPane,
	listPanes,
	openPane,
	resizePane,
	sendPaneKeys,
	typePaneText,
} from "../zmux.js";
import { content, paneDirection, resolveCwd } from "./shared.js";

export function registerPaneTools(pi: ExtensionAPI): void {
	pi.registerTool({
		name: "zmux_pane_send_keys",
		label: "zmux pane send keys",
		description: "Send raw keys to a specific tmux pane id/title/index. Use sparingly for sidecar panes; prefer tab-level zmux_send_keys when a logical tab name exists.",
		promptSnippet: "Send raw keys to a specific pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			keys: Type.Array(Type.String(), { description: "Raw keys to send, e.g. C-c, Enter, Escape, l" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sendPaneKeys(params.pane, params.keys, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_type",
		label: "zmux pane type",
		description: "Type text plus Enter into a specific tmux pane id/title/index. Use sparingly for sidecar panes; prefer tab-level zmux_type when a logical tab name exists.",
		promptSnippet: "Type text into a specific pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			text: Type.String({ description: "Text to type and submit" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typePaneText(params.pane, params.text, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_list",
		label: "zmux pane list",
		description: "List panes in the current window or session. Prefer this over `zmux pane list` in bash.",
		promptSnippet: "List panes in zmux",
		parameters: Type.Object({
			session: Type.Optional(Type.String({ description: "Optional session target; lists joined/session panes when supported" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const panes = await listPanes(ctx.cwd, params.session);
			return content(panes, { panes, session: params.session });
		},
	});

	pi.registerTool({
		name: "zmux_pane_open",
		label: "zmux pane open",
		description: "Open a named sidecar pane using `zmux pane open`. Use for visible sidecars or terminal UI helpers instead of raw tmux split-window.",
		promptSnippet: "Open a named zmux sidecar pane",
		parameters: Type.Object({
			name: Type.String({ description: "Pane title/name" }),
			command: Type.String({ description: "Command to run in the pane" }),
			direction: Type.Optional(Type.String({ description: "Split direction: right, left, down, or up; default right" })),
			size: Type.Optional(Type.String({ description: "Pane size, e.g. 35% or 80 cells" })),
			target: Type.Optional(Type.String({ description: "Target pane/window; defaults to current" })),
			labelTab: Type.Optional(Type.Boolean({ description: "Preserve current tab name as a zmux label before opening pane" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const cwd = resolveCwd(ctx.cwd, params.cwd);
			const result = await openPane({
				name: params.name,
				command: params.command,
				cwd,
				direction: paneDirection(params.direction),
				size: params.size,
				target: params.target,
				labelTab: params.labelTab,
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_focus",
		label: "zmux pane focus",
		description: "Focus a zmux pane by id/title/index. Prefer this over `zmux pane focus` in bash.",
		promptSnippet: "Focus a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to focus" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await focusPane(params.pane, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_close",
		label: "zmux pane close",
		description: "Close a zmux pane by id/title/index. Use for intentional pane cleanup instead of shelling out to `zmux pane close` in bash.",
		promptSnippet: "Close a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to close" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await closePane(params.pane, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_resize",
		label: "zmux pane resize",
		description: "Resize a zmux pane by id/title/index. Use for intentional pane layout control instead of raw tmux resize-pane.",
		promptSnippet: "Resize a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to resize" }),
			size: Type.String({ description: "New pane size, e.g. 40% or 80" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await resizePane(params.pane, resolveCwd(ctx.cwd, params.cwd), params.size);
			return content(result.text, result.details);
		},
	});
}
