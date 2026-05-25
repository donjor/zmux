package tmux

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

func TestGenerateConfContainsGeneral(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`set -g default-terminal "tmux-256color"`,
		`set -g mouse on`,
		`set -g history-limit 50000`,
		`set -g escape-time 10`,
		`set -g base-index 1`,
		`setw -g pane-base-index 1`,
		`set -g renumber-windows on`,
		`setw -g automatic-rename on`,
		`setw -g automatic-rename-format "#{?pane_in_mode,[tmux],#{pane_current_command}}"`,
		`setw -g remain-on-exit failed`,
		`setw -g remain-on-exit-format "Pane stopped unexpectedly#{?#{!=:#{pane_dead_status},}, (status #{pane_dead_status}),}#{?#{!=:#{pane_dead_signal},}, (signal #{pane_dead_signal}),} — press prefix+x to close the tab"`,
		`set -g set-clipboard on`,
		`set -g status-position top`,
		`set -g set-titles on`,
		`set -g set-titles-string "zmux:v1;tty=#{client_tty};sid=#{session_id};wid=#{window_id};pane=#{pane_id} #{session_name}:#{window_index}:#{window_name}"`,
		`set -g focus-events on`,
		`set -g extended-keys on`,
		`set -g terminal-features[90] "xterm-256color:RGB:extkeys"`,
		`set -g terminal-features[91] "xterm-ghostty:RGB:extkeys"`,
		`set -g terminal-features[92] "tmux-256color:RGB:extkeys"`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing general setting: %q", want)
		}
	}
}

func TestGenerateConfContainsPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "unbind C-b") {
		t.Error("conf missing 'unbind C-b'")
	}
	if !strings.Contains(conf, "set -g prefix C-Space") {
		t.Error("conf missing prefix set")
	}
	if !strings.Contains(conf, "bind C-Space send-prefix") {
		t.Error("conf missing prefix bind")
	}
}

func TestGenerateConfCustomPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Prefix = "C-a"
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "set -g prefix C-a") {
		t.Error("conf should use custom prefix C-a")
	}
	if !strings.Contains(conf, "bind C-a send-prefix") {
		t.Error("conf should bind custom prefix C-a")
	}
}

func TestGenerateConfContainsVimCopyMode(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		"setw -g mode-keys vi",
		"bind v copy-mode",
		"bind R respawn-pane -k",
		"bind -T copy-mode-vi v send -X begin-selection",
		"bind -T copy-mode-vi Escape send -X cancel",
		"bind -T copy-mode-vi C-v send -X rectangle-toggle",
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing vim copy mode setting: %q", want)
		}
	}
}

func TestGenerateConfContainsAltTabSwitching(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for i := 1; i <= 5; i++ {
		want := "bind -n M-" + string(rune('0'+i)) + " select-window -t " + string(rune('0'+i))
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing alt tab binding: %q", want)
		}
	}
}

func TestGenerateConfContainsWindowBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`bind c new-window -c "#{pane_current_path}"`,
		"bind n next-window",
		`bind . command-prompt -p "label tab (blank clears):"`,
		`set-option -w -t #{window_id} @zmux_label`,
		`bind x confirm-before`,
		`#{?@zmux_label,#{?#{==:#{@zmux_label},#W},#W,#{@zmux_label} [#W]},#W#{?@zmux_duplicate_name,[#{b:pane_current_path}],}}`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing window binding: %q", want)
		}
	}
}

func TestGenerateConfContainsNoPrefixPaneFocusBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		"bind -n M-S-Left select-pane -L",
		"bind -n M-S-Right select-pane -R",
		"bind -n M-S-Up select-pane -U",
		"bind -n M-S-Down select-pane -D",
	}
	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing pane focus binding: %q", want)
		}
	}
}

func TestGenerateConfContainsSessionBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`bind , command-prompt -p "rename session:"`,
		"bind w display-popup",
		"bind [ run-shell",
		"bind ] run-shell",
		"bind -n M-S-1 run-shell",
		`bind x confirm-before`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing session binding: %q", want)
		}
	}
}

func TestGenerateConfContainsPopupBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind Space display-popup") {
		t.Error("conf missing dashboard popup binding (prefix+Space)")
	}
}

func TestGenerateConfContainsHelpPopup(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, `bind ? display-popup`) {
		t.Error("conf missing help popup binding (prefix+?)")
	}
	if !strings.Contains(conf, "zmux help") {
		t.Error("conf missing zmux help in help popup")
	}
}

func TestGenerateConfContainsBootstrap(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, `run-shell "/usr/local/bin/zmux apply"`) {
		t.Error("conf missing run-shell bootstrap")
	}
}

func TestGenerateConfContainsClipboard(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	// Should contain some clipboard binding (the exact one depends on host)
	if !strings.Contains(conf, "copy-mode-vi y") {
		t.Error("conf missing clipboard y binding")
	}
}

func TestGenerateConfContainsPasteBuffer(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind P paste-buffer") {
		t.Error("conf missing paste-buffer binding (prefix+P)")
	}
}

func TestGenerateConfContainsPaletteBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind p display-popup") {
		t.Error("conf missing command palette popup binding (prefix+p)")
	}
	if !strings.Contains(conf, "--palette") {
		t.Error("conf missing --palette flag in palette popup binding")
	}
}

func TestGenerateConfContainsScratchShell(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind ! display-popup") {
		t.Error("conf missing scratch shell popup binding (prefix+!)")
	}
	if !strings.Contains(conf, `-d "#{pane_current_path}"`) {
		t.Error("conf missing pane cwd start dir for scratch shell")
	}
	if !strings.Contains(conf, `"$SHELL"`) {
		t.Error("conf missing $SHELL invocation in scratch shell popup")
	}
}

func TestGenerateConfContainsReload(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind r run-shell") {
		t.Error("conf missing reload binding")
	}
}

func TestGenerateConfContainsDuplicateNameRefreshHooks(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for _, want := range []string{
		`set-hook -gu window-unlinked[0]`,
		`set-hook -gu window-linked[1]`,
		`set-hook -gu window-unlinked[1]`,
		`set-hook -gu window-renamed[1]`,
		`set-hook -g window-linked[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
		`set-hook -g window-unlinked[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
		`set-hook -g window-renamed[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing duplicate-name hook: %q", want)
		}
	}
}

func testPalette() theme.Palette {
	c := func(r, g, b uint8) theme.Color { return theme.Color{R: r, G: g, B: b} }
	return theme.Palette{
		BG:        c(10, 14, 20),
		FG:        c(184, 191, 198),
		Surface:   c(0, 0, 0),
		Error:     c(255, 51, 51),
		Success:   c(127, 204, 60),
		Accent:    c(255, 180, 84),
		Info:      c(54, 163, 217),
		Special:   c(209, 97, 203),
		Meta:      c(149, 230, 203),
		Muted:     c(200, 200, 200),
		Dim:       c(100, 100, 100),
		Highlight: c(255, 204, 102),
		BGDim:     c(25, 29, 35),
		BGPrefix:  c(35, 39, 45),
	}
}
