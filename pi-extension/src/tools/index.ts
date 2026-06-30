import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { registerCoreTools } from "./core.js";
import { registerPaneTools } from "./panes.js";
import { registerRuntimeTools } from "./runtimes.js";
import { registerTabTools } from "./tabs.js";

export function registerZmuxTools(pi: ExtensionAPI): void {
	registerCoreTools(pi);
	registerTabTools(pi);
	registerPaneTools(pi);
	registerRuntimeTools(pi);
}
