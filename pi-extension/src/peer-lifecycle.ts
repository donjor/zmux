import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { registerPeerLifecycle } from "./lifecycle.js";

export default function (pi: ExtensionAPI): void {
	registerPeerLifecycle(pi);
}
