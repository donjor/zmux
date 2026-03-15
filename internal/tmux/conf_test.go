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
		`set -g default-terminal "xterm-256color"`,
		`set -g mouse on`,
		`set -g history-limit 50000`,
		`set -g escape-time 10`,
		`set -g base-index 1`,
		`setw -g pane-base-index 1`,
		`set -g renumber-windows on`,
		`set -g set-clipboard on`,
		`set -g status-position top`,
		`set -g focus-events on`,
		`set -g extended-keys on`,
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
		`bind . command-prompt -p "rename tab:"`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing window binding: %q", want)
		}
	}
}

func TestGenerateConfContainsSessionBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`bind , command-prompt -p "rename session:"`,
		"bind s choose-tree -s",
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

	if !strings.Contains(conf, "bind d display-popup") {
		t.Error("conf missing popup binding (prefix+d)")
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

	if !strings.Contains(conf, "bind p paste-buffer") {
		t.Error("conf missing paste-buffer binding")
	}
}

func TestGenerateConfContainsReload(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind r source-file") {
		t.Error("conf missing reload binding")
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
