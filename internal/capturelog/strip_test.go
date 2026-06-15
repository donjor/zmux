package capturelog

import "testing"

func strip(s string) string {
	return string((&ansiStripper{}).feed([]byte(s)))
}

func TestAnsiStripper(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain unchanged", "hello world\n", "hello world\n"},
		{"sgr colour", "\x1b[31mred\x1b[0m", "red"},
		{"cursor + clear", "\x1b[2J\x1b[Hhome", "home"},
		{"multi-param csi", "\x1b[1;32;40mx", "x"},
		{"osc title bel", "\x1b]0;my title\x07text", "text"},
		{"osc title st", "\x1b]0;my title\x1b\\text", "text"},
		{"charset designator", "\x1b(Bplain", "plain"},
		{"simple two-byte esc", "\x1bcreset", "reset"},
		{"keep tab and cr", "a\tb\r\n", "a\tb\r\n"},
		{"drop bell and backspace", "a\x07b\x08c", "abc"},
		{"drop del", "a\x7fb", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strip(tt.in); got != tt.want {
				t.Errorf("strip(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The stripper must carry state across feed() calls, since pipe-pane can split
// an escape sequence across chunk boundaries.
func TestAnsiStripperSplitAcrossChunks(t *testing.T) {
	a := &ansiStripper{}
	var got []byte
	for _, chunk := range []string{"\x1b[", "31", "mHI", "\x1b]0;t", "itle\x07!"} {
		got = append(got, a.feed([]byte(chunk))...)
	}
	if string(got) != "HI!" {
		t.Fatalf("split-chunk strip = %q, want %q", got, "HI!")
	}
}
