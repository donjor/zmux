package sync

import (
	"fmt"
	"testing"
)

// mockCmdRunner records and returns predetermined results.
type mockCmdRunner struct {
	output string
	err    error
}

func (m *mockCmdRunner) Run(name string, args ...string) (string, error) {
	return m.output, m.err
}

func TestNvimTarget_Pull(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		err     error
		want    string
		wantErr bool
	}{
		{
			name:   "tokyonight-storm",
			output: "tokyonight-storm",
			want:   "tokyonight",
		},
		{
			name:   "tokyonight",
			output: "tokyonight",
			want:   "tokyonight",
		},
		{
			name:   "catppuccin-macchiato",
			output: "catppuccin-macchiato",
			want:   "catppuccin-mocha",
		},
		{
			name:   "gruvbox-material",
			output: "gruvbox-material",
			want:   "gruvbox-dark",
		},
		{
			name:   "kanagawa-wave",
			output: "kanagawa-wave",
			want:   "kanagawa-dragon",
		},
		{
			name:   "rose-pine-moon",
			output: "rose-pine-moon",
			want:   "rose-pine",
		},
		{
			name:   "material-oceanic",
			output: "material-oceanic",
			want:   "material-darker",
		},
		{
			name:   "unknown scheme passes through",
			output: "my-custom-theme",
			want:   "my-custom-theme",
		},
		{
			name:   "dracula direct match",
			output: "dracula",
			want:   "dracula",
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
		{
			name:    "nvim not found",
			err:     fmt.Errorf("exec: nvim not found"),
			wantErr: true,
		},
		{
			name:    "whitespace only",
			output:  "   \n  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockCmdRunner{output: tt.output, err: tt.err}
			target := NewNvimTarget(runner)

			got, err := target.Pull()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Pull() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNvimTarget_Name(t *testing.T) {
	target := NewNvimTarget(nil)
	if name := target.Name(); name != "nvim" {
		t.Errorf("Name() = %q, want %q", name, "nvim")
	}
}

func TestNormalizeNvimTheme(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"tokyonight", "tokyonight"},
		{"tokyonight-night", "tokyonight"},
		{"tokyonight-storm", "tokyonight"},
		{"catppuccin-latte", "catppuccin-mocha"},
		{"catppuccin", "catppuccin-mocha"},
		{"gruvbox", "gruvbox-dark"},
		{"gruvbox-material", "gruvbox-dark"},
		{"kanagawa", "kanagawa-dragon"},
		{"kanagawa-dragon", "kanagawa-dragon"},
		{"rose-pine", "rose-pine"},
		{"rose-pine-moon", "rose-pine"},
		{"material-oceanic", "material-darker"},
		{"nord", "nord"},
		{"dracula", "dracula"},
		{"my-theme", "my-theme"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeNvimTheme(tt.input)
			if got != tt.want {
				t.Errorf("normalizeNvimTheme(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
