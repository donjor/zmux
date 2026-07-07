package scroll

import "testing"

func TestNeedsScrollbar(t *testing.T) {
	cases := []struct {
		name       string
		height     int
		totalLines int
		want       bool
	}{
		{"zero height", 0, 5, false},
		{"empty content", 10, 0, false},
		{"single line fits", 10, 1, false},
		{"exact fit", 10, 10, false},
		{"one over", 10, 11, true},
		{"much taller", 5, 100, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsScrollbar(tc.height, tc.totalLines); got != tc.want {
				t.Fatalf("needsScrollbar(%d, %d) = %v, want %v", tc.height, tc.totalLines, got, tc.want)
			}
		})
	}
}

func TestThumbGeometry(t *testing.T) {
	cases := []struct {
		name          string
		height        int
		totalLines    int
		scrollPercent float64
		wantStart     int
		wantEnd       int
	}{
		{"top clamp", 10, 100, 0, 0, 1},
		{"bottom clamp flush", 10, 100, 1, 9, 10},
		{"half-height thumb top", 10, 20, 0, 0, 5},
		{"half-height thumb bottom", 10, 20, 1, 5, 10},
		{"half-height thumb mid", 10, 20, 0.5, 3, 8},
		{"single-row viewport", 1, 5, 0, 0, 1},
		{"single-row viewport scrolled", 1, 5, 1, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := thumbGeometry(tc.height, tc.totalLines, tc.scrollPercent)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("thumbGeometry(%d, %d, %v) = (%d, %d), want (%d, %d)",
					tc.height, tc.totalLines, tc.scrollPercent, start, end, tc.wantStart, tc.wantEnd)
			}
			if end > tc.height {
				t.Fatalf("thumb end %d exceeds height %d", end, tc.height)
			}
		})
	}
}
