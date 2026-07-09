import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { registerLiteDispatcher } from "./dispatcher.js";

export default function piZmuxLite(pi: ExtensionAPI): void {
  registerLiteDispatcher(pi);
}
