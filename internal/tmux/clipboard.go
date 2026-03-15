package tmux

import "os/exec"

// DetectClipboard returns the best available clipboard command.
// Checks in order: wl-copy (Wayland), xclip (X11), pbcopy (macOS).
// Returns an empty string if none is found.
func DetectClipboard() string {
	tools := []string{"wl-copy", "xclip", "pbcopy"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			return tool
		}
	}
	return ""
}

// ClipboardBinding returns a tmux copy-mode-vi binding line for the y key
// that pipes the selection to the given clipboard tool.
func ClipboardBinding(tool string) string {
	switch tool {
	case "wl-copy":
		return `bind -T copy-mode-vi y send -X copy-pipe-and-cancel "wl-copy"`
	case "xclip":
		return `bind -T copy-mode-vi y send -X copy-pipe-and-cancel "xclip -selection clipboard"`
	case "pbcopy":
		return `bind -T copy-mode-vi y send -X copy-pipe-and-cancel "pbcopy"`
	default:
		// Fallback: just copy to tmux buffer
		return `bind -T copy-mode-vi y send -X copy-selection-and-cancel`
	}
}
